# ding-ding

Notification tool for AI agent completion events. Get notified when Claude, opencode, or any other agent finishes a task.

**How it works:**
1. Agent finishes → triggers ding-ding (CLI or HTTP)
2. Checks if you're watching the agent terminal (window focus detection)
3. Smart 3-tier notification based on your attention state:
   - **Focused on agent terminal** → nothing (you already see the output)
   - **Active but on a different window** → system notification
   - **Idle (away from computer)** → system notification + push via ntfy/Discord/webhook

## Install

```bash
go install github.com/Digni/ding-ding@latest
```

Or build from source:

```bash
git clone https://github.com/Digni/ding-ding.git
cd ding-ding
go build -o ding-ding .
```

## Usage

### CLI

```bash
# Simple notification
ding-ding notify -m "Build succeeded"

# With agent name and title
ding-ding notify -a claude -t "Claude finished" -m "Refactored auth module"

# Positional args work too
ding-ding notify Task completed successfully

# Pipe output
echo "tests passed" | ding-ding notify

# Force push (always send remote push, even if focused/active)
ding-ding notify -p -m "Deploy complete"

# Force local/system notification even when focused suppression would mute it
ding-ding notify --test-local -m "Testing local notification"

# Force both local/system and remote push
ding-ding notify -p --test-local -m "Test all channels"
```

`--push` only affects remote push backends (ntfy/Discord/webhook). It does not
implicitly force a local/system notification; use `--test-local` for that.

### Agent Setup

```bash
# Initialize (or upsert) Claude project hooks using CLI mode
ding-ding agent init claude project

# Initialize (or upsert) OpenCode global plugin using CLI mode
ding-ding agent init opencode global

# Update Claude global hooks to server mode templates
ding-ding agent update claude global --mode server

# Update OpenCode project plugin in server mode
ding-ding agent update opencode project --mode server
```

Command shape:

```bash
ding-ding agent init <agent> <scope> [--mode cli|server]
ding-ding agent update <agent> <scope> [--mode cli|server]
```

- Supported agents: `claude`, `opencode`
- Supported scopes: `project`, `global`
- Default mode: `cli`

### HTTP Server

```bash
# Start the server
ding-ding serve

# In another terminal or from your agent:
curl -X POST localhost:8228/notify \
  -H "Content-Type: application/json" \
  -d '{"title": "ding ding!", "body": "Task finished", "agent": "claude"}'

# Include PID for focus detection (server checks if that terminal is focused)
curl -X POST localhost:8228/notify \
  -d '{"body": "Done", "agent": "claude", "pid": '$$'}'

# Quick GET request
curl "localhost:8228/notify?message=done&agent=claude"
```

### Agent Integration

#### Claude Code

Use the setup command (recommended):

```bash
ding-ding agent init claude project
# or
ding-ding agent init claude global
```

Manual setup if needed. Add to `.claude/settings.json` (project) or `~/.claude/settings.json` (global):

**Option A: CLI with async hook (simplest)**
```json
{
  "hooks": {
    "Stop": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "ding-ding notify -a claude -m 'Task finished'",
            "async": true
          }
        ]
      }
    ],
    "Notification": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "ding-ding notify -a claude -m 'Needs your attention'",
            "async": true
          }
        ]
      }
    ]
  }
}
```

`Stop` fires when Claude finishes a response. `Notification` fires when Claude is blocked
waiting for you (permission prompt, idle, etc). Together they cover both "done" and
"waiting for you" — the two times you actually want to be pinged.

`"async": true` is important — without it, the hook blocks Claude until ding-ding finishes
sending any remote notifications (ntfy, Discord, etc). With async, Claude returns immediately.

**Option B: Server mode (better for multiple agents)**
```json
{
  "hooks": {
    "Stop": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "curl -s localhost:8228/notify -d '{\"agent\":\"claude\",\"body\":\"Task finished\",\"pid\":'$$'}'",
            "async": true
          }
        ]
      }
    ],
    "Notification": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "curl -s localhost:8228/notify -d '{\"agent\":\"claude\",\"body\":\"Needs your attention\",\"pid\":'$$'}'",
            "async": true
          }
        ]
      }
    ]
  }
}
```

The `$$` sends the shell's PID so the server can check if the agent's terminal is focused.
Start the server separately with `ding-ding serve`.

On macOS, focus detection includes multiplexer-aware fallback for `zellij` and
`tmux` sessions. If multiplexer focus cannot be determined reliably, ding-ding
defaults to suppression (treats focus as uncertain rather than unfocused) to
avoid noisy false-positive notifications.

#### OpenCode

Use the setup command (recommended):

```bash
ding-ding agent init opencode project
# or
ding-ding agent init opencode global
```

Manual setup if needed. OpenCode uses TypeScript plugins. Save as `.opencode/plugins/ding-ding.ts`:

**Option A: CLI**
```typescript
import type { Plugin } from "@opencode-ai/plugin"

export const DingDing: Plugin = async ({ $ }) => ({
  event: async ({ event }) => {
    if (event.type === "session.idle") {
      await $`ding-ding notify -a opencode -m "Task finished"`
    }
  },
})
```

**Option B: Server mode**
```typescript
import type { Plugin } from "@opencode-ai/plugin"

export const DingDing: Plugin = async ({ $ }) => ({
  event: async ({ event }) => {
    if (event.type === "session.idle") {
      const payload = JSON.stringify({
        agent: "opencode",
        body: "Task finished",
        pid: process.pid,
      })
      await $`curl -s localhost:8228/notify -H "Content-Type: application/json" -d ${payload}`
    }
  },
})
```

#### Generic shell hook

```bash
# After any long-running agent command
my-agent run --task "refactor" && ding-ding notify -a my-agent -m "Refactor complete"
```

## Configuration

Config lives at `~/.config/ding-ding/config.yaml`. All fields are optional — sensible defaults are built in.

```bash
# Create default config
ding-ding config init

# Show config path
ding-ding config path
# ~/.config/ding-ding/config.yaml
```

### Example config

```yaml
# ntfy push notifications (https://ntfy.sh)
ntfy:
  enabled: true
  server: "https://your-ntfy-server.com"
  topic: "ding-ding"
  token: ""
  priority: "high"

# Discord webhook
discord:
  enabled: false
  webhook_url: "https://discord.com/api/webhooks/..."

# Generic webhook
webhook:
  enabled: false
  url: "https://example.com/hook"
  method: "POST"

# Send push notifications only when idle for 5+ minutes
idle:
  threshold_seconds: 300
  fallback_policy: "active" # active or idle

# Skip system notification when the agent terminal is focused
notification:
  suppress_when_focused: true

# HTTP server
server:
  address: "127.0.0.1:8228"

# Persistent structured logging
logging:
  enabled: false
  level: "info"     # error, warn, info, debug
  dir: "logs"
  max_size_mb: 20
  max_backups: 7
  compress: false
```

## Logging

Enable persistent logs by setting `logging.enabled: true` in your config.

- **Log files:** `cli.log` (CLI runs) and `server.log` (HTTP server) inside the configured `logging.dir` directory (default: `logs/` relative to the working directory).
- **Format:** JSON lines with UTC timestamps and correlated lifecycle fields (`request_id`, `operation_id`, `status`, `duration_ms`).
- **Levels:** `error`, `warn`, `info`, `debug` via `logging.level` (applies on next process start).
- **Redaction:** Known sensitive keys are masked as `[REDACTED]`; request payloads are logged as metadata only (shape/size/field names), not raw body/query content.
- **Retention controls:** `logging.max_size_mb` controls rotation threshold, `logging.max_backups` bounds retained files, and `logging.compress` toggles compression of rotated files.

## Notification Flow

```
Agent completes task
       │
       ▼
  Check focus + idle
       │
  ┌────┼──────────────┐
  │    │               │
Focused  Unfocused    Idle
  │    │               │
 quiet  system        system notification
        notification  + push via:
        only            ├─ ntfy
                        ├─ Discord
                        └─ Webhook
```

## Platforms

| Feature | Linux | macOS | Windows |
|---------|-------|-------|---------|
| System notifications | `notify-send` | `osascript` | PowerShell toast |
| Idle detection | `xprintidle` / DBus | `ioreg` | `GetLastInputInfo` |
| Focus detection | `xdotool` / `kdotool` | `osascript` + multiplexer-aware fallback (`zellij`, `tmux`) | `GetForegroundWindow` |
| ntfy / Discord / Webhook | ✓ | ✓ | ✓ |

## Maintainer Quality Gate

Before merging, run the canonical quality gate:

```bash
make quality
```

The gate is strict and merge-blocking. It runs:

- `go test ./...`
- `go vet ./...`
- `gofmt -l .` (check-only; does not auto-format)
- a static guardrail that blocks `log.Fatal`, `os.Exit`, and `panic(` in `internal/`

Only a final `QUALITY GATE PASS` should be treated as a valid pre-merge signal.

## License

MIT
