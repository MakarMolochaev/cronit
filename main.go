package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime/debug"
	"time"

	"github.com/MakarMolochaev/cronit/internal/banner"
	"github.com/MakarMolochaev/cronit/internal/crontab"
	"github.com/MakarMolochaev/cronit/internal/manager"
	"github.com/MakarMolochaev/cronit/internal/storage"
	"github.com/MakarMolochaev/cronit/internal/tui"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func resolveVersion() string {
	if version != "dev" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return version
}

func main() {
	version = resolveVersion()
	arguments := os.Args[1:]

	db, err := storage.Open()
	if err != nil {
		panic("Failed to open database: " + err.Error())
	}
	defer db.Close()

	if len(arguments) == 0 {
		if err := tui.Run(db, version); err != nil {
			panic("tui: " + err.Error())
		}
		return
	}

	switch arguments[0] {
	case "add":
		id, err := manager.Add(db, "", arguments[1], arguments[2])
		if err != nil {
			panic("add: " + err.Error())
		}
		fmt.Printf("added job %s\n", id)
		if running, ok := crontab.DaemonRunning(); ok && !running {
			fmt.Println("⚠ cron daemon is not running — jobs won't fire")
			fmt.Println("  start it, e.g.: sudo systemctl enable --now cronie")
		}

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

		if err := db.SaveRun(id, startTime, workTime, exitCode, outBuf.String(), errBuf.String()); err != nil {
			panic("SaveRun: " + err.Error())
		}

	case "rm":
		if err := manager.Remove(db, arguments[1]); err != nil {
			panic("rm: " + err.Error())
		}
		fmt.Printf("removed job %s\n", arguments[1])
	case "version":
		fmt.Printf("Cronit version: %s\n", version)
	case "--version":
		fmt.Printf("Cronit version: %s\n", version)
	case "-v":
		fmt.Printf("Cronit version: %s\n", version)
	case "help", "-h", "--help":
		printHelp()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", arguments[0])
		printHelp()
		os.Exit(2)
	}
}

func printHelp() {
	fmt.Println(banner.Render())
	fmt.Print(`
cronit — a TUI for managing cron jobs with run history

Usage:
  cronit                      launch the interactive TUI
  cronit add <sched> <cmd>    add a job
  cronit rm <id>              remove a job
  cronit version              print version
  cronit help                 show this help
`)
}
