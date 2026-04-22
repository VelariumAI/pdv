package download

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"sync"
	"time"

	"github.com/velariumai/pdv/internal/config"
	"github.com/velariumai/pdv/internal/database"
	"github.com/velariumai/pdv/internal/events"
	"github.com/velariumai/pdv/pkg/output"
)

var runDownload = DownloadWithProgress

// Engine orchestrates download workers, queue persistence, and event publishing.
type Engine struct {
	cfg *config.Config
	db  *database.DB

	workerCount int
	jobChan     chan *output.QueueEntry
	queue       *Queue

	eventsMu sync.RWMutex
	events   map[string][]chan interface{}

	stateMu      sync.RWMutex
	workerStates map[int]*workerState

	cancelFuncsMu sync.RWMutex
	cancelFuncs   map[int64]context.CancelFunc

	lifecycleMu sync.Mutex
	started     bool
	stopped     bool

	ctx    context.Context
	cancel context.CancelFunc

	stopOnce         sync.Once
	dbWriterStop     chan struct{}
	dbWriterStopOnce sync.Once
	wg               sync.WaitGroup

	retryBackoffFn func(int) time.Duration
	log            *slog.Logger
}

type workerState struct {
	id        int
	state     string
	currentID int64
	startedAt time.Time
}

// NewEngine creates a new Engine with the provided config and database.
func NewEngine(cfg *config.Config, db *database.DB) *Engine {
	if cfg == nil {
		cfg = config.New()
	}
	workerCount := cfg.MaxConcurrentQueue
	if workerCount <= 0 {
		workerCount = 2
	}

	e := &Engine{
		cfg:            cfg,
		db:             db,
		workerCount:    workerCount,
		jobChan:        make(chan *output.QueueEntry, workerCount*2),
		queue:          NewQueue(db),
		events:         make(map[string][]chan interface{}),
		workerStates:   make(map[int]*workerState, workerCount),
		cancelFuncs:    make(map[int64]context.CancelFunc),
		dbWriterStop:   make(chan struct{}),
		retryBackoffFn: retryBackoff,
		log:            slog.With("module", "download", "engine", "worker-pool"),
	}
	return e
}

// Add inserts a queue entry in persistent storage and schedules it when the engine is running.
func (e *Engine) Add(ctx context.Context, url string, opts *output.AddOpts) (*output.QueueEntry, error) {
	entry, err := e.queue.Add(ctx, url, opts)
	if err != nil {
		return nil, err
	}
	if e.canSubmit() {
		if err := e.submitEntry(entry); err != nil {
			return nil, fmt.Errorf("download: add: submit %d: %w", entry.ID, err)
		}
	}
	return entry, nil
}

// Pause sets a queue entry to paused and cancels active work.
func (e *Engine) Pause(ctx context.Context, id int64) error {
	cancelFn := e.lookupCancel(id)
	if err := e.queue.Pause(ctx, id, cancelFn); err != nil {
		return err
	}
	e.publish("DownloadPaused", &events.DownloadPaused{
		ID:        id,
		Timestamp: time.Now().UTC(),
	})
	return nil
}

// Resume sets a paused/failed entry back to pending and re-enqueues when running.
func (e *Engine) Resume(ctx context.Context, id int64) error {
	submitFn := e.optionalSubmitFn()
	return e.queue.Resume(ctx, id, submitFn)
}

// Cancel marks an entry cancelled, cancels active work, and removes it from queue storage.
func (e *Engine) Cancel(ctx context.Context, id int64) error {
	cancelFn := e.lookupCancel(id)
	return e.queue.Cancel(ctx, id, cancelFn)
}

// Retry increments retry_count, sets the entry to pending, and re-enqueues when running.
func (e *Engine) Retry(ctx context.Context, id int64) error {
	cancelFn := e.lookupCancel(id)
	submitFn := e.optionalSubmitFn()
	return e.queue.Retry(ctx, id, submitFn, cancelFn)
}

// Start launches workers, DB writer, and startup queue rescan.
func (e *Engine) Start(ctx context.Context) error {
	if e.db == nil {
		return fmt.Errorf("download: engine start: database is nil")
	}

	e.lifecycleMu.Lock()
	if e.started && !e.stopped {
		e.lifecycleMu.Unlock()
		return fmt.Errorf("download: engine already started")
	}
	if e.started && e.stopped {
		e.lifecycleMu.Unlock()
		return fmt.Errorf("download: engine cannot be restarted")
	}

	e.ctx, e.cancel = context.WithCancel(ctx)
	e.started = true
	e.stopped = false
	e.lifecycleMu.Unlock()

	e.wg.Add(1)
	go e.dbWriter()

	for i := 0; i < e.workerCount; i++ {
		e.workerStates[i] = &workerState{id: i, state: "idle"}
		e.wg.Add(1)
		go e.workerLoop(i)
		e.publish("WorkerStarted", &events.WorkerStarted{ID: i, Timestamp: time.Now().UTC()})
	}

	if err := e.rescanAndRequeue(e.ctx); err != nil {
		e.log.Warn("startup rescan failed", "error", err)
	}
	e.log.Info("engine started", "workers", e.workerCount)
	return nil
}

// Stop gracefully drains workers and the DB writer.
func (e *Engine) Stop(ctx context.Context) error {
	e.lifecycleMu.Lock()
	if !e.started {
		e.lifecycleMu.Unlock()
		return fmt.Errorf("download: engine not started")
	}
	if e.stopped {
		e.lifecycleMu.Unlock()
		return nil
	}
	e.stopped = true
	e.lifecycleMu.Unlock()

	e.stopOnce.Do(func() {
		close(e.jobChan)
		if e.cancel != nil {
			e.cancel()
		}
	})
	e.dbWriterStopOnce.Do(func() { close(e.dbWriterStop) })

	done := make(chan struct{})
	go func() {
		e.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		e.log.Info("engine stopped gracefully")
		return nil
	case <-ctx.Done():
		return fmt.Errorf("download: engine shutdown timeout: %w", ctx.Err())
	}
}

// Submit enqueues a job for workers.
func (e *Engine) Submit(job *output.QueueEntry) error {
	if job == nil {
		return fmt.Errorf("download: job is nil")
	}

	e.lifecycleMu.Lock()
	started := e.started
	stopped := e.stopped
	ctx := e.ctx
	e.lifecycleMu.Unlock()
	if !started {
		return fmt.Errorf("download: engine not started")
	}
	if stopped {
		return fmt.Errorf("download: engine stopped")
	}
	if ctx != nil && ctx.Err() != nil {
		return fmt.Errorf("download: engine stopped")
	}

	select {
	case e.jobChan <- job:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("download: engine stopped")
	default:
		return fmt.Errorf("download: job queue full")
	}
}

// SubmitBatch enqueues many jobs.
func (e *Engine) SubmitBatch(jobs []*output.QueueEntry) error {
	for _, job := range jobs {
		if err := e.Submit(job); err != nil {
			return err
		}
	}
	return nil
}

// Workers returns a deterministic snapshot of worker status.
func (e *Engine) Workers() []output.WorkerStatus {
	e.stateMu.RLock()
	defer e.stateMu.RUnlock()

	ids := make([]int, 0, len(e.workerStates))
	for id := range e.workerStates {
		ids = append(ids, id)
	}
	sort.Ints(ids)

	result := make([]output.WorkerStatus, 0, len(ids))
	for _, id := range ids {
		ws := e.workerStates[id]
		status := output.WorkerStatus{
			ID:    ws.id,
			State: ws.state,
		}
		if ws.currentID != 0 {
			status.CurrentID = ws.currentID
			if !ws.startedAt.IsZero() {
				started := ws.startedAt
				status.StartedAt = &started
			}
		}
		result = append(result, status)
	}
	return result
}

func (e *Engine) workerLoop(workerID int) {
	defer e.wg.Done()
	defer e.publish("WorkerStopped", &events.WorkerStopped{ID: workerID, Timestamp: time.Now().UTC()})

	for {
		select {
		case <-e.ctx.Done():
			return
		case job, ok := <-e.jobChan:
			if !ok {
				return
			}
			e.handleJob(workerID, job)
		}
	}
}

func (e *Engine) handleJob(workerID int, job *output.QueueEntry) {
	entry, err := e.db.GetQueueEntry(e.ctx, job.ID)
	if err != nil || entry == nil {
		e.log.Warn("job skipped: queue entry missing", "id", job.ID, "error", err)
		return
	}
	if entry.Status == output.StatusPaused || entry.Status == output.StatusCancelled {
		return
	}
	if err := e.queue.TransitionToActive(e.ctx, job.ID, workerID); err != nil {
		e.log.Warn("job transition failed", "id", job.ID, "error", err)
		return
	}

	startedAt := time.Now().UTC()
	e.setWorkerState(workerID, "downloading", job.ID, startedAt)
	e.publish("DownloadStarted", &events.DownloadStarted{
		ID:        job.ID,
		Timestamp: startedAt,
		URL:       entry.URL,
		Title:     entry.Title,
	})

	jobCtx, jobCancel := context.WithCancel(e.ctx)
	e.cancelFuncsMu.Lock()
	e.cancelFuncs[job.ID] = jobCancel
	e.cancelFuncsMu.Unlock()

	opts := e.buildDownloadOpts(entry)
	downloadErr := runDownload(jobCtx, entry, opts, func(ev output.ProgressEvent) {
		e.publish("DownloadProgress", &events.DownloadProgress{
			ID:         ev.ID,
			Timestamp:  time.Now().UTC(),
			Percentage: ev.Percentage,
			Speed:      ev.Speed,
			ETA:        ev.ETA,
		})
		_ = e.queue.UpdateProgress(e.ctx, ev.ID, workerID, ev.Percentage)
	})

	e.cancelFuncsMu.Lock()
	delete(e.cancelFuncs, job.ID)
	e.cancelFuncsMu.Unlock()
	e.setWorkerState(workerID, "idle", 0, time.Time{})

	now := time.Now().UTC()
	if downloadErr != nil {
		e.handleDownloadFailure(entry.ID, downloadErr, now)
		return
	}

	filePath := BuildOutputPath(opts.Template, entry.Title, "mp4", "")
	e.publish("DownloadCompleted", &events.DownloadCompleted{
		ID:        entry.ID,
		Timestamp: now,
		URL:       entry.URL,
		Title:     entry.Title,
		FilePath:  filePath,
	})
}

func (e *Engine) handleDownloadFailure(id int64, downloadErr error, failedAt time.Time) {
	opCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	entry, err := e.db.GetQueueEntry(opCtx, id)
	if err != nil || entry == nil {
		return
	}

	if entry.Status == output.StatusPaused {
		e.publish("DownloadPaused", &events.DownloadPaused{ID: id, Timestamp: failedAt})
		return
	}
	if entry.Status == output.StatusCancelled {
		return
	}
	if errors.Is(downloadErr, context.Canceled) && e.ctx.Err() != nil {
		entry.Status = output.StatusPending
		entry.WorkerID = 0
		entry.StartedAt = nil
		if err := e.db.UpdateQueueEntry(opCtx, entry); err != nil {
			e.log.Warn("shutdown cancel update failed", "id", id, "error", err)
		}
		return
	}

	if isPermanentDownloadError(downloadErr) {
		entry.RetryCount++
		entry.ErrorMsg = downloadErr.Error()
		entry.Status = output.StatusFailed
		if err := e.db.UpdateQueueEntry(opCtx, entry); err != nil {
			e.log.Warn("mark permanent failure update failed", "id", id, "error", err)
		}
		e.publish("DownloadFailed", &events.DownloadFailed{
			ID:        id,
			Timestamp: failedAt,
			URL:       entry.URL,
			Error:     downloadErr.Error(),
		})
		return
	}

	maxRetries := e.cfg.Retries
	if maxRetries < 0 {
		maxRetries = 0
	}
	entry.RetryCount++
	entry.ErrorMsg = downloadErr.Error()
	if entry.RetryCount > maxRetries {
		entry.Status = output.StatusFailed
		if err := e.db.UpdateQueueEntry(opCtx, entry); err != nil {
			e.log.Warn("mark failed update failed", "id", id, "error", err)
		}
		e.publish("DownloadFailed", &events.DownloadFailed{
			ID:        id,
			Timestamp: failedAt,
			URL:       entry.URL,
			Error:     downloadErr.Error(),
		})
		return
	}

	entry.Status = output.StatusPending
	entry.WorkerID = 0
	entry.StartedAt = nil
	if err := e.db.UpdateQueueEntry(opCtx, entry); err != nil {
		e.log.Warn("retry update failed", "id", id, "error", err)
		return
	}

	backoff := e.retryBackoffFn(entry.RetryCount)
	e.log.Info("retry scheduled", "id", id, "retry_count", entry.RetryCount, "backoff", backoff.Round(time.Second))
	go e.requeueAfterBackoff(id, backoff)
}

func isPermanentDownloadError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	return isPermanentYTDLPFailure(err.Error())
}

func (e *Engine) requeueAfterBackoff(id int64, backoff time.Duration) {
	timer := time.NewTimer(backoff)
	defer timer.Stop()
	select {
	case <-timer.C:
	case <-e.ctx.Done():
		return
	}
	entry, err := e.db.GetQueueEntry(e.ctx, id)
	if err != nil || entry == nil {
		return
	}
	if entry.Status != output.StatusPending {
		return
	}
	if err := e.submitEntry(entry); err != nil {
		e.log.Warn("retry submit failed", "id", id, "error", err)
	}
}

func retryBackoff(retryCount int) time.Duration {
	if retryCount < 1 {
		retryCount = 1
	}
	backoff := 30 * time.Second << (retryCount - 1)
	if backoff > 5*time.Minute {
		return 5 * time.Minute
	}
	return backoff
}

func (e *Engine) buildDownloadOpts(job *output.QueueEntry) *DownloadOpts {
	opts := &DownloadOpts{
		Quality:   e.cfg.DefaultQuality,
		Template:  e.cfg.OutputTemplate,
		Cookies:   e.cfg.CookieFile,
		Proxy:     e.cfg.Proxy,
		UserAgent: e.cfg.UserAgent,
	}
	addOpts := decodeAddOpts(job.Options)
	if addOpts.Quality != "" {
		opts.Quality = addOpts.Quality
	}
	if addOpts.Format != "" {
		opts.Format = addOpts.Format
	}
	if addOpts.Template != "" {
		opts.Template = addOpts.Template
	}
	opts.IsPlaylist = addOpts.IsPlaylist
	if opts.IsPlaylist && e.cfg.OutputTemplatePlaylist != "" {
		opts.Template = e.cfg.OutputTemplatePlaylist
	}
	return opts
}

func decodeAddOpts(raw string) output.AddOpts {
	if raw == "" {
		return output.AddOpts{}
	}
	var opts output.AddOpts
	if err := json.Unmarshal([]byte(raw), &opts); err != nil {
		return output.AddOpts{}
	}
	return opts
}

func (e *Engine) dbWriter() {
	defer e.wg.Done()

	dbEvents := make(chan interface{}, 64)
	offCompleted := e.Subscribe("DownloadCompleted", dbEvents)
	offFailed := e.Subscribe("DownloadFailed", dbEvents)
	defer func() {
		offCompleted()
		offFailed()
	}()

	for {
		select {
		case <-e.dbWriterStop:
			e.drainDBWriter(dbEvents)
			return
		case ev := <-dbEvents:
			e.handleDBEvent(ev)
		}
	}
}

func (e *Engine) drainDBWriter(dbEvents chan interface{}) {
	for {
		select {
		case ev := <-dbEvents:
			e.handleDBEvent(ev)
		default:
			return
		}
	}
}

func (e *Engine) handleDBEvent(ev interface{}) {
	switch v := ev.(type) {
	case *events.DownloadCompleted:
		e.writeHistoryEntry(v, output.StatusCompleted)
		if err := e.queue.MarkCompleted(context.Background(), v.ID, output.StatusCompleted, ""); err != nil {
			e.log.Warn("mark completed failed", "id", v.ID, "error", err)
		}
	case *events.DownloadFailed:
		e.writeHistoryEntryFromFailed(v)
		if err := e.queue.MarkCompleted(context.Background(), v.ID, output.StatusFailed, v.Error); err != nil {
			e.log.Warn("mark failed failed", "id", v.ID, "error", err)
		}
	}
}

func (e *Engine) writeHistoryEntry(ev *events.DownloadCompleted, finalStatus output.DownloadStatus) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	category := categorize(ev.FilePath)
	entry, err := e.db.GetQueueEntry(ctx, ev.ID)
	if err == nil && entry != nil {
		addOpts := decodeAddOpts(entry.Options)
		if addOpts.Category != "" {
			category = addOpts.Category
		}
	}

	filePath := ev.FilePath
	if e.cfg.AutoCategorize && filePath != "" {
		destDir := categoryDir(category, e.cfg.DownloadDir)
		moved, moveErr := moveFile(filePath, destDir)
		if moveErr == nil && moved != "" {
			filePath = moved
		}
	}

	h := &output.HistoryEntry{
		URL:          ev.URL,
		Title:        ev.Title,
		FinalStatus:  string(finalStatus),
		FilePath:     filePath,
		FileSize:     ev.FileSize,
		Category:     category,
		DownloadedAt: ev.Timestamp,
	}
	if _, err := e.db.CreateHistoryEntry(ctx, h); err != nil {
		e.log.Error("write history completed failed", "id", ev.ID, "url", ev.URL, "error", err)
	}
}

func (e *Engine) writeHistoryEntryFromFailed(ev *events.DownloadFailed) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	h := &output.HistoryEntry{
		URL:          ev.URL,
		FinalStatus:  string(output.StatusFailed),
		ErrorMsg:     ev.Error,
		DownloadedAt: ev.Timestamp,
	}
	if _, err := e.db.CreateHistoryEntry(ctx, h); err != nil {
		e.log.Error("write history failed failed", "id", ev.ID, "url", ev.URL, "error", err)
	}
}

func (e *Engine) rescanAndRequeue(ctx context.Context) error {
	entries, err := e.queue.Rescan(ctx)
	if err != nil {
		return err
	}

	var count int
	for _, entry := range entries {
		if entry.Status == output.StatusActive {
			entry.Status = output.StatusPending
			entry.WorkerID = 0
			entry.StartedAt = nil
			if err := e.db.UpdateQueueEntry(ctx, entry); err != nil {
				e.log.Warn("rescan update active->pending failed", "id", entry.ID, "error", err)
				continue
			}
		}
		if entry.Status == output.StatusPending {
			if err := e.submitEntry(entry); err != nil {
				e.log.Warn("rescan submit failed", "id", entry.ID, "error", err)
				continue
			}
			count++
		}
	}
	e.log.Info("rescan complete", "reenqueued", count)
	return nil
}

func (e *Engine) submitEntry(entry *output.QueueEntry) error {
	return e.Submit(entry)
}

func (e *Engine) setWorkerState(id int, state string, currentID int64, startedAt time.Time) {
	e.stateMu.Lock()
	defer e.stateMu.Unlock()

	ws := e.workerStates[id]
	if ws == nil {
		ws = &workerState{id: id}
		e.workerStates[id] = ws
	}
	ws.state = state
	ws.currentID = currentID
	ws.startedAt = startedAt
}

func (e *Engine) publish(eventType string, event interface{}) {
	e.eventsMu.RLock()
	subscribers := e.events[eventType]
	e.eventsMu.RUnlock()

	for _, ch := range subscribers {
		select {
		case ch <- event:
		default:
		}
	}
}

// Subscribe registers an event subscriber and returns an unsubscribe callback.
func (e *Engine) Subscribe(eventType string, ch chan interface{}) func() {
	e.eventsMu.Lock()
	e.events[eventType] = append(e.events[eventType], ch)
	e.eventsMu.Unlock()

	return func() {
		e.eventsMu.Lock()
		defer e.eventsMu.Unlock()
		subs := e.events[eventType]
		for i, sub := range subs {
			if sub == ch {
				e.events[eventType] = append(subs[:i], subs[i+1:]...)
				return
			}
		}
	}
}

func (e *Engine) canSubmit() bool {
	e.lifecycleMu.Lock()
	defer e.lifecycleMu.Unlock()
	return e.started && !e.stopped
}

func (e *Engine) optionalSubmitFn() func(*output.QueueEntry) error {
	if e.canSubmit() {
		return e.submitEntry
	}
	return nil
}

func (e *Engine) lookupCancel(id int64) func() {
	e.cancelFuncsMu.RLock()
	defer e.cancelFuncsMu.RUnlock()
	return e.cancelFuncs[id]
}

// ListQueue returns queue entries optionally filtered by status.
func (e *Engine) ListQueue(ctx context.Context, status string) ([]output.QueueEntry, error) {
	if e.db == nil {
		return nil, fmt.Errorf("download: list queue: database is nil")
	}
	return e.db.ListQueueEntries(ctx, status)
}

// GetQueue returns one queue entry by id.
func (e *Engine) GetQueue(ctx context.Context, id int64) (*output.QueueEntry, error) {
	if e.db == nil {
		return nil, fmt.Errorf("download: get queue: database is nil")
	}
	return e.db.GetQueueEntry(ctx, id)
}

// PauseAll pauses every pending/active queue entry.
func (e *Engine) PauseAll(ctx context.Context) error {
	entries, err := e.db.ListQueueEntries(ctx, "")
	if err != nil {
		return fmt.Errorf("download: pause all: list queue: %w", err)
	}
	for i := range entries {
		if entries[i].Status == output.StatusPending || entries[i].Status == output.StatusActive {
			if err := e.Pause(ctx, entries[i].ID); err != nil {
				return fmt.Errorf("download: pause all: pause %d: %w", entries[i].ID, err)
			}
		}
	}
	return nil
}

// ResumeAll resumes every paused/failed queue entry.
func (e *Engine) ResumeAll(ctx context.Context) error {
	entries, err := e.db.ListQueueEntries(ctx, "")
	if err != nil {
		return fmt.Errorf("download: resume all: list queue: %w", err)
	}
	for i := range entries {
		if entries[i].Status == output.StatusPaused || entries[i].Status == output.StatusFailed {
			if err := e.Resume(ctx, entries[i].ID); err != nil {
				return fmt.Errorf("download: resume all: resume %d: %w", entries[i].ID, err)
			}
		}
	}
	return nil
}

// ClearQueue cancels and removes all queue entries.
func (e *Engine) ClearQueue(ctx context.Context) error {
	entries, err := e.db.ListQueueEntries(ctx, "")
	if err != nil {
		return fmt.Errorf("download: clear queue: list queue: %w", err)
	}
	for i := range entries {
		if err := e.Cancel(ctx, entries[i].ID); err != nil {
			return fmt.Errorf("download: clear queue: cancel %d: %w", entries[i].ID, err)
		}
	}
	e.publish("QueueCleared", &events.QueueCleared{
		Timestamp: time.Now().UTC(),
		Count:     len(entries),
	})
	return nil
}

// ListHistory returns history entries optionally filtered by final_status.
func (e *Engine) ListHistory(ctx context.Context, status string) ([]output.HistoryEntry, error) {
	if e.db == nil {
		return nil, fmt.Errorf("download: list history: database is nil")
	}
	return e.db.ListHistoryEntries(ctx, status)
}

// GetHistory returns one history entry by id.
func (e *Engine) GetHistory(ctx context.Context, id int64) (*output.HistoryEntry, error) {
	if e.db == nil {
		return nil, fmt.Errorf("download: get history: database is nil")
	}
	return e.db.GetHistoryEntry(ctx, id)
}

// DeleteHistory removes one history entry by id.
func (e *Engine) DeleteHistory(ctx context.Context, id int64) error {
	if e.db == nil {
		return fmt.Errorf("download: delete history: database is nil")
	}
	return e.db.DeleteHistoryEntry(ctx, id)
}

// ClearHistory removes all history entries.
func (e *Engine) ClearHistory(ctx context.Context) error {
	if e.db == nil {
		return fmt.Errorf("download: clear history: database is nil")
	}
	items, err := e.db.ListHistoryEntries(ctx, "")
	if err != nil {
		return fmt.Errorf("download: clear history: list: %w", err)
	}
	for i := range items {
		if err := e.db.DeleteHistoryEntry(ctx, items[i].ID); err != nil {
			return fmt.Errorf("download: clear history: delete %d: %w", items[i].ID, err)
		}
	}
	return nil
}
