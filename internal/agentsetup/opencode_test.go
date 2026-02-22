package agentsetup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUpsertOpenCodePlugin_CreatesFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".opencode", "plugins", "ding-ding.ts")

	status, err := upsertOpenCodePlugin(path, ModeCLI)
	if err != nil {
		t.Fatalf("upsertOpenCodePlugin() error = %v", err)
	}
	if status != StatusCreated {
		t.Fatalf("status = %q, want %q", status, StatusCreated)
	}

	content := readTextFile(t, path)
	if !strings.Contains(content, `ding-ding notify -a opencode -m "Task finished"`) {
		t.Fatalf("unexpected plugin content:\n%s", content)
	}
}

func TestUpsertOpenCodePlugin_UpdatesWhenContentDiffers(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".opencode", "plugins", "ding-ding.ts")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte("// old"), 0o644); err != nil {
		t.Fatalf("write old file: %v", err)
	}

	status, err := upsertOpenCodePlugin(path, ModeCLI)
	if err != nil {
		t.Fatalf("upsertOpenCodePlugin() error = %v", err)
	}
	if status != StatusUpdated {
		t.Fatalf("status = %q, want %q", status, StatusUpdated)
	}
}

func TestUpsertOpenCodePlugin_UnchangedWhenIdentical(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".opencode", "plugins", "ding-ding.ts")

	if _, err := upsertOpenCodePlugin(path, ModeCLI); err != nil {
		t.Fatalf("initial upsert error = %v", err)
	}
	status, err := upsertOpenCodePlugin(path, ModeCLI)
	if err != nil {
		t.Fatalf("second upsert error = %v", err)
	}
	if status != StatusUnchanged {
		t.Fatalf("status = %q, want %q", status, StatusUnchanged)
	}
}

func TestUpsertOpenCodePlugin_ServerTemplate(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".opencode", "plugins", "ding-ding.ts")

	if _, err := upsertOpenCodePlugin(path, ModeServer); err != nil {
		t.Fatalf("upsert server error = %v", err)
	}

	content := readTextFile(t, path)
	if !strings.Contains(content, "localhost:8228/notify") {
		t.Fatalf("expected curl integration, got:\n%s", content)
	}
	if !strings.Contains(content, "process.pid") {
		t.Fatalf("expected process.pid payload, got:\n%s", content)
	}
}

func readTextFile(t *testing.T, path string) string {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file %q: %v", path, err)
	}
	return string(data)
}
