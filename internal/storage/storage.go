package storage

import (
	"database/sql"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

type Database struct {
	db *sql.DB
}

type Job struct {
	ID       string
	Schedule string
	Command  string
}

type Run struct {
	StartedAt  time.Time
	DurationMs int64
	ExitCode   int
	Stdout     string
	Stderr     string
}

func dbPath() (string, error) {
	dir := os.Getenv("XDG_DATA_HOME")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		dir = filepath.Join(home, ".local", "share")
	}
	dir = filepath.Join(dir, "cronit")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return filepath.Join(dir, "cronit.db"), nil
}

const schema = `
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

func Open() (*Database, error) {
	d := &Database{}
	path, err := dbPath()
	if err != nil {
		return nil, err
	}

	dsn := "file:" + path + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)"
	d.db, err = sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}

	d.db.SetMaxOpenConns(1)

	if err := d.db.Ping(); err != nil {
		d.db.Close()
		return nil, err
	}
	if _, err := d.db.Exec(schema); err != nil {
		d.db.Close()
		return nil, err
	}
	return d, nil
}

func (d *Database) Close() error {
	return d.db.Close()
}

func (d *Database) SaveJob(id, schedule, command string) error {
	_, err := d.db.Exec(
		`INSERT INTO jobs (id, schedule, command, enabled, created_at)
		 VALUES (?, ?, ?, 1, ?)
		 ON CONFLICT(id) DO UPDATE SET
		     schedule = excluded.schedule,
		     command  = excluded.command`,
		id, schedule, command, time.Now().Unix(),
	)
	return err
}

func (d *Database) Jobs() ([]Job, error) {
	rows, err := d.db.Query(`SELECT id, schedule, command FROM jobs ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []Job
	for rows.Next() {
		var j Job
		if err := rows.Scan(&j.ID, &j.Schedule, &j.Command); err != nil {
			return nil, err
		}
		jobs = append(jobs, j)
	}
	return jobs, rows.Err()
}

func (d *Database) DeleteJob(id string) error {
	_, err := d.db.Exec(`DELETE FROM jobs WHERE id = ?`, id)
	return err
}

func (d *Database) SaveRun(jobID string, start time.Time, dur time.Duration, exitCode int, stdout, stderr string) error {
	_, err := d.db.Exec(
		`INSERT INTO
		runs(job_id, started_at, duration_ms, exit_code, stdout, stderr)
		VALUES (?, ?, ?, ?, ?, ?)`,
		jobID, start.Unix(), dur.Milliseconds(), exitCode, stdout, stderr,
	)
	return err
}

func (d *Database) RecentRuns(jobID string, limit int) ([]Run, error) {
	rows, err := d.db.Query(
		`SELECT started_at, duration_ms, exit_code, stdout, stderr
		 FROM runs WHERE job_id = ? ORDER BY started_at DESC LIMIT ?`,
		jobID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var runs []Run
	for rows.Next() {
		var r Run
		var startedUnix int64
		if err := rows.Scan(&startedUnix, &r.DurationMs, &r.ExitCode, &r.Stdout, &r.Stderr); err != nil {
			return nil, err
		}
		r.StartedAt = time.Unix(startedUnix, 0)
		runs = append(runs, r)
	}
	return runs, rows.Err()
}
