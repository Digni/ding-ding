# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Run Commands

| Task | Command |
|------|---------|
| Build | `go build -o ding-ding .` |
| Build with version | `go build -ldflags="-X main.Version=vX.Y.Z" -o ding-ding .` |
| Vet/lint | `go vet ./...` |
| Run tests | `go test ./...` |
| Run single test | `go test ./internal/notifier -run TestName` |
| Install | `go install github.com/Digni/ding-ding@latest` |

## Architecture

**ding-ding** is a Go CLI tool + optional HTTP server that sends context-aware notifications when AI agent tasks complete. It uses a 3-tier notification system based on user attention state.

### 3-Tier Notification Flow

```
Agent fires ding-ding
       │
       ▼
 ┌─ Check idle duration + terminal focus ─┐
 │                                         │
 ▼                  ▼                      ▼
Focused &        Unfocused &            Idle (≥ threshold)
 Active           Active
 │                  │                      │
silent         system notify         system notify
                                    + push (ntfy/Discord/webhook)
```

### Key Packages

- **`cmd/`** — Cobra CLI commands (`notify`, `serve`, `config`). Subcommands register via `init()` functions. `Version` is set from `main.go`.
- **`internal/notifier/`** — Core notification logic. Both CLI (`Notify()`) and HTTP (`NotifyRemote()`) share the same 3-tier logic. `NotifyRemote` accepts a `pid` so the server can detect focus for the remote agent's process tree.
- **`internal/focus/`** — Terminal focus detection with platform-specific implementations via build tags (`_darwin.go`, `_linux.go`, `_windows.go`). Climbs the process tree to find the terminal emulator.
- **`internal/idle/`** — User idle time detection with platform-specific implementations via build tags. Uses ioreg (macOS), xprintidle/dbus (Linux), GetLastInputInfo (Windows).
- **`internal/config/`** — YAML config at `~/.config/ding-ding/config.yaml`. `DefaultConfig()` provides defaults; `Load()` overlays user config on top.

### Platform Abstraction Pattern

The `focus` and `idle` packages use Go build tags (`//go:build darwin|linux|windows`) with unexported platform-specific functions that exported wrappers call. No interfaces needed.

### HTTP Server

Uses Go 1.22+ pattern-matching (`"POST /notify"`, `"GET /notify"`, `"GET /health"`). Clients pass `"pid": $$` so the server can do per-request focus detection for the correct terminal.

### Dependencies

Only two direct runtime dependencies: `github.com/spf13/cobra` (CLI) and `gopkg.in/yaml.v3` (config). All system interactions use `os/exec` or direct syscalls.

## Commit Convention

Use [Conventional Commits](https://www.conventionalcommits.org/). Format:

```
<type>(<scope>): <description>

[optional body]

[optional footer(s)]
```

Types: `feat`, `fix`, `docs`, `style`, `refactor`, `perf`, `test`, `build`, `ci`, `chore`.
Scope examples: `notifier`, `focus`, `idle`, `config`, `cmd`, `server`.

Do NOT add `Co-Authored-By` trailers to commits.
