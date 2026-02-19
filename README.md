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

# Quick GET request
curl "localhost:8228/notify?message=done&agent=claude"
```

### Agent Integration Examples

**Claude Code hook** (`.claude/hooks.json`):
```json
{
  "hooks": {
    "PostToolUse": [
      {
        "matcher": "Stop",
        "hooks": [
          {
            "type": "command",
            "command": "ding-ding notify -a claude -m 'Claude has finished the task'"
          }
        ]
      }
    ]
  }
}
```

**Generic shell hook:**
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

# Skip system notification when the agent terminal is focused
notification:
  suppress_when_focused: true

# HTTP server
server:
  address: ":8228"
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
