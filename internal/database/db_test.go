package database

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/velariumai/pdv/pkg/output"
)

func testDB(t *testing.T) *DB {
	t.Helper()
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "pdv.db")
	db, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	})
	return db
}

func TestOpenAndMigrate(t *testing.T) {
	t.Parallel()
	db := testDB(t)
	ctx := context.Background()
	for _, table := range []string{"queue", "history", "files", "config", "schema_migrations"} {
		var count int
		query := "SELECT COUNT(1) FROM sqlite_master WHERE type='table' AND name=?"
		if err := db.sql.QueryRowContext(ctx, query, table).Scan(&count); err != nil {
			t.Fatalf("table check for %s failed: %v", table, err)
		}
		if count != 1 {
			t.Fatalf("table %s count = %d, want 1", table, count)
		}
	}
}

func TestQueueCRUD(t *testing.T) {
	t.Parallel()
	db := testDB(t)
	ctx := context.Background()
	item := &output.QueueEntry{
		URL:      "https://example.com/video",
		Title:    "Video",
		Status:   output.StatusPending,
		Progress: 0,
	}
	id, err := db.CreateQueueEntry(ctx, item)
	if err != nil {
		t.Fatalf("CreateQueueEntry() error = %v", err)
	}
	got, err := db.GetQueueEntry(ctx, id)
	if err != nil {
		t.Fatalf("GetQueueEntry() error = %v", err)
	}
	if got == nil || got.URL != item.URL {
		t.Fatalf("GetQueueEntry() got = %#v, want URL %q", got, item.URL)
	}
	got.Status = output.StatusActive
	got.Progress = 50.5
	if err := db.UpdateQueueEntry(ctx, got); err != nil {
		t.Fatalf("UpdateQueueEntry() error = %v", err)
	}
	list, err := db.ListQueueEntries(ctx, "active")
	if err != nil {
		t.Fatalf("ListQueueEntries() error = %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("ListQueueEntries(active) len = %d, want 1", len(list))
	}
	if err := db.DeleteQueueEntry(ctx, id); err != nil {
		t.Fatalf("DeleteQueueEntry() error = %v", err)
	}
	missing, err := db.GetQueueEntry(ctx, id)
	if err != nil {
		t.Fatalf("GetQueueEntry(after delete) error = %v", err)
	}
	if missing != nil {
		t.Fatalf("GetQueueEntry(after delete) = %#v, want nil", missing)
	}
}

func TestHistoryCRUD(t *testing.T) {
	t.Parallel()
	db := testDB(t)
	ctx := context.Background()
	item := &output.HistoryEntry{
		URL:          "https://example.com/audio",
		Title:        "Audio",
		FinalStatus:  "completed",
		FilePath:     "/tmp/audio.mp3",
		FileSize:     123,
		Category:     "audio",
		DownloadedAt: time.Now().UTC(),
	}
	id, err := db.CreateHistoryEntry(ctx, item)
	if err != nil {
		t.Fatalf("CreateHistoryEntry() error = %v", err)
	}
	got, err := db.GetHistoryEntry(ctx, id)
	if err != nil {
		t.Fatalf("GetHistoryEntry() error = %v", err)
	}
	if got == nil || got.Title != item.Title {
		t.Fatalf("GetHistoryEntry() got = %#v, want title %q", got, item.Title)
	}
	got.Category = "document"
	if err := db.UpdateHistoryEntry(ctx, got); err != nil {
		t.Fatalf("UpdateHistoryEntry() error = %v", err)
	}
	list, err := db.ListHistoryEntries(ctx, "completed")
	if err != nil {
		t.Fatalf("ListHistoryEntries() error = %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("ListHistoryEntries(completed) len = %d, want 1", len(list))
	}
	if err := db.DeleteHistoryEntry(ctx, id); err != nil {
		t.Fatalf("DeleteHistoryEntry() error = %v", err)
	}
	missing, err := db.GetHistoryEntry(ctx, id)
	if err != nil {
		t.Fatalf("GetHistoryEntry(after delete) error = %v", err)
	}
	if missing != nil {
		t.Fatalf("GetHistoryEntry(after delete) = %#v, want nil", missing)
	}
}

func TestFileCRUD(t *testing.T) {
	t.Parallel()
	db := testDB(t)
	ctx := context.Background()
	item := &output.FileEntry{
		HistoryID: 1,
		Filename:  "video.mp4",
		Ext:       "mp4",
		SizeBytes: 200,
		MimeType:  "video/mp4",
		CreatedAt: time.Now().UTC(),
	}
	id, err := db.CreateFileEntry(ctx, item)
	if err != nil {
		t.Fatalf("CreateFileEntry() error = %v", err)
	}
	got, err := db.GetFileEntry(ctx, id)
	if err != nil {
		t.Fatalf("GetFileEntry() error = %v", err)
	}
	if got == nil || got.Filename != item.Filename {
		t.Fatalf("GetFileEntry() got = %#v, want filename %q", got, item.Filename)
	}
	got.MimeType = "application/octet-stream"
	if err := db.UpdateFileEntry(ctx, got); err != nil {
		t.Fatalf("UpdateFileEntry() error = %v", err)
	}
	list, err := db.ListFileEntries(ctx, 1)
	if err != nil {
		t.Fatalf("ListFileEntries() error = %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("ListFileEntries(historyID=1) len = %d, want 1", len(list))
	}
	if err := db.DeleteFileEntry(ctx, id); err != nil {
		t.Fatalf("DeleteFileEntry() error = %v", err)
	}
	missing, err := db.GetFileEntry(ctx, id)
	if err != nil {
		t.Fatalf("GetFileEntry(after delete) error = %v", err)
	}
	if missing != nil {
		t.Fatalf("GetFileEntry(after delete) = %#v, want nil", missing)
	}
}

func TestOpenInvalidPath(t *testing.T) {
	t.Parallel()
	_, err := Open(context.Background(), "/proc/pdv/blocked.db")
	if err == nil {
		t.Fatal("Open(invalid path) error = nil, want non-nil")
	}
}

func TestCloseNilDB(t *testing.T) {
	t.Parallel()
	var db *DB
	if err := db.Close(); err != nil {
		t.Fatalf("nil db Close() error = %v", err)
	}
}

func TestMethodsFailAfterClose(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := testDB(t)
	if err := db.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	queue := &output.QueueEntry{URL: "https://example.com", Status: output.StatusPending}
	if _, err := db.CreateQueueEntry(ctx, queue); err == nil {
		t.Fatal("CreateQueueEntry() on closed db error = nil, want non-nil")
	}
	if _, err := db.ListQueueEntries(ctx, ""); err == nil {
		t.Fatal("ListQueueEntries() on closed db error = nil, want non-nil")
	}
	if _, err := db.GetQueueEntry(ctx, 1); err == nil {
		t.Fatal("GetQueueEntry() on closed db error = nil, want non-nil")
	}
	if err := db.UpdateQueueEntry(ctx, &output.QueueEntry{ID: 1}); err == nil {
		t.Fatal("UpdateQueueEntry() on closed db error = nil, want non-nil")
	}
	if err := db.DeleteQueueEntry(ctx, 1); err == nil {
		t.Fatal("DeleteQueueEntry() on closed db error = nil, want non-nil")
	}
	history := &output.HistoryEntry{URL: "https://example.com", FinalStatus: "failed", DownloadedAt: time.Now().UTC()}
	if _, err := db.CreateHistoryEntry(ctx, history); err == nil {
		t.Fatal("CreateHistoryEntry() on closed db error = nil, want non-nil")
	}
	if _, err := db.ListHistoryEntries(ctx, ""); err == nil {
		t.Fatal("ListHistoryEntries() on closed db error = nil, want non-nil")
	}
	if _, err := db.GetHistoryEntry(ctx, 1); err == nil {
		t.Fatal("GetHistoryEntry() on closed db error = nil, want non-nil")
	}
	if err := db.UpdateHistoryEntry(ctx, &output.HistoryEntry{ID: 1, DownloadedAt: time.Now().UTC()}); err == nil {
		t.Fatal("UpdateHistoryEntry() on closed db error = nil, want non-nil")
	}
	if err := db.DeleteHistoryEntry(ctx, 1); err == nil {
		t.Fatal("DeleteHistoryEntry() on closed db error = nil, want non-nil")
	}
	file := &output.FileEntry{HistoryID: 1, Filename: "x", CreatedAt: time.Now().UTC()}
	if _, err := db.CreateFileEntry(ctx, file); err == nil {
		t.Fatal("CreateFileEntry() on closed db error = nil, want non-nil")
	}
	if _, err := db.ListFileEntries(ctx, 0); err == nil {
		t.Fatal("ListFileEntries() on closed db error = nil, want non-nil")
	}
	if _, err := db.GetFileEntry(ctx, 1); err == nil {
		t.Fatal("GetFileEntry() on closed db error = nil, want non-nil")
	}
	if err := db.UpdateFileEntry(ctx, &output.FileEntry{ID: 1, HistoryID: 1, Filename: "x", CreatedAt: time.Now().UTC()}); err == nil {
		t.Fatal("UpdateFileEntry() on closed db error = nil, want non-nil")
	}
	if err := db.DeleteFileEntry(ctx, 1); err == nil {
		t.Fatal("DeleteFileEntry() on closed db error = nil, want non-nil")
	}
}

func TestMigrateOnClosedDB(t *testing.T) {
	t.Parallel()
	db := testDB(t)
	if err := db.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if err := db.Migrate(context.Background()); err == nil {
		t.Fatal("Migrate() on closed db error = nil, want non-nil")
	}
	if _, err := db.isApplied(context.Background(), 1); err == nil {
		t.Fatal("isApplied() on closed db error = nil, want non-nil")
	}
	if err := db.markApplied(context.Background(), 2); err == nil {
		t.Fatal("markApplied() on closed db error = nil, want non-nil")
	}
}

func TestCreateQueueEntryDefaultStatus(t *testing.T) {
	t.Parallel()
	db := testDB(t)
	ctx := context.Background()
	entry := &output.QueueEntry{URL: "https://example.com/default-status"}
	if _, err := db.CreateQueueEntry(ctx, entry); err != nil {
		t.Fatalf("CreateQueueEntry() error = %v", err)
	}
	if entry.Status != output.StatusPending {
		t.Fatalf("entry.Status = %q, want %q", entry.Status, output.StatusPending)
	}
}

func TestMigrationTrackingHelpers(t *testing.T) {
	t.Parallel()
	db := testDB(t)
	ctx := context.Background()
	applied, err := db.isApplied(ctx, 1)
	if err != nil {
		t.Fatalf("isApplied(1) error = %v", err)
	}
	if !applied {
		t.Fatal("isApplied(1) = false, want true")
	}
	if err := db.markApplied(ctx, 999); err != nil {
		t.Fatalf("markApplied(999) error = %v", err)
	}
	applied, err = db.isApplied(ctx, 999)
	if err != nil {
		t.Fatalf("isApplied(999) error = %v", err)
	}
	if !applied {
		t.Fatal("isApplied(999) = false, want true")
	}
}
