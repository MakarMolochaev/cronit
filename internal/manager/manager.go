package manager

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"crypto/rand"
	"encoding/base32"
	mrand "math/rand/v2"
	"strings"

	"github.com/MakarMolochaev/cronit/internal/crontab"
	"github.com/MakarMolochaev/cronit/internal/schedule"
	"github.com/MakarMolochaev/cronit/internal/storage"
)

func Add(db *storage.Database, name, sched, command string) (string, error) {
	if err := validate(sched); err != nil {
		return "", err
	}
	if strings.TrimSpace(name) == "" {
		name = RandomName()
	}
	id := newID()
	if err := db.SaveJob(id, name, sched, command); err != nil {
		return "", err
	}
	if err := syncCrontab(id, sched, command, true); err != nil {
		return "", err
	}
	return id, nil
}

func Update(db *storage.Database, id, name, sched, command string) error {
	if err := validate(sched); err != nil {
		return err
	}
	if strings.TrimSpace(name) == "" {
		name = RandomName()
	}
	cur, err := db.Job(id)
	if err != nil {
		return err
	}
	if err := db.SaveJob(id, name, sched, command); err != nil {
		return err
	}
	return syncCrontab(id, sched, command, cur.Enabled)
}

func SetEnabled(db *storage.Database, id string, enabled bool) error {
	job, err := db.Job(id)
	if err != nil {
		return err
	}
	if err := db.SetEnabled(id, enabled); err != nil {
		return err
	}
	return syncCrontab(id, job.Schedule, job.Command, enabled)
}

func Remove(db *storage.Database, id string) error {
	if err := crontab.RemoveJob(id); err != nil {
		return err
	}
	return db.DeleteJob(id)
}

type ImportResult struct {
	Imported   int
	BackupPath string
}

func Import(db *storage.Database, entries []crontab.ForeignEntry) (ImportResult, error) {
	var res ImportResult
	if len(entries) == 0 {
		return res, errors.New("nothing to import")
	}

	current, err := crontab.Read()
	if err != nil {
		return res, err
	}
	lines := strings.Split(current, "\n")

	remove := map[int]bool{}
	for _, e := range entries {
		if e.Err != "" {
			return res, fmt.Errorf("cannot import %q: %s", strings.TrimSpace(e.Line), e.Err)
		}
		if err := validate(e.Schedule); err != nil {
			return res, err
		}
		if e.LineNo < 0 || e.LineNo >= len(lines) || lines[e.LineNo] != e.Line {
			return res, errors.New("crontab changed since it was read, reopen import")
		}
		remove[e.LineNo] = true
		if e.CommentLine >= 0 {
			remove[e.CommentLine] = true
		}
	}

	backup, err := backupCrontab(current)
	if err != nil {
		return res, err
	}
	res.BackupPath = backup

	self, err := os.Executable()
	if err != nil {
		return res, err
	}

	kept := make([]string, 0, len(lines))
	for i, ln := range lines {
		if !remove[i] {
			kept = append(kept, ln)
		}
	}
	for len(kept) > 0 && strings.TrimSpace(kept[len(kept)-1]) == "" {
		kept = kept[:len(kept)-1]
	}

	var b strings.Builder
	for _, ln := range kept {
		b.WriteString(ln)
		b.WriteByte('\n')
	}
	for _, e := range entries {
		name := strings.TrimSpace(e.Name)
		if name == "" {
			name = RandomName()
		}
		id := newID()
		if err := db.SaveJob(id, name, e.Schedule, e.Command); err != nil {
			return res, err
		}
		line := fmt.Sprintf("%s start %s %q", self, id, e.Command)
		b.WriteString(crontab.FormatJob(id, e.Schedule, escapePercent(line)))
		res.Imported++
	}

	if err := crontab.Write(b.String()); err != nil {
		return res, err
	}
	return res, nil
}

func backupCrontab(content string) (string, error) {
	dir, err := storage.DataDir()
	if err != nil {
		return "", err
	}
	path := filepath.Join(dir, "crontab-"+time.Now().Format("20060102-150405")+".bak")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		return "", err
	}
	return path, nil
}

func validate(sched string) error {
	if _, err := schedule.Parse(sched); err != nil {
		return fmt.Errorf("invalid schedule: %w", err)
	}
	return nil
}

func syncCrontab(id, sched, command string, enabled bool) error {
	if err := crontab.RemoveJob(id); err != nil {
		return err
	}
	if !enabled {
		return nil
	}
	self, err := os.Executable()
	if err != nil {
		return err
	}
	line := fmt.Sprintf("%s start %s %q", self, id, command)
	return crontab.AddJob(id, sched, escapePercent(line))
}

func escapePercent(s string) string {
	return strings.ReplaceAll(s, "%", `\%`)
}

func newID() string {
	b := make([]byte, 5)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	enc := base32.StdEncoding.WithPadding(base32.NoPadding)
	return strings.ToLower(enc.EncodeToString(b))
}

var nameAdjectives = []string{
	"amber", "bold", "brave", "bright", "calm", "clever", "cosmic", "crimson",
	"curious", "dapper", "eager", "fuzzy", "gentle", "distributed", "golden", "happy", "jolly",
	"lively", "lucky", "mellow", "merry", "misty", "wish", "nimble", "quiet", "rapid",
	"shiny", "silent", "silver", "sly", "snappy", "spry", "sunny", "swift",
	"tidy", "vivid", "witty", "zesty",
}

var nameNouns = []string{
	"otter", "falcon", "panda", "lynx", "heron", "koala", "gecko", "raven",
	"badger", "marten", "ferret", "beaver", "walrus", "puffin", "meerkat", "narwhal",
	"lemur", "bison", "mantis", "cricket", "sparrow", "magpie", "wombat", "civet",
	"tapir", "ocelot", "quokka", "fedos", "ibex", "manatee", "pelican", "salmon", "hedgehog",
	"comet", "cypress", "willow", "cobalt",
}

func RandomName() string {
	return nameAdjectives[mrand.IntN(len(nameAdjectives))] + "-" + nameNouns[mrand.IntN(len(nameNouns))]
}
