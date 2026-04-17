package download

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/velariumai/pdv/internal/config"
	"github.com/velariumai/pdv/internal/events"
	"github.com/velariumai/pdv/pkg/output"
)

// Engine orchestrates download workers, job distribution, and event publishing.
type Engine struct {
	cfg *config.Config

	workerCount int
	jobChan     chan *output.QueueEntry

	// Event dispatcher: event type -> list of subscriber channels
	eventsMu sync.RWMutex
	events   map[string][]chan interface{}

	// Worker state tracking
	stateMu      sync.RWMutex
	workerStates map[int]*workerState

	// Lifecycle control
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// workerState tracks internal state of a worker goroutine.
type workerState struct {
	id        int
	state     string    // "idle" | "downloading"
	currentID int64     // queue entry ID being processed
	startedAt time.Time
}

// NewEngine creates a new Engine with the given configuration.
// Worker count defaults to MaxConcurrentQueue from config.
func NewEngine(cfg *config.Config) *Engine {
	if cfg == nil {
		cfg = config.New()
	}

	workerCount := cfg.MaxConcurrentQueue
	if workerCount <= 0 {
		workerCount = 2
	}

	return &Engine{
		cfg:           cfg,
		workerCount:   workerCount,
		jobChan:       make(chan *output.QueueEntry, workerCount*2),
		events:        make(map[string][]chan interface{}),
		workerStates:  make(map[int]*workerState),
	}
}

// Start spawns worker goroutines and initializes the engine.
// Workers will be ready to consume jobs immediately.
func (e *Engine) Start(ctx context.Context) error {
	if e.ctx != nil {
		return fmt.Errorf("download: engine already started")
	}

	e.ctx, e.cancel = context.WithCancel(ctx)

	// Initialize worker states
	e.stateMu.Lock()
	for i := 0; i < e.workerCount; i++ {
		e.workerStates[i] = &workerState{id: i, state: "idle"}
	}
	e.stateMu.Unlock()

	// Spawn worker goroutines
	for i := 0; i < e.workerCount; i++ {
		e.wg.Add(1)
		go e.workerLoop(i)

		// Publish WorkerStarted event
		e.publish("WorkerStarted", &events.WorkerStarted{
			ID:        i,
			Timestamp: time.Now().UTC(),
		})
	}

	return nil
}

// Stop gracefully shuts down the engine and all workers.
// Closes job channel, drains in-flight jobs, cancels context, and waits for workers.
// Returns error if shutdown exceeds timeout.
func (e *Engine) Stop(ctx context.Context) error {
	if e.ctx == nil {
		return fmt.Errorf("download: engine not started")
	}

	// Close job channel to signal workers to drain
	close(e.jobChan)

	// Cancel worker context to interrupt subprocesses
	e.cancel()

	// Wait for all workers with timeout
	done := make(chan struct{})
	go func() {
		e.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// All workers stopped
	case <-ctx.Done():
		return fmt.Errorf("download: engine shutdown timeout")
	}

	// Publish WorkerStopped events for each worker
	e.stateMu.RLock()
	for i := range e.workerStates {
		e.publish("WorkerStopped", &events.WorkerStopped{
			ID:        i,
			Timestamp: time.Now().UTC(),
		})
	}
	e.stateMu.RUnlock()

	return nil
}

// Submit adds a single job to the worker queue.
// Returns error if job channel is full or closed.
func (e *Engine) Submit(job *output.QueueEntry) error {
	if job == nil {
		return fmt.Errorf("download: job is nil")
	}

	select {
	case e.jobChan <- job:
		return nil
	case <-e.ctx.Done():
		return fmt.Errorf("download: engine stopped")
	default:
		return fmt.Errorf("download: job queue full")
	}
}

// SubmitBatch adds multiple jobs to the queue.
// Returns first error encountered, or nil if all succeed.
func (e *Engine) SubmitBatch(jobs []*output.QueueEntry) error {
	for _, job := range jobs {
		if err := e.Submit(job); err != nil {
			return err
		}
	}
	return nil
}

// Workers returns a snapshot of current worker statuses.
func (e *Engine) Workers() []output.WorkerStatus {
	e.stateMu.RLock()
	defer e.stateMu.RUnlock()

	result := make([]output.WorkerStatus, 0, len(e.workerStates))
	for _, ws := range e.workerStates {
		status := output.WorkerStatus{
			ID:    ws.id,
			State: ws.state,
		}
		if ws.currentID != 0 {
			status.CurrentID = ws.currentID
			if ws.startedAt != (time.Time{}) {
				status.StartedAt = &ws.startedAt
			}
		}
		result = append(result, status)
	}
	return result
}

// workerLoop runs in a dedicated goroutine per worker.
// Consumes jobs from jobChan, executes them, and publishes events.
func (e *Engine) workerLoop(id int) {
	defer e.wg.Done()

	for job := range e.jobChan {
		// Update state: mark as downloading
		e.stateMu.Lock()
		ws := e.workerStates[id]
		ws.state = "downloading"
		ws.currentID = job.ID
		ws.startedAt = time.Now().UTC()
		e.stateMu.Unlock()

		// Publish DownloadStarted event
		e.publish("DownloadStarted", &events.DownloadStarted{
			ID:        job.ID,
			Timestamp: time.Now().UTC(),
			URL:       job.URL,
			Title:     job.Title,
		})

		// Execute download
		// For now, this is a placeholder that logs success.
		// Tranche 3 will wire this to the database layer.
		// Tranche 2 focus: verify the engine and event plumbing work.

		// Simulate download completion (will be replaced in Tranche 3)
		time.Sleep(100 * time.Millisecond)

		// Publish DownloadCompleted event
		e.publish("DownloadCompleted", &events.DownloadCompleted{
			ID:        job.ID,
			Timestamp: time.Now().UTC(),
			URL:       job.URL,
			Title:     job.Title,
			FilePath:  "/downloads/" + job.Title,
			FileSize:  1024,
		})

		// Update state: mark as idle
		e.stateMu.Lock()
		ws.state = "idle"
		ws.currentID = 0
		ws.startedAt = time.Time{}
		e.stateMu.Unlock()
	}
}

// publish sends an event to all registered subscribers for that event type.
// Non-blocking: if a subscriber's channel is full, the event is dropped
// (subscriber should use buffered channels or goroutine to prevent blocking).
func (e *Engine) publish(eventType string, event interface{}) {
	e.eventsMu.RLock()
	subscribers := e.events[eventType]
	e.eventsMu.RUnlock()

	for _, ch := range subscribers {
		select {
		case ch <- event:
		default:
			// Subscriber channel full; drop event to avoid blocking
		}
	}
}

// Subscribe registers a channel to receive events of the given type.
// Returns a function to unsubscribe.
func (e *Engine) Subscribe(eventType string, ch chan interface{}) func() {
	e.eventsMu.Lock()
	if e.events[eventType] == nil {
		e.events[eventType] = make([]chan interface{}, 0)
	}
	e.events[eventType] = append(e.events[eventType], ch)
	e.eventsMu.Unlock()

	return func() {
		e.eventsMu.Lock()
		subscribers := e.events[eventType]
		for i, sub := range subscribers {
			if sub == ch {
				// Remove by swapping with last and truncating
				e.events[eventType] = append(subscribers[:i], subscribers[i+1:]...)
				break
			}
		}
		e.eventsMu.Unlock()
	}
}
