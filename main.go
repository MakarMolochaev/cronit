package main

import (
	"bytes"
	"crypto/rand"
	"encoding/base32"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/MakarMolochaev/cronit/internal/crontab"
	"github.com/MakarMolochaev/cronit/internal/storage"
)

func main() {
	arguments := os.Args[1:]

	db, err := storage.Open()
	if err != nil {
		panic("Failed to open database: " + err.Error())
	}
	defer db.Close()

	switch arguments[0] {
	case "add":
		schedule := arguments[1]
		command := arguments[2]
		id := newID()
		if err := db.SaveJob(id, schedule, command); err != nil {
			panic("SaveJob: " + err.Error())
		}
		self, err := os.Executable()
		if err != nil {
			panic("os.Executable: " + err.Error())
		}
		line := fmt.Sprintf("%s start %s %q", self, id, command)
		if err := crontab.AddJob(id, schedule, line); err != nil {
			panic("crontab.AddJob: " + err.Error())
		}
		fmt.Printf("added job %s\n", id)

	case "start":
		id := arguments[1]
		command := arguments[2]

		cmd := exec.Command("sh", "-c", command)
		var outBuf, errBuf bytes.Buffer
		cmd.Stdout = io.MultiWriter(os.Stdout, &outBuf)
		cmd.Stderr = io.MultiWriter(os.Stderr, &errBuf)

		startTime := time.Now()
		runErr := cmd.Run()
		workTime := time.Since(startTime)

		exitCode := 0
		if runErr != nil {
			var ee *exec.ExitError
			if errors.As(runErr, &ee) {
				exitCode = ee.ExitCode()
			} else {
				exitCode = -1
				errBuf.WriteString(runErr.Error())
			}
		}

		Stdout := outBuf.String()
		Stderr := errBuf.String()
		if err := db.SaveRun(id, startTime, workTime, exitCode, Stdout, Stderr); err != nil {
			panic("SaveRun: " + err.Error())
		}
	case "rm":

		id := arguments[1]
		if err := crontab.RemoveJob(id); err != nil {
			panic("crontab.RemoveJob: " + err.Error())
		}
		if err := db.DeleteJob(id); err != nil {
			panic("DeleteJob: " + err.Error())
		}
		fmt.Printf("removed job %s\n", id)
	}
}

func newID() string {
	b := make([]byte, 5)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	enc := base32.StdEncoding.WithPadding(base32.NoPadding)
	return strings.ToLower(enc.EncodeToString(b))
}
