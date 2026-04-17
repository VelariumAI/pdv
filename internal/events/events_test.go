package events

import (
	"testing"
	"time"
)

func TestEventStructs(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC()
	started := DownloadStarted{ID: 1, Timestamp: now, URL: "u", Title: "t"}
	if started.ID != 1 || started.URL != "u" || started.Title != "t" {
		t.Fatal("DownloadStarted fields not retained")
	}
	progress := DownloadProgress{ID: 1, Timestamp: now, Percentage: 10.5, Speed: "1MiB/s", ETA: "5s"}
	if progress.Percentage != 10.5 || progress.Speed == "" {
		t.Fatal("DownloadProgress fields not retained")
	}
	completed := DownloadCompleted{ID: 2, Timestamp: now, URL: "u2", Title: "t2", FilePath: "/tmp/f", FileSize: 100}
	if completed.FileSize != 100 || completed.FilePath == "" {
		t.Fatal("DownloadCompleted fields not retained")
	}
	failed := DownloadFailed{ID: 3, Timestamp: now, URL: "u3", Error: "boom"}
	if failed.Error == "" {
		t.Fatal("DownloadFailed fields not retained")
	}
	paused := DownloadPaused{ID: 4, Timestamp: now}
	if paused.ID != 4 {
		t.Fatal("DownloadPaused fields not retained")
	}
	cleared := QueueCleared{Timestamp: now, Count: 9}
	if cleared.Count != 9 {
		t.Fatal("QueueCleared fields not retained")
	}
	updated := ConfigUpdated{Timestamp: now, Key: "k", Value: "v"}
	if updated.Key != "k" || updated.Value != "v" {
		t.Fatal("ConfigUpdated fields not retained")
	}
	ws := WorkerStarted{ID: 5, Timestamp: now}
	we := WorkerStopped{ID: 5, Timestamp: now}
	if ws.ID != we.ID {
		t.Fatal("WorkerStarted/WorkerStopped fields not retained")
	}
}
