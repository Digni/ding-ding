package agentsetup

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	claudeEventStop         = "Stop"
	claudeEventNotification = "Notification"
)

func upsertClaudeSettings(path string, mode Mode) (ChangeStatus, error) {
	existing, err := os.ReadFile(path)
	fileExists := err == nil
	if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("read claude settings %q: %w", path, err)
	}

	root := map[string]any{}
	if fileExists {
		if err := json.Unmarshal(existing, &root); err != nil {
			return "", fmt.Errorf("parse claude settings %q: %w", path, err)
		}
	}

	if err := upsertClaudeHooks(root, mode); err != nil {
		return "", fmt.Errorf("upsert claude hooks: %w", err)
	}

	data, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal claude settings: %w", err)
	}
	data = append(data, '\n')

	if fileExists && bytes.Equal(existing, data) {
		return StatusUnchanged, nil
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", fmt.Errorf("create claude settings directory: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", fmt.Errorf("write claude settings %q: %w", path, err)
	}

	if fileExists {
		return StatusUpdated, nil
	}
	return StatusCreated, nil
}

func upsertClaudeHooks(root map[string]any, mode Mode) error {
	hooksValue, ok := root["hooks"]
	if !ok {
		hooksValue = map[string]any{}
		root["hooks"] = hooksValue
	}

	hooks, ok := hooksValue.(map[string]any)
	if !ok {
		return fmt.Errorf("expected hooks to be an object")
	}

	events := []string{claudeEventStop, claudeEventNotification}
	for _, event := range events {
		existingEvent := []any{}
		if value, exists := hooks[event]; exists {
			arr, ok := value.([]any)
			if !ok {
				return fmt.Errorf("expected hooks.%s to be an array", event)
			}
			existingEvent = arr
		}

		cleaned, err := removeManagedClaudeHooks(event, existingEvent)
		if err != nil {
			return err
		}
		hooks[event] = append(cleaned, canonicalClaudeEventHook(event, mode))
	}

	return nil
}

func removeManagedClaudeHooks(event string, entries []any) ([]any, error) {
	cleanedEntries := make([]any, 0, len(entries))

	for _, entry := range entries {
		entryMap, ok := entry.(map[string]any)
		if !ok {
			cleanedEntries = append(cleanedEntries, entry)
			continue
		}

		hooksValue, hasHooks := entryMap["hooks"]
		if !hasHooks {
			cleanedEntries = append(cleanedEntries, entryMap)
			continue
		}

		hookList, ok := hooksValue.([]any)
		if !ok {
			return nil, fmt.Errorf("expected hooks.%s[].hooks to be an array", event)
		}

		filteredHooks := make([]any, 0, len(hookList))
		for _, hook := range hookList {
			hookMap, ok := hook.(map[string]any)
			if !ok {
				filteredHooks = append(filteredHooks, hook)
				continue
			}

			command, _ := hookMap["command"].(string)
			if command != "" && isManagedClaudeCommand(command) {
				continue
			}
			filteredHooks = append(filteredHooks, hookMap)
		}

		if len(filteredHooks) == 0 && mapHasOnlyKey(entryMap, "hooks") {
			continue
		}

		entryMap["hooks"] = filteredHooks
		cleanedEntries = append(cleanedEntries, entryMap)
	}

	return cleanedEntries, nil
}

func canonicalClaudeEventHook(event string, mode Mode) map[string]any {
	return map[string]any{
		"hooks": []any{
			map[string]any{
				"type":    "command",
				"command": claudeCommandForEvent(event, mode),
				"async":   true,
			},
		},
	}
}

func claudeCommandForEvent(event string, mode Mode) string {
	switch mode {
	case ModeServer:
		if event == claudeEventNotification {
			return `curl -s localhost:8228/notify -d '{"agent":"claude","body":"Needs your attention","pid":'$$'}'`
		}
		return `curl -s localhost:8228/notify -d '{"agent":"claude","body":"Task finished","pid":'$$'}'`
	default:
		if event == claudeEventNotification {
			return "ding-ding notify -a claude -m 'Needs your attention'"
		}
		return "ding-ding notify -a claude -m 'Task finished'"
	}
}

func isManagedClaudeCommand(command string) bool {
	if strings.Contains(command, "ding-ding notify -a claude") {
		return true
	}

	return strings.Contains(command, "localhost:8228/notify") &&
		strings.Contains(command, `"agent":"claude"`)
}

func mapHasOnlyKey(value map[string]any, key string) bool {
	if len(value) != 1 {
		return false
	}
	_, ok := value[key]
	return ok
}
