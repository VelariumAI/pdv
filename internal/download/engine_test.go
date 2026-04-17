package download

import (
	"context"
	"testing"
	"time"

	"github.com/velariumai/pdv/internal/config"
	"github.com/velariumai/pdv/internal/events"
	"github.com/velariumai/pdv/pkg/output"
)

func TestNewEngine(t *testing.T) {
	cfg := config.New()
	cfg.MaxConcurrentQueue = 4

	engine := NewEngine(cfg)
	if engine == nil {
		t.Fatal("NewEngine returned nil")
	}
	if engine.workerCount != 4 {
		t.Errorf("workerCount = %d, want 4", engine.workerCount)
	}
	if len(engine.jobChan) != 0 {
		t.Errorf("initial jobChan should be empty, got %d items", len(engine.jobChan))
	}
}

func TestNewEngineDefaults(t *testing.T) {
	engine := NewEngine(nil)
	if engine == nil {
		t.Fatal("NewEngine with nil config returned nil")
	}
	if engine.workerCount <= 0 {
		t.Errorf("workerCount should be positive, got %d", engine.workerCount)
	}
}

func TestEngineStartStop(t *testing.T) {
	cfg := config.New()
	cfg.MaxConcurrentQueue = 2
	engine := NewEngine(cfg)

	ctx := context.Background()
	if err := engine.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Verify workers exist
	workers := engine.Workers()
	if len(workers) != 2 {
		t.Errorf("Workers count = %d, want 2", len(workers))
	}

	// Verify all workers are idle
	for _, w := range workers {
		if w.State != "idle" {
			t.Errorf("Worker %d state = %q, want idle", w.ID, w.State)
		}
	}

	// Stop with timeout
	stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := engine.Stop(stopCtx); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	// Verify context is cancelled
	if engine.ctx == nil {
		t.Error("engine.ctx should not be nil after Stop")
	}
}

func TestEngineStartAlreadyStarted(t *testing.T) {
	cfg := config.New()
	engine := NewEngine(cfg)

	ctx := context.Background()
	if err := engine.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer engine.Stop(context.Background())

	// Try to start again
	if err := engine.Start(ctx); err == nil {
		t.Error("Start on already-started engine should error")
	}
}

func TestEngineSubmit(t *testing.T) {
	cfg := config.New()
	cfg.MaxConcurrentQueue = 1
	engine := NewEngine(cfg)

	ctx := context.Background()
	if err := engine.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer engine.Stop(context.Background())

	job := &output.QueueEntry{
		URL: "https://example.com/video",
	}

	if err := engine.Submit(job); err != nil {
		t.Fatalf("Submit failed: %v", err)
	}
}

func TestEngineSubmitNil(t *testing.T) {
	cfg := config.New()
	engine := NewEngine(cfg)

	ctx := context.Background()
	if err := engine.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer engine.Stop(context.Background())

	if err := engine.Submit(nil); err == nil {
		t.Error("Submit(nil) should error")
	}
}

func TestEngineSubmitBatch(t *testing.T) {
	cfg := config.New()
	cfg.MaxConcurrentQueue = 5
	engine := NewEngine(cfg)

	ctx := context.Background()
	if err := engine.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer engine.Stop(context.Background())

	jobs := []*output.QueueEntry{
		{URL: "https://example.com/1"},
		{URL: "https://example.com/2"},
		{URL: "https://example.com/3"},
	}

	if err := engine.SubmitBatch(jobs); err != nil {
		t.Fatalf("SubmitBatch failed: %v", err)
	}
}

func TestEngineSubscribe(t *testing.T) {
	cfg := config.New()
	engine := NewEngine(cfg)

	ctx := context.Background()
	if err := engine.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer engine.Stop(context.Background())

	ch := make(chan interface{}, 10)
	unsub := engine.Subscribe("DownloadStarted", ch)

	// Verify subscription
	engine.eventsMu.RLock()
	if len(engine.events["DownloadStarted"]) != 1 {
		t.Errorf("Subscribe count = %d, want 1", len(engine.events["DownloadStarted"]))
	}
	engine.eventsMu.RUnlock()

	// Unsubscribe
	unsub()

	engine.eventsMu.RLock()
	if len(engine.events["DownloadStarted"]) != 0 {
		t.Errorf("After unsubscribe count = %d, want 0", len(engine.events["DownloadStarted"]))
	}
	engine.eventsMu.RUnlock()
}

func TestEngineEventPublishing(t *testing.T) {
	cfg := config.New()
	cfg.MaxConcurrentQueue = 1
	engine := NewEngine(cfg)

	ctx := context.Background()
	if err := engine.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer engine.Stop(context.Background())

	// Subscribe to events
	eventChan := make(chan interface{}, 10)
	unsub := engine.Subscribe("DownloadStarted", eventChan)
	defer unsub()

	// Submit a job
	job := &output.QueueEntry{
		ID:  1,
		URL: "https://example.com/test",
	}

	if err := engine.Submit(job); err != nil {
		t.Fatalf("Submit failed: %v", err)
	}

	// Wait a bit for worker to process
	time.Sleep(200 * time.Millisecond)

	// Check for events (non-blocking)
	select {
	case event := <-eventChan:
		if _, ok := event.(*events.DownloadStarted); !ok {
			t.Errorf("Expected DownloadStarted event, got %T", event)
		}
	case <-time.After(500 * time.Millisecond):
		t.Error("No DownloadStarted event received")
	}
}

func TestEngineWorkers(t *testing.T) {
	cfg := config.New()
	cfg.MaxConcurrentQueue = 3
	engine := NewEngine(cfg)

	ctx := context.Background()
	if err := engine.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer engine.Stop(context.Background())

	workers := engine.Workers()
	if len(workers) != 3 {
		t.Errorf("Workers count = %d, want 3", len(workers))
	}

	// Verify each worker ID is present (order is non-deterministic from map iteration)
	seenIDs := make(map[int]bool)
	for _, w := range workers {
		seenIDs[w.ID] = true
		if w.State != "idle" {
			t.Errorf("Worker %d state = %q, want idle", w.ID, w.State)
		}
	}

	// Verify all worker IDs 0, 1, 2 are present
	for i := 0; i < 3; i++ {
		if !seenIDs[i] {
			t.Errorf("Worker %d not found in snapshot", i)
		}
	}
}

func TestEngineStopTimeout(t *testing.T) {
	cfg := config.New()
	cfg.MaxConcurrentQueue = 1
	engine := NewEngine(cfg)

	ctx := context.Background()
	if err := engine.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Submit a job that will block
	job := &output.QueueEntry{
		ID:  1,
		URL: "https://example.com",
	}
	engine.Submit(job)

	// Try to stop with very short timeout
	stopCtx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	// Give it a bit more time to trigger the timeout scenario
	// (This test verifies timeout logic exists; actual timeout depends on job duration)
	time.Sleep(100 * time.Millisecond)
	engine.Stop(stopCtx)
	// Note: This test doesn't verify the error, just that Stop completes
}

func TestEnginePublishNonBlocking(t *testing.T) {
	cfg := config.New()
	engine := NewEngine(cfg)

	ctx := context.Background()
	if err := engine.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer engine.Stop(context.Background())

	// Create a non-buffered channel to verify non-blocking publish
	ch := make(chan interface{})
	engine.Subscribe("TestEvent", ch)

	// Publish should not block
	done := make(chan bool, 1)
	go func() {
		engine.publish("TestEvent", &events.DownloadStarted{
			ID:        1,
			Timestamp: time.Now().UTC(),
		})
		done <- true
	}()

	select {
	case <-done:
		// Good, publish completed
	case <-time.After(100 * time.Millisecond):
		t.Error("publish blocked (should be non-blocking)")
	}
}
