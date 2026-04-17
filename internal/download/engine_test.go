package download

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/velariumai/pdv/internal/config"
	"github.com/velariumai/pdv/internal/events"
	"github.com/velariumai/pdv/pkg/output"
)

func TestEngineLifecycleAndWorkers(t *testing.T) {
	cfg := config.New()
	cfg.MaxConcurrentQueue = 2
	engine := NewEngine(cfg)
	if err := engine.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	workers := engine.Workers()
	if len(workers) != 2 {
		t.Fatalf("Workers() len = %d, want 2", len(workers))
	}
	if err := engine.Start(context.Background()); err == nil {
		t.Fatal("Start(second) error = nil, want non-nil")
	}
	stopCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := engine.Stop(stopCtx); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
	if err := engine.Stop(stopCtx); err != nil {
		t.Fatalf("Stop(second) should be idempotent, got error: %v", err)
	}
}

func TestEngineSubmitValidation(t *testing.T) {
	engine := NewEngine(config.New())
	if err := engine.Submit(&output.QueueEntry{ID: 1, URL: "x"}); err == nil {
		t.Fatal("Submit(before start) error = nil, want non-nil")
	}
	if err := engine.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer engine.Stop(context.Background())
	if err := engine.Submit(nil); err == nil {
		t.Fatal("Submit(nil) error = nil, want non-nil")
	}
	if err := engine.SubmitBatch([]*output.QueueEntry{{ID: 1, URL: "a"}, {ID: 2, URL: "b"}}); err != nil {
		t.Fatalf("SubmitBatch() error = %v", err)
	}
}

func TestEngineEventsAndDownloadFlow(t *testing.T) {
	orig := runDownload
	t.Cleanup(func() { runDownload = orig })
	runDownload = func(ctx context.Context, entry *output.QueueEntry, opts *DownloadOpts, onProgress func(output.ProgressEvent)) error {
		onProgress(output.ProgressEvent{ID: entry.ID, Percentage: 45, Speed: "1MiB/s", ETA: "2s"})
		onProgress(output.ProgressEvent{ID: entry.ID, Percentage: 100, Speed: "2MiB/s", ETA: "0"})
		return nil
	}
	cfg := config.New()
	cfg.MaxConcurrentQueue = 1
	cfg.OutputTemplate = "%(title)s.%(ext)s"
	engine := NewEngine(cfg)
	if err := engine.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer engine.Stop(context.Background())
	startedCh := make(chan interface{}, 4)
	progressCh := make(chan interface{}, 4)
	completedCh := make(chan interface{}, 4)
	engine.Subscribe("DownloadStarted", startedCh)
	engine.Subscribe("DownloadProgress", progressCh)
	engine.Subscribe("DownloadCompleted", completedCh)

	job := &output.QueueEntry{ID: 11, URL: "https://example.com", Title: "test video"}
	if err := engine.Submit(job); err != nil {
		t.Fatalf("Submit() error = %v", err)
	}
	waitEvent(t, startedCh, func(v interface{}) {
		if _, ok := v.(*events.DownloadStarted); !ok {
			t.Fatalf("DownloadStarted event type mismatch: %T", v)
		}
	})
	waitEvent(t, progressCh, func(v interface{}) {
		if ev, ok := v.(*events.DownloadProgress); !ok || ev.ID != 11 {
			t.Fatalf("DownloadProgress event mismatch: %#v", v)
		}
	})
	waitEvent(t, completedCh, func(v interface{}) {
		ev, ok := v.(*events.DownloadCompleted)
		if !ok {
			t.Fatalf("DownloadCompleted event type mismatch: %T", v)
		}
		if ev.FilePath == "" {
			t.Fatal("DownloadCompleted.FilePath empty")
		}
	})
}

func TestEngineDownloadFailurePublishesFailed(t *testing.T) {
	orig := runDownload
	t.Cleanup(func() { runDownload = orig })
	runDownload = func(context.Context, *output.QueueEntry, *DownloadOpts, func(output.ProgressEvent)) error {
		return errors.New("download failed")
	}
	cfg := config.New()
	cfg.MaxConcurrentQueue = 1
	engine := NewEngine(cfg)
	if err := engine.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer engine.Stop(context.Background())
	failedCh := make(chan interface{}, 2)
	engine.Subscribe("DownloadFailed", failedCh)
	if err := engine.Submit(&output.QueueEntry{ID: 12, URL: "https://example.com/f"}); err != nil {
		t.Fatalf("Submit() error = %v", err)
	}
	waitEvent(t, failedCh, func(v interface{}) {
		ev, ok := v.(*events.DownloadFailed)
		if !ok || ev.ID != 12 {
			t.Fatalf("DownloadFailed event mismatch: %#v", v)
		}
	})
}

func TestPublishNonBlocking(t *testing.T) {
	engine := NewEngine(config.New())
	if err := engine.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer engine.Stop(context.Background())
	ch := make(chan interface{})
	engine.Subscribe("X", ch)
	done := make(chan struct{})
	go func() {
		engine.publish("X", &events.DownloadStarted{ID: 1, Timestamp: time.Now().UTC()})
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("publish blocked on unbuffered subscriber")
	}
}

func waitEvent(t *testing.T, ch <-chan interface{}, assert func(interface{})) {
	t.Helper()
	select {
	case ev := <-ch:
		assert(ev)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for event")
	}
}
