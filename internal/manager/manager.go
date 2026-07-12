package manager

import (
	"fmt"
	"os"

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
	return crontab.AddJob(id, sched, line)
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
