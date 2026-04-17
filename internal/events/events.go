package events

import "time"

// DownloadStarted is published when a download begins.
type DownloadStarted struct {
	ID        int64
	Timestamp time.Time
	URL       string
	Title     string
}

// DownloadProgress is published as a download makes progress.
type DownloadProgress struct {
	ID         int64
	Timestamp  time.Time
	Percentage float64
	Speed      string
	ETA        string
}

// DownloadCompleted is published when a download finishes successfully.
type DownloadCompleted struct {
	ID        int64
	Timestamp time.Time
	URL       string
	Title     string
	FilePath  string
	FileSize  int64
}

// DownloadFailed is published when a download fails.
type DownloadFailed struct {
	ID        int64
	Timestamp time.Time
	URL       string
	Error     string
}

// DownloadPaused is published when a download is paused.
type DownloadPaused struct {
	ID        int64
	Timestamp time.Time
}

// QueueCleared is published when the entire queue is cleared.
type QueueCleared struct {
	Timestamp time.Time
	Count     int
}

// ConfigUpdated is published when configuration changes.
type ConfigUpdated struct {
	Timestamp time.Time
	Key       string
	Value     string
}

// WorkerStarted is published when a worker goroutine starts.
type WorkerStarted struct {
	ID        int
	Timestamp time.Time
}

// WorkerStopped is published when a worker goroutine stops.
type WorkerStopped struct {
	ID        int
	Timestamp time.Time
}
