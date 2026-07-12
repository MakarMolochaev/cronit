package manager

import (
	"fmt"
	"os"

	"crypto/rand"
	"encoding/base32"
	"strings"

	"github.com/MakarMolochaev/cronit/internal/crontab"
	"github.com/MakarMolochaev/cronit/internal/schedule"
	"github.com/MakarMolochaev/cronit/internal/storage"
)

func Add(db *storage.Database, sched, command string) (string, error) {
	if err := validate(sched); err != nil {
		return "", err
	}
	id := newID()
	if err := write(db, id, sched, command); err != nil {
		return "", err
	}
	return id, nil
}

func Update(db *storage.Database, id, sched, command string) error {
	if err := validate(sched); err != nil {
		return err
	}
	return write(db, id, sched, command)
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

func write(db *storage.Database, id, sched, command string) error {
	if err := db.SaveJob(id, sched, command); err != nil {
		return err
	}
	self, err := os.Executable()
	if err != nil {
		return err
	}
	line := fmt.Sprintf("%s start %s %q", self, id, command)
	if err := crontab.RemoveJob(id); err != nil {
		return err
	}
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
