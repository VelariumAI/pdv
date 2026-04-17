package database

type migration struct {
	id   int
	name string
	sql  string
}

func migrations() []migration {
	return []migration{
		{
			id:   1,
			name: "initial_schema",
			sql: `
CREATE TABLE IF NOT EXISTS queue (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    url TEXT NOT NULL,
    title TEXT,
    options TEXT,
    status TEXT NOT NULL DEFAULT 'pending',
    progress REAL NOT NULL DEFAULT 0,
    error_msg TEXT,
    retry_count INTEGER NOT NULL DEFAULT 0,
    worker_id INTEGER,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    started_at TEXT,
    completed_at TEXT
);
CREATE TABLE IF NOT EXISTS history (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    url TEXT NOT NULL,
    title TEXT,
    final_status TEXT NOT NULL,
    file_path TEXT,
    file_size INTEGER,
    category TEXT,
    error_msg TEXT,
    downloaded_at TEXT NOT NULL,
    duration_secs INTEGER
);
CREATE TABLE IF NOT EXISTS files (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    history_id INTEGER REFERENCES history(id),
    filename TEXT NOT NULL,
    ext TEXT,
    size_bytes INTEGER,
    mime_type TEXT,
    created_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS config (
    key TEXT PRIMARY KEY,
    value TEXT
);`,
		},
	}
}
