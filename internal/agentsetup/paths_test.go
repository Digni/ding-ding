package agentsetup

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestResolveTargetPath(t *testing.T) {
	tests := []struct {
		name string
		opts Options
		want string
	}{
		{
			name: "claude project",
			opts: Options{Agent: AgentClaude, Scope: ScopeProject, Mode: ModeCLI, CWD: "/repo"},
			want: filepath.Join("/repo", ".claude", "settings.json"),
		},
		{
			name: "claude global",
			opts: Options{Agent: AgentClaude, Scope: ScopeGlobal, Mode: ModeCLI, HomeDir: "/home/alex"},
			want: filepath.Join("/home/alex", ".claude", "settings.json"),
		},
		{
			name: "opencode project",
			opts: Options{Agent: AgentOpenCode, Scope: ScopeProject, Mode: ModeCLI, CWD: "/repo"},
			want: filepath.Join("/repo", ".opencode", "plugins", "ding-ding.ts"),
		},
		{
			name: "opencode global",
			opts: Options{Agent: AgentOpenCode, Scope: ScopeGlobal, Mode: ModeCLI, HomeDir: "/home/alex"},
			want: filepath.Join("/home/alex", ".config", "opencode", "plugins", "ding-ding.ts"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolveTargetPath(tt.opts)
			if err != nil {
				t.Fatalf("ResolveTargetPath() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("ResolveTargetPath() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResolveTargetPath_HomeLookupFailure(t *testing.T) {
	orig := lookupUserHomeDir
	defer func() { lookupUserHomeDir = orig }()

	lookupUserHomeDir = func() (string, error) {
		return "", errors.New("home unavailable")
	}

	_, err := ResolveTargetPath(Options{
		Agent: AgentClaude,
		Scope: ScopeGlobal,
		Mode:  ModeCLI,
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestResolveTargetPath_WorkingDirLookupFailure(t *testing.T) {
	orig := lookupWorkingDir
	defer func() { lookupWorkingDir = orig }()

	lookupWorkingDir = func() (string, error) {
		return "", errors.New("cwd unavailable")
	}

	_, err := ResolveTargetPath(Options{
		Agent: AgentOpenCode,
		Scope: ScopeProject,
		Mode:  ModeCLI,
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
