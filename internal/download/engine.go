package download

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/velariumai/pdv/internal/config"
	"github.com/velariumai/pdv/internal/events"
	"github.com/velariumai/pdv/pkg/output"
)

type runDownloadFunc func(context.Context, *output.QueueEntry, *DownloadOpts, func(output.ProgressEvent)) error

var runDownload = DownloadWithProgress

// Engine orchestrates download workers, job distribution, and event publishing.
type Engine struct {
	cfg *config.Config

	workerCount int
	jobChan     chan *output.QueueEntry

	eventsMu sync.RWMutex
	events   map[string][]chan interface{}

	stateMu      sync.RWMutex
	workerStates map[int]*workerState

	lifecycleMu sync.Mutex
	started     bool
	stopped     bool

	ctx      context.Context
	cancel   context.CancelFunc
	stopOnce sync.Once
	wg       sync.WaitGroup
}

type workerState struct {
	id        int
	state     string
	currentID int64
	startedAt time.Time
}

// NewEngine creates a new Engine with the given configuration.
func NewEngine(cfg *config.Config) *Engine {
	if cfg == nil {
		cfg = config.New()
	}
	workerCount := cfg.MaxConcurrentQueue
	if workerCount <= 0 {
		workerCount = 2
	}
	return &Engine{
		cfg:          cfg,
		workerCount:  workerCount,
		jobChan:      make(chan *output.QueueEntry, workerCount*2),
		events:       make(map[string][]chan interface{}),
		workerStates: make(map[int]*workerState, workerCount),
	}
}

// Start spawns worker goroutines and initializes the engine.
func (e *Engine) Start(ctx context.Context) error {
	e.lifecycleMu.Lock()
	defer e.lifecycleMu.Unlock()
	if e.started && !e.stopped {
		return fmt.Errorf("download: engine already started")
	}
	if e.started && e.stopped {
		return fmt.Errorf("download: engine cannot be restarted")
	}
	e.ctx, e.cancel = context.WithCancel(ctx)
	e.started = true
	e.stopped = false
	for i := 0; i < e.workerCount; i++ {
		e.workerStates[i] = &workerState{id: i, state: "idle"}
		e.wg.Add(1)
		go e.workerLoop(i)
		e.publish("WorkerStarted", &events.WorkerStarted{
			ID:        i,
			Timestamp: time.Now().UTC(),
		})
	}
	return nil
}

// Stop gracefully shuts down all workers and waits until completion or timeout.
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
		e.cancel()
	})

	done := make(chan struct{})
	go func() {
		e.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("download: engine shutdown timeout: %w", ctx.Err())
	}
}

// Submit adds one job to the queue.
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
	select {
	case e.jobChan <- job:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("download: engine stopped")
	default:
		return fmt.Errorf("download: job queue full")
	}
}

// SubmitBatch adds multiple jobs to the queue.
func (e *Engine) SubmitBatch(jobs []*output.QueueEntry) error {
	for _, job := range jobs {
		if err := e.Submit(job); err != nil {
			return err
		}
	}
	return nil
}

// Workers returns a deterministic snapshot of current worker status.
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
		s := output.WorkerStatus{ID: ws.id, State: ws.state}
		if ws.currentID != 0 {
			s.CurrentID = ws.currentID
			if !ws.startedAt.IsZero() {
				startedAt := ws.startedAt
				s.StartedAt = &startedAt
			}
		}
		result = append(result, s)
	}
	return result
}

func (e *Engine) workerLoop(id int) {
	defer e.wg.Done()
	defer e.publish("WorkerStopped", &events.WorkerStopped{
		ID:        id,
		Timestamp: time.Now().UTC(),
	})
	for {
		select {
		case <-e.ctx.Done():
			return
		case job, ok := <-e.jobChan:
			if !ok {
				return
			}
			e.handleJob(id, job)
		}
	}
}

func (e *Engine) handleJob(workerID int, job *output.QueueEntry) {
	startedAt := time.Now().UTC()
	e.setWorkerState(workerID, "downloading", job.ID, startedAt)
	e.publish("DownloadStarted", &events.DownloadStarted{
		ID:        job.ID,
		Timestamp: startedAt,
		URL:       job.URL,
		Title:     job.Title,
	})

	opts := &DownloadOpts{
		Quality:   e.cfg.DefaultQuality,
		Format:    "",
		Template:  e.cfg.OutputTemplate,
		Cookies:   e.cfg.CookieFile,
		Proxy:     e.cfg.Proxy,
		UserAgent: e.cfg.UserAgent,
	}
	err := runDownload(e.ctx, job, opts, func(ev output.ProgressEvent) {
		e.publish("DownloadProgress", &events.DownloadProgress{
			ID:         ev.ID,
			Timestamp:  time.Now().UTC(),
			Percentage: ev.Percentage,
			Speed:      ev.Speed,
			ETA:        ev.ETA,
		})
	})
	if err != nil {
		e.publish("DownloadFailed", &events.DownloadFailed{
			ID:        job.ID,
			Timestamp: time.Now().UTC(),
			URL:       job.URL,
			Error:     err.Error(),
		})
		e.setWorkerState(workerID, "idle", 0, time.Time{})
		return
	}

	e.publish("DownloadCompleted", &events.DownloadCompleted{
		ID:        job.ID,
		Timestamp: time.Now().UTC(),
		URL:       job.URL,
		Title:     job.Title,
		FilePath:  BuildOutputPath(opts.Template, job.Title, "mp4", ""),
		FileSize:  0,
	})
	e.setWorkerState(workerID, "idle", 0, time.Time{})
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

// Subscribe registers a channel for events and returns an unsubscribe callback.
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
