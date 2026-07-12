package crontab

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

const marker = "# cronit:id="

func DaemonRunning() (running bool, ok bool) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return false, false
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if _, err := strconv.Atoi(e.Name()); err != nil {
			continue
		}
		data, err := os.ReadFile("/proc/" + e.Name() + "/comm")
		if err != nil {
			continue
		}
		switch strings.TrimSpace(string(data)) {
		case "cron", "crond", "cronie", "fcron":
			return true, true
		}
	}
	return false, true
}

func Read() (string, error) {
	cmd := exec.Command("crontab", "-l")
	var out, errBuf bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		if strings.Contains(errBuf.String(), "no crontab") {
			return "", nil
		}
		var ee *exec.ExitError
		if errors.As(err, &ee) && out.Len() == 0 {
			return "", nil
		}
		return "", fmt.Errorf("crontab -l: %w: %s", err, strings.TrimSpace(errBuf.String()))
	}
	return out.String(), nil
}

func Write(content string) error {
	cmd := exec.Command("crontab", "-")
	cmd.Stdin = strings.NewReader(content)
	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("crontab -: %w: %s", err, strings.TrimSpace(errBuf.String()))
	}
	return nil
}

func AddJob(id, schedule, line string) error {
	current, err := Read()
	if err != nil {
		return err
	}

	var b strings.Builder
	b.WriteString(current)
	if current != "" && !strings.HasSuffix(current, "\n") {
		b.WriteByte('\n')
	}
	b.WriteString(marker)
	b.WriteString(id)
	b.WriteString("\n")
	b.WriteString(schedule)
	b.WriteString(" ")
	b.WriteString(line)
	b.WriteString("\n")

	return Write(b.String())
}

func RemoveJob(id string) error {
	current, err := Read()
	if err != nil {
		return err
	}

	target := marker + id
	lines := strings.Split(current, "\n")
	out := make([]string, 0, len(lines))
	for i := 0; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == target {
			i++
			continue
		}
		out = append(out, lines[i])
	}

	return Write(strings.Join(out, "\n"))
}
