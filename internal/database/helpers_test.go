package database

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/velariumai/pdv/pkg/output"
)

type fakeScanner struct {
	scanFn func(dest ...any) error
}

func (f fakeScanner) Scan(dest ...any) error {
	return f.scanFn(dest...)
}

func TestHelpers(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC().Round(0)
	text := toText(now)
	parsed, err := parseTextTime(text)
	if err != nil {
		t.Fatalf("parseTextTime(valid) error = %v", err)
	}
	if parsed.Format(time.RFC3339Nano) != now.Format(time.RFC3339Nano) {
		t.Fatalf("parseTextTime(valid) mismatch")
	}
	if _, err := parseTextTime("bad-time"); err == nil {
		t.Fatal("parseTextTime(invalid) error = nil, want non-nil")
	}
	nilTime, err := parseNullableTime(sql.NullString{})
	if err != nil {
		t.Fatalf("parseNullableTime(nil) error = %v", err)
	}
	if nilTime != nil {
		t.Fatal("parseNullableTime(nil) != nil")
	}
	validNull := toNullableTime(&now)
	got, err := parseNullableTime(validNull)
	if err != nil {
		t.Fatalf("parseNullableTime(valid) error = %v", err)
	}
	if got == nil {
		t.Fatal("parseNullableTime(valid) returned nil")
	}
	if _, err := parseNullableTime(sql.NullString{String: "broken", Valid: true}); err == nil {
		t.Fatal("parseNullableTime(invalid) error = nil, want non-nil")
	}
	if nullInt(0).Valid {
		t.Fatal("nullInt(0).Valid = true, want false")
	}
	if !nullInt(5).Valid {
		t.Fatal("nullInt(5).Valid = false, want true")
	}
}

func TestScanQueueErrors(t *testing.T) {
	t.Parallel()
	base := fakeScanner{
		scanFn: func(dest ...any) error {
			*dest[0].(*int64) = 1
			*dest[1].(*string) = "u"
			*dest[2].(*string) = "t"
			*dest[3].(*string) = "{}"
			*dest[4].(*output.DownloadStatus) = output.StatusPending
			*dest[5].(*float64) = 0
			*dest[6].(*string) = ""
			*dest[7].(*int) = 0
			*dest[8].(*sql.NullInt64) = sql.NullInt64{}
			*dest[9].(*string) = "bad-time"
			*dest[10].(*string) = time.Now().UTC().Format(time.RFC3339Nano)
			*dest[11].(*sql.NullString) = sql.NullString{}
			*dest[12].(*sql.NullString) = sql.NullString{}
			return nil
		},
	}
	if _, err := scanQueue(base); err == nil {
		t.Fatal("scanQueue(invalid time) error = nil, want non-nil")
	}
	errScan := errors.New("scan failed")
	if _, err := scanQueue(fakeScanner{scanFn: func(dest ...any) error { return errScan }}); err == nil {
		t.Fatal("scanQueue(scan error) error = nil, want non-nil")
	}
}

func TestScanQueueHappyPathWithOptionalFields(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	q := fakeScanner{
		scanFn: func(dest ...any) error {
			*dest[0].(*int64) = 10
			*dest[1].(*string) = "https://example.com/item"
			*dest[2].(*string) = "Item"
			*dest[3].(*string) = `{"k":"v"}`
			*dest[4].(*output.DownloadStatus) = output.StatusActive
			*dest[5].(*float64) = 90
			*dest[6].(*string) = ""
			*dest[7].(*int) = 1
			*dest[8].(*sql.NullInt64) = sql.NullInt64{Int64: 2, Valid: true}
			*dest[9].(*string) = now
			*dest[10].(*string) = now
			*dest[11].(*sql.NullString) = sql.NullString{String: now, Valid: true}
			*dest[12].(*sql.NullString) = sql.NullString{String: now, Valid: true}
			return nil
		},
	}
	item, err := scanQueue(q)
	if err != nil {
		t.Fatalf("scanQueue(happy path) error = %v", err)
	}
	if item.WorkerID != 2 {
		t.Fatalf("WorkerID = %d, want 2", item.WorkerID)
	}
	if item.StartedAt == nil || item.CompletedAt == nil {
		t.Fatal("StartedAt/CompletedAt nil, want non-nil")
	}
}

func TestScanHistoryAndFileErrors(t *testing.T) {
	t.Parallel()
	history := fakeScanner{
		scanFn: func(dest ...any) error {
			*dest[0].(*int64) = 1
			*dest[1].(*string) = "u"
			*dest[2].(*string) = "t"
			*dest[3].(*string) = "completed"
			*dest[4].(*string) = "f"
			*dest[5].(*int64) = 1
			*dest[6].(*string) = "video"
			*dest[7].(*string) = ""
			*dest[8].(*string) = "bad-time"
			*dest[9].(*int) = 0
			return nil
		},
	}
	if _, err := scanHistory(history); err == nil {
		t.Fatal("scanHistory(invalid time) error = nil, want non-nil")
	}
	file := fakeScanner{
		scanFn: func(dest ...any) error {
			*dest[0].(*int64) = 1
			*dest[1].(*int64) = 1
			*dest[2].(*string) = "x"
			*dest[3].(*string) = "mp4"
			*dest[4].(*int64) = 1
			*dest[5].(*string) = "video/mp4"
			*dest[6].(*string) = "bad-time"
			return nil
		},
	}
	if _, err := scanFile(file); err == nil {
		t.Fatal("scanFile(invalid time) error = nil, want non-nil")
	}
}

func TestMigrateIdempotent(t *testing.T) {
	t.Parallel()
	db := testDB(t)
	ctx := context.Background()
	if err := db.Migrate(ctx); err != nil {
		t.Fatalf("Migrate(second run) error = %v", err)
	}
}
