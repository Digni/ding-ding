package agentsetup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUpsert_ClaudeProject(t *testing.T) {
	cwd := t.TempDir()

	result, err := Upsert(Options{
		Agent: AgentClaude,
		Scope: ScopeProject,
		Mode:  ModeCLI,
		CWD:   cwd,
	})
	if err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}

	wantPath := filepath.Join(cwd, ".claude", "settings.json")
	if result.Path != wantPath {
		t.Fatalf("result.Path = %q, want %q", result.Path, wantPath)
	}
	if result.Status != StatusCreated {
		t.Fatalf("result.Status = %q, want %q", result.Status, StatusCreated)
	}

	result, err = Upsert(Options{
		Agent: AgentClaude,
		Scope: ScopeProject,
		Mode:  ModeCLI,
		CWD:   cwd,
	})
	if err != nil {
		t.Fatalf("second Upsert() error = %v", err)
	}
	if result.Status != StatusUnchanged {
		t.Fatalf("second result.Status = %q, want %q", result.Status, StatusUnchanged)
	}
}

func TestUpsert_OpenCodeGlobal(t *testing.T) {
	home := t.TempDir()

	result, err := Upsert(Options{
		Agent:   AgentOpenCode,
		Scope:   ScopeGlobal,
		Mode:    ModeCLI,
		HomeDir: home,
	})
	if err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}

	wantPath := filepath.Join(home, ".config", "opencode", "plugins", "ding-ding.ts")
	if result.Path != wantPath {
		t.Fatalf("result.Path = %q, want %q", result.Path, wantPath)
	}
	if result.Status != StatusCreated {
		t.Fatalf("result.Status = %q, want %q", result.Status, StatusCreated)
	}
}

func TestUpsert_PropagatesClaudeParseError(t *testing.T) {
	cwd := t.TempDir()
	path := filepath.Join(cwd, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte("{ invalid"), 0o644); err != nil {
		t.Fatalf("write invalid settings: %v", err)
	}

	_, err := Upsert(Options{
		Agent: AgentClaude,
		Scope: ScopeProject,
		Mode:  ModeCLI,
		CWD:   cwd,
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "parse claude settings") {
		t.Fatalf("error = %q, want parse context", err)
	}
}
