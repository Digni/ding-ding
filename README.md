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

# Force push (ignore idle check, always send to ntfy/Discord/webhook)
ding-ding notify -p -m "Deploy complete"
```

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

Add to `.claude/settings.json` (project) or `~/.claude/settings.json` (global):

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

#### OpenCode

OpenCode uses TypeScript plugins. Save as `.opencode/plugins/ding-ding.ts`:

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
      await $`curl -s localhost:8228/notify -d '{"agent":"opencode","body":"Task finished","pid":'$$'}'`
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
```

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
| Focus detection | `xdotool` / `kdotool` | `osascript` | `GetForegroundWindow` |
| ntfy / Discord / Webhook | ✓ | ✓ | ✓ |

## License

MIT
