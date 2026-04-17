package output

import "time"

// DownloadStatus describes the lifecycle state of a queue entry.
type DownloadStatus string

const (
	// StatusPending marks a queue entry waiting to be processed.
	StatusPending DownloadStatus = "pending"
	// StatusActive marks a queue entry being processed by a worker.
	StatusActive DownloadStatus = "active"
	// StatusPaused marks a queue entry paused by user action.
	StatusPaused DownloadStatus = "paused"
	// StatusCompleted marks a queue entry that completed successfully.
	StatusCompleted DownloadStatus = "completed"
	// StatusFailed marks a queue entry that failed.
	StatusFailed DownloadStatus = "failed"
	// StatusCancelled marks a queue entry cancelled by user action.
	StatusCancelled DownloadStatus = "cancelled"
)

// QueueEntry represents one row in the queue table.
type QueueEntry struct {
	ID          int64          `json:"id"`
	URL         string         `json:"url"`
	Title       string         `json:"title,omitempty"`
	Options     string         `json:"options,omitempty"`
	Status      DownloadStatus `json:"status"`
	Progress    float64        `json:"progress"`
	ErrorMsg    string         `json:"error_msg,omitempty"`
	RetryCount  int            `json:"retry_count"`
	WorkerID    int            `json:"worker_id,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	StartedAt   *time.Time     `json:"started_at,omitempty"`
	CompletedAt *time.Time     `json:"completed_at,omitempty"`
}

// HistoryEntry represents one row in the history table.
type HistoryEntry struct {
	ID           int64     `json:"id"`
	URL          string    `json:"url"`
	Title        string    `json:"title,omitempty"`
	FinalStatus  string    `json:"final_status"`
	FilePath     string    `json:"file_path,omitempty"`
	FileSize     int64     `json:"file_size,omitempty"`
	Category     string    `json:"category,omitempty"`
	ErrorMsg     string    `json:"error_msg,omitempty"`
	DownloadedAt time.Time `json:"downloaded_at"`
	DurationSecs int       `json:"duration_secs,omitempty"`
}

// FileEntry represents one row in the files table.
type FileEntry struct {
	ID        int64     `json:"id"`
	HistoryID int64     `json:"history_id"`
	Filename  string    `json:"filename"`
	Ext       string    `json:"ext,omitempty"`
	SizeBytes int64     `json:"size_bytes,omitempty"`
	MimeType  string    `json:"mime_type,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// Format describes a downloadable format (video, audio, etc.) available from a media source.
type Format struct {
	FormatID         string `json:"format_id"`
	Resolution       string `json:"resolution,omitempty"`
	Codec            string `json:"codec,omitempty"`
	FileSizeEstimate int64  `json:"file_size_estimate,omitempty"`
	Ext              string `json:"ext"`
}

// ProbeResult contains metadata extracted by yt-dlp about a media URL.
type ProbeResult struct {
	Title        string   `json:"title"`
	Uploader     string   `json:"uploader,omitempty"`
	Duration     int      `json:"duration,omitempty"` // seconds
	UploadDate   string   `json:"upload_date,omitempty"`
	Formats      []Format `json:"formats,omitempty"`
	Subtitles    []string `json:"subtitles,omitempty"`
	ThumbnailURL string   `json:"thumbnail_url,omitempty"`
}

// ProgressEvent represents a download progress update.
type ProgressEvent struct {
	ID         int64     `json:"id"`
	Timestamp  time.Time `json:"timestamp"`
	Percentage float64   `json:"percentage"`
	Speed      string    `json:"speed,omitempty"`
	ETA        string    `json:"eta,omitempty"`
	Status     string    `json:"status,omitempty"`
}

// WorkerStatus represents the current state of a worker goroutine.
type WorkerStatus struct {
	ID        int         `json:"id"`
	State     string      `json:"state"` // "idle" | "downloading" | "paused"
	CurrentID int64       `json:"current_id,omitempty"`
	StartedAt *time.Time  `json:"started_at,omitempty"`
	Current   *QueueEntry `json:"current,omitempty"`
}
