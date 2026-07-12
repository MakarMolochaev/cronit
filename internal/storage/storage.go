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
	Name     string
	Schedule string
	Command  string
	Enabled  bool
}

type Run struct {
	StartedAt  time.Time
	DurationMs int64
	ExitCode   int
	Stdout     string
	Stderr     string
}

type Template struct {
	Name     string
	Schedule string
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
    name      TEXT NOT NULL DEFAULT '',
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

CREATE INDEX IF NOT EXISTS idx_runs_job ON runs(job_id, started_at DESC);

CREATE TABLE IF NOT EXISTS templates (
    name       TEXT PRIMARY KEY,
    schedule   TEXT NOT NULL,
    created_at INTEGER NOT NULL
);`

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
	if err := d.ensureColumn("jobs", "name", "name TEXT NOT NULL DEFAULT ''"); err != nil {
		d.db.Close()
		return nil, err
	}
	return d, nil
}

func (d *Database) ensureColumn(table, col, ddl string) error {
	rows, err := d.db.Query("PRAGMA table_info(" + table + ")")
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var cid, notnull, pk int
		var name, ctype string
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return err
		}
		if name == col {
			return nil
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	_, err = d.db.Exec("ALTER TABLE " + table + " ADD COLUMN " + ddl)
	return err
}

func (d *Database) Close() error {
	return d.db.Close()
}

func (d *Database) SaveJob(id, name, schedule, command string) error {
	_, err := d.db.Exec(
		`INSERT INTO jobs (id, name, schedule, command, enabled, created_at)
		 VALUES (?, ?, ?, ?, 1, ?)
		 ON CONFLICT(id) DO UPDATE SET
		     name     = excluded.name,
		     schedule = excluded.schedule,
		     command  = excluded.command`,
		id, name, schedule, command, time.Now().Unix(),
	)
	return err
}

func (d *Database) Jobs() ([]Job, error) {
	rows, err := d.db.Query(`SELECT id, name, schedule, command, enabled FROM jobs ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []Job
	for rows.Next() {
		var j Job
		var enabled int
		if err := rows.Scan(&j.ID, &j.Name, &j.Schedule, &j.Command, &enabled); err != nil {
			return nil, err
		}
		j.Enabled = enabled != 0
		jobs = append(jobs, j)
	}
	return jobs, rows.Err()
}

func (d *Database) Job(id string) (Job, error) {
	var j Job
	var enabled int
	err := d.db.QueryRow(`SELECT id, name, schedule, command, enabled FROM jobs WHERE id = ?`, id).
		Scan(&j.ID, &j.Name, &j.Schedule, &j.Command, &enabled)
	j.Enabled = enabled != 0
	return j, err
}

func (d *Database) SetEnabled(id string, enabled bool) error {
	v := 0
	if enabled {
		v = 1
	}
	_, err := d.db.Exec(`UPDATE jobs SET enabled = ? WHERE id = ?`, v, id)
	return err
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

func (d *Database) SaveTemplate(name, schedule string) error {
	_, err := d.db.Exec(
		`INSERT INTO templates (name, schedule, created_at)
		 VALUES (?, ?, ?)
		 ON CONFLICT(name) DO UPDATE SET schedule = excluded.schedule`,
		name, schedule, time.Now().Unix(),
	)
	return err
}

func (d *Database) Templates() ([]Template, error) {
	rows, err := d.db.Query(`SELECT name, schedule FROM templates ORDER BY created_at DESC, rowid DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var templates []Template
	for rows.Next() {
		var t Template
		if err := rows.Scan(&t.Name, &t.Schedule); err != nil {
			return nil, err
		}
		templates = append(templates, t)
	}
	return templates, rows.Err()
}

func (d *Database) DeleteTemplate(name string) error {
	_, err := d.db.Exec(`DELETE FROM templates WHERE name = ?`, name)
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
