package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/velariumai/pdv/pkg/output"
)

// CreateHistoryEntry inserts a history row and returns the new id.
func (db *DB) CreateHistoryEntry(ctx context.Context, e *output.HistoryEntry) (int64, error) {
	if e.DownloadedAt.IsZero() {
		e.DownloadedAt = time.Now().UTC()
	}
	res, err := db.sql.ExecContext(
		ctx,
		`INSERT INTO history(url,title,final_status,file_path,file_size,category,error_msg,downloaded_at,duration_secs)
VALUES(?,?,?,?,?,?,?,?,?)`,
		e.URL, e.Title, e.FinalStatus, e.FilePath, e.FileSize, e.Category, e.ErrorMsg, toText(e.DownloadedAt), e.DurationSecs,
	)
	if err != nil {
		return 0, fmt.Errorf("database: insert history entry: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("database: history entry last insert id: %w", err)
	}
	e.ID = id
	return id, nil
}

// ListHistoryEntries returns history rows optionally filtered by final status.
func (db *DB) ListHistoryEntries(ctx context.Context, finalStatus string) ([]output.HistoryEntry, error) {
	base := `SELECT id,url,title,final_status,file_path,file_size,category,error_msg,downloaded_at,duration_secs FROM history`
	args := []any{}
	if finalStatus != "" {
		base += ` WHERE final_status = ?`
		args = append(args, finalStatus)
	}
	base += ` ORDER BY id DESC`
	rows, err := db.sql.QueryContext(ctx, base, args...)
	if err != nil {
		return nil, fmt.Errorf("database: list history entries: %w", err)
	}
	defer rows.Close()
	items := make([]output.HistoryEntry, 0)
	for rows.Next() {
		item, err := scanHistory(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("database: iterate history entries: %w", err)
	}
	return items, nil
}

// GetHistoryEntry fetches one history row by id.
func (db *DB) GetHistoryEntry(ctx context.Context, id int64) (*output.HistoryEntry, error) {
	row := db.sql.QueryRowContext(
		ctx,
		`SELECT id,url,title,final_status,file_path,file_size,category,error_msg,downloaded_at,duration_secs FROM history WHERE id = ?`,
		id,
	)
	item, err := scanHistory(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &item, nil
}

// UpdateHistoryEntry updates mutable history row fields.
func (db *DB) UpdateHistoryEntry(ctx context.Context, e *output.HistoryEntry) error {
	_, err := db.sql.ExecContext(
		ctx,
		`UPDATE history SET url=?,title=?,final_status=?,file_path=?,file_size=?,category=?,error_msg=?,downloaded_at=?,duration_secs=? WHERE id=?`,
		e.URL, e.Title, e.FinalStatus, e.FilePath, e.FileSize, e.Category, e.ErrorMsg, toText(e.DownloadedAt), e.DurationSecs, e.ID,
	)
	if err != nil {
		return fmt.Errorf("database: update history entry %d: %w", e.ID, err)
	}
	return nil
}

// DeleteHistoryEntry removes a history row by id.
func (db *DB) DeleteHistoryEntry(ctx context.Context, id int64) error {
	if _, err := db.sql.ExecContext(ctx, `DELETE FROM history WHERE id = ?`, id); err != nil {
		return fmt.Errorf("database: delete history entry %d: %w", id, err)
	}
	return nil
}

func scanHistory(s scanner) (output.HistoryEntry, error) {
	var item output.HistoryEntry
	var downloadedText string
	if err := s.Scan(
		&item.ID, &item.URL, &item.Title, &item.FinalStatus, &item.FilePath, &item.FileSize, &item.Category,
		&item.ErrorMsg, &downloadedText, &item.DurationSecs,
	); err != nil {
		return output.HistoryEntry{}, fmt.Errorf("database: scan history entry: %w", err)
	}
	downloadedAt, err := parseTextTime(downloadedText)
	if err != nil {
		return output.HistoryEntry{}, fmt.Errorf("database: parse history downloaded_at: %w", err)
	}
	item.DownloadedAt = downloadedAt
	return item, nil
}
