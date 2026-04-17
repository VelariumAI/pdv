package database

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// DB wraps the SQLite connection and migration runner.
type DB struct {
	sql *sql.DB
}

// Open creates a DB connection, verifies connectivity, and applies migrations.
func Open(ctx context.Context, path string) (*DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("database: create dir for %q: %w", path, err)
	}
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("database: open sqlite %q: %w", path, err)
	}
	conn.SetMaxOpenConns(1)
	conn.SetConnMaxLifetime(0)
	if err := conn.PingContext(ctx); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("database: ping %q: %w", path, err)
	}
	db := &DB{sql: conn}
	if err := db.Migrate(ctx); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("database: migrate %q: %w", path, err)
	}
	return db, nil
}

// Close releases the underlying database connection.
func (db *DB) Close() error {
	if db == nil || db.sql == nil {
		return nil
	}
	if err := db.sql.Close(); err != nil {
		return fmt.Errorf("database: close: %w", err)
	}
	return nil
}

// Migrate ensures all schema migrations are applied in order.
func (db *DB) Migrate(ctx context.Context) error {
	if err := db.ensureMigrationsTable(ctx); err != nil {
		return err
	}
	for _, m := range migrations() {
		applied, err := db.isApplied(ctx, m.id)
		if err != nil {
			return err
		}
		if applied {
			continue
		}
		if _, err := db.sql.ExecContext(ctx, m.sql); err != nil {
			return fmt.Errorf("database: apply migration %d (%s): %w", m.id, m.name, err)
		}
		if err := db.markApplied(ctx, m.id); err != nil {
			return err
		}
	}
	return nil
}

func (db *DB) ensureMigrationsTable(ctx context.Context) error {
	const stmt = `
CREATE TABLE IF NOT EXISTS schema_migrations (
    id INTEGER PRIMARY KEY,
    applied_at TEXT NOT NULL
);`
	if _, err := db.sql.ExecContext(ctx, stmt); err != nil {
		return fmt.Errorf("database: ensure schema_migrations: %w", err)
	}
	return nil
}

func (db *DB) isApplied(ctx context.Context, id int) (bool, error) {
	var count int
	if err := db.sql.QueryRowContext(ctx, "SELECT COUNT(1) FROM schema_migrations WHERE id = ?", id).Scan(&count); err != nil {
		return false, fmt.Errorf("database: query migration %d: %w", id, err)
	}
	return count > 0, nil
}

func (db *DB) markApplied(ctx context.Context, id int) error {
	appliedAt := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := db.sql.ExecContext(ctx, "INSERT INTO schema_migrations(id, applied_at) VALUES(?, ?)", id, appliedAt); err != nil {
		return fmt.Errorf("database: insert migration %d: %w", id, err)
	}
	return nil
}
