package download

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/velariumai/pdv/internal/database"
	"github.com/velariumai/pdv/pkg/output"
)

type Queue struct {
	db *database.DB
}

func NewQueue(db *database.DB) *Queue {
	return &Queue{db: db}
}

func (q *Queue) Add(ctx context.Context, url string, opts *output.AddOpts) (*output.QueueEntry, error) {
	if q.db == nil {
		return nil, fmt.Errorf("download: queue add: database is nil")
	}
	if url == "" {
		return nil, fmt.Errorf("download: queue add: url is empty")
	}
	var options string
	if opts != nil {
		raw, _ := json.Marshal(opts)
		options = string(raw)
	}
	now := time.Now().UTC()
	entry := &output.QueueEntry{
		URL:       url,
		Options:   options,
		Status:    output.StatusPending,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if _, err := q.db.CreateQueueEntry(ctx, entry); err != nil {
		return nil, err
	}
	return entry, nil
}

func (q *Queue) TransitionToActive(ctx context.Context, id int64, workerID int) error {
	entry, err := q.db.GetQueueEntry(ctx, id)
	if err != nil {
		return err
	}
	if entry == nil {
		// Direct submissions used by tests/ephemeral flows may not persist queue rows.
		return nil
	}
	now := time.Now().UTC()
	entry.Status = output.StatusActive
	entry.WorkerID = workerID
	entry.StartedAt = &now
	return q.db.UpdateQueueEntry(ctx, entry)
}

func (q *Queue) UpdateProgress(ctx context.Context, id int64, workerID int, progress float64) error {
	entry, err := q.db.GetQueueEntry(ctx, id)
	if err != nil {
		return err
	}
	if entry == nil {
		return nil
	}
	entry.Progress = progress
	entry.WorkerID = workerID
	return q.db.UpdateQueueEntry(ctx, entry)
}

func (q *Queue) Pause(ctx context.Context, id int64, cancel func()) error {
	entry, err := q.db.GetQueueEntry(ctx, id)
	if err != nil {
		return err
	}
	if entry == nil {
		return fmt.Errorf("download: queue pause: id %d not found", id)
	}
	if cancel != nil {
		cancel()
	}
	entry.Status = output.StatusPaused
	entry.WorkerID = 0
	entry.StartedAt = nil
	return q.db.UpdateQueueEntry(ctx, entry)
}

func (q *Queue) Resume(ctx context.Context, id int64, submit func(*output.QueueEntry) error) error {
	entry, err := q.db.GetQueueEntry(ctx, id)
	if err != nil {
		return err
	}
	if entry == nil {
		return fmt.Errorf("download: queue resume: id %d not found", id)
	}
	entry.Status = output.StatusPending
	entry.WorkerID = 0
	entry.StartedAt = nil
	entry.ErrorMsg = ""
	if err := q.db.UpdateQueueEntry(ctx, entry); err != nil {
		return err
	}
	if submit != nil {
		return submit(entry)
	}
	return nil
}

func (q *Queue) Retry(ctx context.Context, id int64, submit func(*output.QueueEntry) error, cancel func()) error {
	entry, err := q.db.GetQueueEntry(ctx, id)
	if err != nil {
		return err
	}
	if entry == nil {
		return fmt.Errorf("download: queue retry: id %d not found", id)
	}
	if cancel != nil {
		cancel()
	}
	entry.Status = output.StatusPending
	entry.WorkerID = 0
	entry.StartedAt = nil
	entry.ErrorMsg = ""
	entry.RetryCount = 0
	if err := q.db.UpdateQueueEntry(ctx, entry); err != nil {
		return err
	}
	if submit != nil {
		return submit(entry)
	}
	return nil
}

func (q *Queue) Cancel(ctx context.Context, id int64, cancel func()) error {
	entry, err := q.db.GetQueueEntry(ctx, id)
	if err != nil {
		return err
	}
	if entry == nil {
		return nil
	}
	if cancel != nil {
		cancel()
	}
	entry.Status = output.StatusCancelled
	entry.WorkerID = 0
	entry.StartedAt = nil
	if err := q.db.UpdateQueueEntry(ctx, entry); err != nil {
		return err
	}
	return q.db.DeleteQueueEntry(ctx, id)
}

func (q *Queue) MarkCompleted(ctx context.Context, id int64, status output.DownloadStatus, errMsg string) error {
	entry, err := q.db.GetQueueEntry(ctx, id)
	if err != nil {
		return err
	}
	if entry == nil {
		return nil
	}
	now := time.Now().UTC()
	entry.Status = status
	entry.ErrorMsg = errMsg
	entry.CompletedAt = &now
	entry.WorkerID = 0
	if err := q.db.UpdateQueueEntry(ctx, entry); err != nil {
		return err
	}
	return q.db.DeleteQueueEntry(ctx, id)
}

func (q *Queue) Rescan(ctx context.Context) ([]output.QueueEntry, error) {
	return q.db.ListQueueEntries(ctx, "")
}
