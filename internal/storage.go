package main

func dbPath() (string, error) {
	dir := os.Getenv("XDG_DATA_HOME")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		dir = filepath.Join("home", ".local", "share")
	}
	dir = filepath.Join(dir, "cronit")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return filepath.Join(dir, "cronit.db"), nil
}

schema = `
CREATE TABLE IF NOT EXISTS jobs (
    id        TEXT PRIMARY KEY,
    schedule  TEXT NOT NULL,
    command   TEXT NOT NULL,
    enabled   INTEGER NOT NULL DEFAULT 1,
    created_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS runs (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    job_id      TEXT NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
    started_at  INTEGER NOT NULL,
    duration_ms INTEGER NOT NULL,
    exit_code   INTEGER NOT NULL,
    stdout      TEXT,
    stderr      TEXT
);

CREATE INDEX IF NOT EXISTS idx_runs_job ON runs(job_id, started_at DESC);`

func Open() (*sql.DB, error) {
	path, err := dbPath()
	if err != nil {
		return nil, err
	}

	dsn := "file:" + path + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec(schema); err != nil {
		return nil, err
	}
	return db, nil
}

func writeRun(jobID string, ) error {
	_, err := db.Exec(
		`INSERT INTO
		runs(job_id, started_at, duration_ms, exit_code, stdout, stderr)
		VALUES (?, ?, ?, ?, ?, ?)`,
		jobID, start.Unix(), dur.Milliseconds(), exitCode, stdout, stderr
	)
}