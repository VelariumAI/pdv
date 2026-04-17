package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/velariumai/pdv/pkg/output"
)

// CreateQueueEntry inserts a queue row and returns the new id.
func (db *DB) CreateQueueEntry(ctx context.Context, e *output.QueueEntry) (int64, error) {
	now := time.Now().UTC()
	if e.Status == "" {
		e.Status = output.StatusPending
	}
	e.CreatedAt = now
	e.UpdatedAt = now
	res, err := db.sql.ExecContext(
		ctx,
		`INSERT INTO queue(url,title,options,status,progress,error_msg,retry_count,worker_id,created_at,updated_at,started_at,completed_at)
VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`,
		e.URL, e.Title, e.Options, string(e.Status), e.Progress, e.ErrorMsg, e.RetryCount, nullInt(e.WorkerID),
		toText(now), toText(now), toNullableTime(e.StartedAt), toNullableTime(e.CompletedAt),
	)
	if err != nil {
		return 0, fmt.Errorf("database: insert queue entry: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("database: queue entry last insert id: %w", err)
	}
	e.ID = id
	return id, nil
}

// ListQueueEntries returns queue rows optionally filtered by status.
func (db *DB) ListQueueEntries(ctx context.Context, status string) ([]output.QueueEntry, error) {
	base := `SELECT id,url,title,options,status,progress,error_msg,retry_count,worker_id,created_at,updated_at,started_at,completed_at FROM queue`
	args := []any{}
	if status != "" {
		base += ` WHERE status = ?`
		args = append(args, status)
	}
	base += ` ORDER BY id`
	rows, err := db.sql.QueryContext(ctx, base, args...)
	if err != nil {
		return nil, fmt.Errorf("database: list queue entries: %w", err)
	}
	defer rows.Close()
	items := make([]output.QueueEntry, 0)
	for rows.Next() {
		item, err := scanQueue(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("database: iterate queue entries: %w", err)
	}
	return items, nil
}

// GetQueueEntry fetches one queue row by id.
func (db *DB) GetQueueEntry(ctx context.Context, id int64) (*output.QueueEntry, error) {
	row := db.sql.QueryRowContext(
		ctx,
		`SELECT id,url,title,options,status,progress,error_msg,retry_count,worker_id,created_at,updated_at,started_at,completed_at FROM queue WHERE id = ?`,
		id,
	)
	item, err := scanQueue(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &item, nil
}

// UpdateQueueEntry updates mutable queue row fields.
func (db *DB) UpdateQueueEntry(ctx context.Context, e *output.QueueEntry) error {
	e.UpdatedAt = time.Now().UTC()
	_, err := db.sql.ExecContext(
		ctx,
		`UPDATE queue SET url=?,title=?,options=?,status=?,progress=?,error_msg=?,retry_count=?,worker_id=?,updated_at=?,started_at=?,completed_at=? WHERE id=?`,
		e.URL, e.Title, e.Options, string(e.Status), e.Progress, e.ErrorMsg, e.RetryCount, nullInt(e.WorkerID),
		toText(e.UpdatedAt), toNullableTime(e.StartedAt), toNullableTime(e.CompletedAt), e.ID,
	)
	if err != nil {
		return fmt.Errorf("database: update queue entry %d: %w", e.ID, err)
	}
	return nil
}

// DeleteQueueEntry removes a queue row by id.
func (db *DB) DeleteQueueEntry(ctx context.Context, id int64) error {
	if _, err := db.sql.ExecContext(ctx, `DELETE FROM queue WHERE id = ?`, id); err != nil {
		return fmt.Errorf("database: delete queue entry %d: %w", id, err)
	}
	return nil
}

type scanner interface {
	Scan(dest ...any) error
}

func scanQueue(s scanner) (output.QueueEntry, error) {
	var item output.QueueEntry
	var worker sql.NullInt64
	var createdText string
	var updatedText string
	var startedText sql.NullString
	var completedText sql.NullString
	if err := s.Scan(
		&item.ID, &item.URL, &item.Title, &item.Options, &item.Status, &item.Progress, &item.ErrorMsg, &item.RetryCount,
		&worker, &createdText, &updatedText, &startedText, &completedText,
	); err != nil {
		return output.QueueEntry{}, fmt.Errorf("database: scan queue entry: %w", err)
	}
	createdAt, err := parseTextTime(createdText)
	if err != nil {
		return output.QueueEntry{}, fmt.Errorf("database: parse queue created_at: %w", err)
	}
	updatedAt, err := parseTextTime(updatedText)
	if err != nil {
		return output.QueueEntry{}, fmt.Errorf("database: parse queue updated_at: %w", err)
	}
	item.CreatedAt = createdAt
	item.UpdatedAt = updatedAt
	if worker.Valid {
		item.WorkerID = int(worker.Int64)
	}
	item.StartedAt, err = parseNullableTime(startedText)
	if err != nil {
		return output.QueueEntry{}, fmt.Errorf("database: parse queue started_at: %w", err)
	}
	item.CompletedAt, err = parseNullableTime(completedText)
	if err != nil {
		return output.QueueEntry{}, fmt.Errorf("database: parse queue completed_at: %w", err)
	}
	return item, nil
}
