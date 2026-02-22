package agentsetup

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUpsertClaudeSettings_CreatesMinimalSettings(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".claude", "settings.json")

	status, err := upsertClaudeSettings(path, ModeCLI)
	if err != nil {
		t.Fatalf("upsertClaudeSettings() error = %v", err)
	}
	if status != StatusCreated {
		t.Fatalf("status = %q, want %q", status, StatusCreated)
	}

	root := readJSONFile(t, path)
	if _, ok := root["hooks"]; !ok {
		t.Fatal("expected hooks object to be present")
	}

	stopCommands := eventCommands(t, root, claudeEventStop)
	notificationCommands := eventCommands(t, root, claudeEventNotification)

	assertContainsCommand(t, stopCommands, "ding-ding notify -a claude -m 'Task finished'")
	assertContainsCommand(t, notificationCommands, "ding-ding notify -a claude -m 'Needs your attention'")
}

func TestUpsertClaudeSettings_MergesAndDeduplicatesManagedHooks(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	seed := `{
  "theme": "dark",
  "hooks": {
    "Stop": [
      {
        "hooks": [
          {"type": "command", "command": "echo keep-stop", "async": true},
          {"type": "command", "command": "ding-ding notify -a claude -m 'old'", "async": true}
        ]
      },
      {
        "hooks": [
          {"type": "command", "command": "ding-ding notify -a claude -m 'older'", "async": true}
        ]
      }
    ],
    "Notification": [
      {
        "hooks": [
          {"type": "command", "command": "echo keep-notification", "async": false}
        ]
      },
      {
        "hooks": [
          {"type": "command", "command": "curl -s localhost:8228/notify -d '{\"agent\":\"claude\",\"body\":\"Task finished\",\"pid\":'$$'}'", "async": true}
        ]
      }
    ]
  }
}`
	if err := os.WriteFile(path, []byte(seed), 0o644); err != nil {
		t.Fatalf("write seed: %v", err)
	}

	status, err := upsertClaudeSettings(path, ModeCLI)
	if err != nil {
		t.Fatalf("upsertClaudeSettings() error = %v", err)
	}
	if status != StatusUpdated {
		t.Fatalf("status = %q, want %q", status, StatusUpdated)
	}

	root := readJSONFile(t, path)
	if got, _ := root["theme"].(string); got != "dark" {
		t.Fatalf("theme = %q, want %q", got, "dark")
	}

	stopCommands := eventCommands(t, root, claudeEventStop)
	notificationCommands := eventCommands(t, root, claudeEventNotification)

	assertContainsCommand(t, stopCommands, "echo keep-stop")
	assertContainsCommand(t, notificationCommands, "echo keep-notification")
	assertContainsCommand(t, stopCommands, "ding-ding notify -a claude -m 'Task finished'")
	assertContainsCommand(t, notificationCommands, "ding-ding notify -a claude -m 'Needs your attention'")

	if countManagedCommands(claudeEventStop, stopCommands) != 1 {
		t.Fatalf("Stop managed hook count = %d, want 1", countManagedCommands(claudeEventStop, stopCommands))
	}
	if countManagedCommands(claudeEventNotification, notificationCommands) != 1 {
		t.Fatalf("Notification managed hook count = %d, want 1", countManagedCommands(claudeEventNotification, notificationCommands))
	}
}

func TestUpsertClaudeSettings_ModeSwitchUpdatesCommands(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")

	status, err := upsertClaudeSettings(path, ModeCLI)
	if err != nil {
		t.Fatalf("initial upsert error = %v", err)
	}
	if status != StatusCreated {
		t.Fatalf("initial status = %q, want %q", status, StatusCreated)
	}

	status, err = upsertClaudeSettings(path, ModeServer)
	if err != nil {
		t.Fatalf("server upsert error = %v", err)
	}
	if status != StatusUpdated {
		t.Fatalf("server status = %q, want %q", status, StatusUpdated)
	}

	root := readJSONFile(t, path)
	stopCommands := eventCommands(t, root, claudeEventStop)
	notificationCommands := eventCommands(t, root, claudeEventNotification)

	assertContainsCommandWith(t, stopCommands, "curl -s localhost:8228/notify")
	assertContainsCommandWith(t, stopCommands, `"agent":"claude"`)
	assertContainsCommandWith(t, notificationCommands, "Needs your attention")
}

func TestUpsertClaudeSettings_PreservesCustomServerHook(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	customCommand := `curl -s localhost:8228/notify -d '{"agent":"claude","body":"custom","pid":'$$'}'`
	seed := `{
  "hooks": {
    "Stop": [
      {
        "hooks": [
          {"type": "command", "command": "curl -s localhost:8228/notify -d '{\"agent\":\"claude\",\"body\":\"custom\",\"pid\":'$$'}'", "async": true}
        ]
      }
    ]
  }
}`
	if err := os.WriteFile(path, []byte(seed), 0o644); err != nil {
		t.Fatalf("write seed: %v", err)
	}

	status, err := upsertClaudeSettings(path, ModeCLI)
	if err != nil {
		t.Fatalf("upsertClaudeSettings() error = %v", err)
	}
	if status != StatusUpdated {
		t.Fatalf("status = %q, want %q", status, StatusUpdated)
	}

	root := readJSONFile(t, path)
	stopCommands := eventCommands(t, root, claudeEventStop)
	assertContainsCommand(t, stopCommands, customCommand)
	assertContainsCommand(t, stopCommands, "ding-ding notify -a claude -m 'Task finished'")
	if countManagedCommands(claudeEventStop, stopCommands) != 1 {
		t.Fatalf("managed stop commands = %d, want 1", countManagedCommands(claudeEventStop, stopCommands))
	}
}

func TestUpsertClaudeSettings_RemovesEntryWhenManagedHooksLeaveNoHooks(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	seed := `{
  "hooks": {
    "Stop": [
      {
        "matcher": "foo",
        "hooks": [
          {"type": "command", "command": "ding-ding notify -a claude -m 'Task finished'", "async": true}
        ]
      }
    ]
  }
}`
	if err := os.WriteFile(path, []byte(seed), 0o644); err != nil {
		t.Fatalf("write seed: %v", err)
	}

	status, err := upsertClaudeSettings(path, ModeCLI)
	if err != nil {
		t.Fatalf("upsertClaudeSettings() error = %v", err)
	}
	if status != StatusUpdated {
		t.Fatalf("status = %q, want %q", status, StatusUpdated)
	}

	root := readJSONFile(t, path)
	stopEntries := eventEntries(t, root, claudeEventStop)
	for _, entry := range stopEntries {
		if _, hasMatcher := entry["matcher"]; hasMatcher {
			t.Fatalf("matcher entry should have been removed when no hooks remained: %#v", entry)
		}
		hooks, ok := entry["hooks"].([]any)
		if !ok {
			t.Fatalf("entry hooks is missing or invalid: %#v", entry)
		}
		if len(hooks) == 0 {
			t.Fatalf("entry has empty hooks array: %#v", entry)
		}
	}
}

func TestUpsertClaudeSettings_PreservesMatcherEntryWithNonManagedHook(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	seed := `{
  "hooks": {
    "Stop": [
      {
        "matcher": "foo",
        "hooks": [
          {"type": "command", "command": "ding-ding notify -a claude -m 'Task finished'", "async": true},
          {"type": "command", "command": "echo keep-stop", "async": true}
        ]
      }
    ]
  }
}`
	if err := os.WriteFile(path, []byte(seed), 0o644); err != nil {
		t.Fatalf("write seed: %v", err)
	}

	status, err := upsertClaudeSettings(path, ModeCLI)
	if err != nil {
		t.Fatalf("upsertClaudeSettings() error = %v", err)
	}
	if status != StatusUpdated {
		t.Fatalf("status = %q, want %q", status, StatusUpdated)
	}

	root := readJSONFile(t, path)
	stopCommands := eventCommands(t, root, claudeEventStop)
	assertContainsCommand(t, stopCommands, "echo keep-stop")
	assertContainsCommand(t, stopCommands, "ding-ding notify -a claude -m 'Task finished'")
	if countManagedCommands(claudeEventStop, stopCommands) != 1 {
		t.Fatalf("managed stop commands = %d, want 1", countManagedCommands(claudeEventStop, stopCommands))
	}

	stopEntries := eventEntries(t, root, claudeEventStop)
	matcherEntryFound := false
	for _, entry := range stopEntries {
		if entry["matcher"] != "foo" {
			continue
		}
		matcherEntryFound = true
		hooks, ok := entry["hooks"].([]any)
		if !ok {
			t.Fatalf("matcher entry hooks invalid: %#v", entry)
		}
		if len(hooks) != 1 {
			t.Fatalf("matcher entry hook count = %d, want 1", len(hooks))
		}
		hook, ok := hooks[0].(map[string]any)
		if !ok {
			t.Fatalf("matcher entry hook has unexpected type: %#v", hooks[0])
		}
		command, _ := hook["command"].(string)
		if command != "echo keep-stop" {
			t.Fatalf("matcher entry command = %q, want %q", command, "echo keep-stop")
		}
	}
	if !matcherEntryFound {
		t.Fatal("expected matcher entry to be preserved")
	}
}

func TestUpsertClaudeSettings_InvalidJSONFails(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(path, []byte("{ invalid json"), 0o644); err != nil {
		t.Fatalf("write invalid file: %v", err)
	}

	_, err := upsertClaudeSettings(path, ModeCLI)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "parse claude settings") {
		t.Fatalf("error = %q, want parse context", err)
	}
}

func TestUpsertClaudeSettings_UnchangedWhenAlreadyCanonical(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")

	if _, err := upsertClaudeSettings(path, ModeCLI); err != nil {
		t.Fatalf("initial upsert error = %v", err)
	}
	status, err := upsertClaudeSettings(path, ModeCLI)
	if err != nil {
		t.Fatalf("second upsert error = %v", err)
	}
	if status != StatusUnchanged {
		t.Fatalf("status = %q, want %q", status, StatusUnchanged)
	}
}

func readJSONFile(t *testing.T, path string) map[string]any {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file %q: %v", path, err)
	}

	root := map[string]any{}
	if err := json.Unmarshal(data, &root); err != nil {
		t.Fatalf("parse json: %v", err)
	}
	return root
}

func eventCommands(t *testing.T, root map[string]any, event string) []string {
	t.Helper()

	hooks, ok := root["hooks"].(map[string]any)
	if !ok {
		t.Fatalf("hooks is missing or not an object")
	}
	entries, ok := hooks[event].([]any)
	if !ok {
		t.Fatalf("hooks.%s is missing or not an array", event)
	}

	commands := make([]string, 0, 4)
	for _, entry := range entries {
		entryMap, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		hooksList, ok := entryMap["hooks"].([]any)
		if !ok {
			continue
		}
		for _, hook := range hooksList {
			hookMap, ok := hook.(map[string]any)
			if !ok {
				continue
			}
			command, _ := hookMap["command"].(string)
			if command != "" {
				commands = append(commands, command)
			}
		}
	}
	return commands
}

func assertContainsCommand(t *testing.T, commands []string, want string) {
	t.Helper()
	for _, command := range commands {
		if command == want {
			return
		}
	}
	t.Fatalf("commands %v do not contain exact %q", commands, want)
}

func assertContainsCommandWith(t *testing.T, commands []string, contains string) {
	t.Helper()
	for _, command := range commands {
		if strings.Contains(command, contains) {
			return
		}
	}
	t.Fatalf("commands %v do not contain substring %q", commands, contains)
}

func eventEntries(t *testing.T, root map[string]any, event string) []map[string]any {
	t.Helper()

	hooks, ok := root["hooks"].(map[string]any)
	if !ok {
		t.Fatalf("hooks is missing or not an object")
	}
	entries, ok := hooks[event].([]any)
	if !ok {
		t.Fatalf("hooks.%s is missing or not an array", event)
	}

	out := make([]map[string]any, 0, len(entries))
	for _, entry := range entries {
		entryMap, ok := entry.(map[string]any)
		if !ok {
			t.Fatalf("hooks.%s contains non-object entry: %#v", event, entry)
		}
		out = append(out, entryMap)
	}
	return out
}

func countManagedCommands(event string, commands []string) int {
	count := 0
	for _, command := range commands {
		if isManagedClaudeCommand(event, command) {
			count++
		}
	}
	return count
}
