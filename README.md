<div align="center">

# cronit

**A beautiful terminal UI for managing cron jobs — with run history.**

[![Release](https://img.shields.io/github/v/release/MakarMolochaev/cronit)](https://github.com/MakarMolochaev/cronit/releases)
[![Go Version](https://img.shields.io/github/go-mod/go-version/MakarMolochaev/cronit)](go.mod)
[![License](https://img.shields.io/github/license/MakarMolochaev/cronit)](LICENSE)

</div>

---

`cronit` sits on top of your regular crontab and gives you a friendly TUI to create, edit, pause and inspect cron jobs — including the stdout/stderr, exit code and duration of every past run.

<!-- demo gif goes here: record with `vhs` (https://github.com/charmbracelet/vhs) -->

## Features

- **Interactive TUI** — job list with live details panel, all keyboard-driven
- **Run history** — every run is recorded: exit code, duration, full stdout/stderr
- **Schedule picker** — common presets plus your own saved templates
- **Schedule builder** — compose a schedule step by step, no cron syntax required
- **Pause / resume** — disable a job without deleting it or losing its history
- **Human-readable schedules** — descriptions and upcoming run times for every job
- **Named jobs** — give jobs names, or get a generated one like `brave-otter`
- **Single static binary** — pure Go, no cgo, no runtime dependencies

## Installation

### From release binaries

Grab an archive for your platform from the [releases page](https://github.com/MakarMolochaev/cronit/releases), then:

```sh
tar -xzf cronit_*.tar.gz
sudo mv cronit /usr/local/bin/
```

### With `go install`

```sh
go install github.com/MakarMolochaev/cronit@latest
```

The binary lands in `$GOBIN` (defaults to `~/go/bin` — make sure it is in your `PATH`).

### From source

```sh
git clone https://github.com/MakarMolochaev/cronit.git
cd cronit
./install.sh
```

`install.sh` builds the binary and installs it to `~/.local/bin` (override with `BINDIR=/some/path ./install.sh`).

## Usage

Launch the TUI:

```sh
cronit
```

Or use the CLI directly:

```sh
cronit add "*/5 * * * *" "curl -s https://example.com/ping"
cronit add "@daily" "backup.sh"
cronit rm <id>
cronit version
cronit help
```

Schedules accept standard five-field cron expressions as well as the `@yearly`, `@monthly`, `@weekly`, `@daily`, `@hourly` and `@reboot` shortcuts.

### Keys

| Key | Action |
| --- | --- |
| `↑`/`↓` | select job |
| `a` | add job |
| `e` | edit job |
| `space` | pause / resume job |
| `enter` | view run history & output |
| `d` | delete job |
| `q` | quit |

In the add/edit form:

| Key | Action |
| --- | --- |
| `tab` | next field |
| `ctrl+t` | schedule picker (presets & saved templates) |
| `ctrl+b` | schedule builder |
| `enter` | save |
| `esc` | cancel |

## How it works

`cronit` does not run its own daemon — your system's cron does the scheduling. Each job is written to your crontab as a marked entry:

```
# cronit:id=abc12
*/5 * * * * /usr/local/bin/cronit start abc12 "curl -s https://example.com/ping"
```

When cron fires, `cronit start` executes the command through `sh -c`, passes stdout/stderr through untouched, and records the exit code, duration and captured output.

Jobs, templates and run history live in a SQLite database at `~/.local/share/cronit/cronit.db` (respects `$XDG_DATA_HOME`). Pausing a job removes its crontab line but keeps the job and its history.

Requires Linux or macOS with a cron daemon (cronie, vixie-cron, etc.).

## License

[MIT](LICENSE)
