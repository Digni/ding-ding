package agentsetup

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
)

func upsertOpenCodePlugin(path string, mode Mode) (ChangeStatus, error) {
	content := []byte(openCodePluginTemplate(mode))

	existing, err := os.ReadFile(path)
	fileExists := err == nil
	if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("read opencode plugin %q: %w", path, err)
	}

	if fileExists && bytes.Equal(existing, content) {
		return StatusUnchanged, nil
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", fmt.Errorf("create opencode plugin directory: %w", err)
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		return "", fmt.Errorf("write opencode plugin %q: %w", path, err)
	}

	if fileExists {
		return StatusUpdated, nil
	}
	return StatusCreated, nil
}

func openCodePluginTemplate(mode Mode) string {
	switch mode {
	case ModeServer:
		return `import type { Plugin } from "@opencode-ai/plugin"

export const DingDing: Plugin = async ({ $ }) => ({
  event: async ({ event }) => {
    if (event.type === "session.idle") {
      const payload = JSON.stringify({
        agent: "opencode",
        body: "Task finished",
        pid: process.pid,
      })
      await $` + "`" + `curl -s localhost:8228/notify -H "Content-Type: application/json" -d ${payload}` + "`" + `
    }
  },
})
`
	default:
		return `import type { Plugin } from "@opencode-ai/plugin"

export const DingDing: Plugin = async ({ $ }) => ({
  event: async ({ event }) => {
    if (event.type === "session.idle") {
      await $` + "`" + `ding-ding notify -a opencode -m "Task finished"` + "`" + `
    }
  },
})
`
	}
}
