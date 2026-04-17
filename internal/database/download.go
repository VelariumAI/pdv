package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/velariumai/pdv/pkg/output"
)

// CreateFileEntry inserts a files row and returns the new id.
func (db *DB) CreateFileEntry(ctx context.Context, e *output.FileEntry) (int64, error) {
	if e.CreatedAt.IsZero() {
		e.CreatedAt = time.Now().UTC()
	}
	res, err := db.sql.ExecContext(
		ctx,
		`INSERT INTO files(history_id,filename,ext,size_bytes,mime_type,created_at) VALUES(?,?,?,?,?,?)`,
		e.HistoryID, e.Filename, e.Ext, e.SizeBytes, e.MimeType, toText(e.CreatedAt),
	)
	if err != nil {
		return 0, fmt.Errorf("database: insert file entry: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("database: file entry last insert id: %w", err)
	}
	e.ID = id
	return id, nil
}

// ListFileEntries returns file rows optionally filtered by history id.
func (db *DB) ListFileEntries(ctx context.Context, historyID int64) ([]output.FileEntry, error) {
	base := `SELECT id,history_id,filename,ext,size_bytes,mime_type,created_at FROM files`
	args := []any{}
	if historyID > 0 {
		base += ` WHERE history_id = ?`
		args = append(args, historyID)
	}
	base += ` ORDER BY id`
	rows, err := db.sql.QueryContext(ctx, base, args...)
	if err != nil {
		return nil, fmt.Errorf("database: list file entries: %w", err)
	}
	defer rows.Close()
	items := make([]output.FileEntry, 0)
	for rows.Next() {
		item, err := scanFile(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("database: iterate file entries: %w", err)
	}
	return items, nil
}

// GetFileEntry fetches one files row by id.
func (db *DB) GetFileEntry(ctx context.Context, id int64) (*output.FileEntry, error) {
	row := db.sql.QueryRowContext(
		ctx,
		`SELECT id,history_id,filename,ext,size_bytes,mime_type,created_at FROM files WHERE id = ?`,
		id,
	)
	item, err := scanFile(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &item, nil
}

// UpdateFileEntry updates mutable file row fields.
func (db *DB) UpdateFileEntry(ctx context.Context, e *output.FileEntry) error {
	_, err := db.sql.ExecContext(
		ctx,
		`UPDATE files SET history_id=?,filename=?,ext=?,size_bytes=?,mime_type=?,created_at=? WHERE id=?`,
		e.HistoryID, e.Filename, e.Ext, e.SizeBytes, e.MimeType, toText(e.CreatedAt), e.ID,
	)
	if err != nil {
		return fmt.Errorf("database: update file entry %d: %w", e.ID, err)
	}
	return nil
}

// DeleteFileEntry removes a files row by id.
func (db *DB) DeleteFileEntry(ctx context.Context, id int64) error {
	if _, err := db.sql.ExecContext(ctx, `DELETE FROM files WHERE id = ?`, id); err != nil {
		return fmt.Errorf("database: delete file entry %d: %w", id, err)
	}
	return nil
}

func scanFile(s scanner) (output.FileEntry, error) {
	var item output.FileEntry
	var createdText string
	if err := s.Scan(
		&item.ID, &item.HistoryID, &item.Filename, &item.Ext, &item.SizeBytes, &item.MimeType, &createdText,
	); err != nil {
		return output.FileEntry{}, fmt.Errorf("database: scan file entry: %w", err)
	}
	createdAt, err := parseTextTime(createdText)
	if err != nil {
		return output.FileEntry{}, fmt.Errorf("database: parse file created_at: %w", err)
	}
	item.CreatedAt = createdAt
	return item, nil
}
