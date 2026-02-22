package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Digni/ding-ding/internal/agentsetup"
	"github.com/spf13/cobra"
)

func TestAgentInitAndUpdateBothUseUpsert(t *testing.T) {
	origUpsert := agentUpsert
	origMode := agentMode
	defer func() {
		agentUpsert = origUpsert
		agentMode = origMode
	}()

	agentMode = string(agentsetup.ModeCLI)
	calls := 0
	agentUpsert = func(opts agentsetup.Options) (agentsetup.Result, error) {
		calls++
		if opts.Agent != agentsetup.AgentClaude {
			t.Fatalf("opts.Agent = %q, want %q", opts.Agent, agentsetup.AgentClaude)
		}
		if opts.Scope != agentsetup.ScopeProject {
			t.Fatalf("opts.Scope = %q, want %q", opts.Scope, agentsetup.ScopeProject)
		}
		if opts.Mode != agentsetup.ModeCLI {
			t.Fatalf("opts.Mode = %q, want %q", opts.Mode, agentsetup.ModeCLI)
		}
		return agentsetup.Result{
			Path:   "/tmp/path",
			Status: agentsetup.StatusUpdated,
		}, nil
	}

	for _, c := range []*cobra.Command{agentInitCmd, agentUpdateCmd} {
		var stdout bytes.Buffer
		cmd := &cobra.Command{}
		cmd.SetOut(&stdout)

		if err := c.RunE(cmd, []string{"claude", "project"}); err != nil {
			t.Fatalf("RunE returned error: %v", err)
		}
		if !strings.Contains(stdout.String(), "configured claude (project, mode=cli): /tmp/path [updated]") {
			t.Fatalf("unexpected output %q", stdout.String())
		}
	}

	if calls != 2 {
		t.Fatalf("upsert calls = %d, want 2", calls)
	}
}

func TestAgentRunE_InvalidAgent(t *testing.T) {
	err := runAgentConfigure(&cobra.Command{}, []string{"unknown", "project"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "invalid agent") {
		t.Fatalf("error %q does not contain %q", err, "invalid agent")
	}
}

func TestAgentRunE_InvalidScope(t *testing.T) {
	err := runAgentConfigure(&cobra.Command{}, []string{"claude", "workspace"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "invalid scope") {
		t.Fatalf("error %q does not contain %q", err, "invalid scope")
	}
}

func TestAgentRunE_InvalidMode(t *testing.T) {
	origMode := agentMode
	defer func() { agentMode = origMode }()
	agentMode = "bad"

	err := runAgentConfigure(&cobra.Command{}, []string{"claude", "project"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "invalid mode") {
		t.Fatalf("error %q does not contain %q", err, "invalid mode")
	}
}

func TestAgentRunE_ServerModePassesThrough(t *testing.T) {
	origUpsert := agentUpsert
	origMode := agentMode
	defer func() {
		agentUpsert = origUpsert
		agentMode = origMode
	}()

	agentMode = string(agentsetup.ModeServer)
	agentUpsert = func(opts agentsetup.Options) (agentsetup.Result, error) {
		if opts.Mode != agentsetup.ModeServer {
			t.Fatalf("opts.Mode = %q, want %q", opts.Mode, agentsetup.ModeServer)
		}
		return agentsetup.Result{
			Path:   "/tmp/server",
			Status: agentsetup.StatusCreated,
		}, nil
	}

	var stdout bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&stdout)

	if err := runAgentConfigure(cmd, []string{"opencode", "global"}); err != nil {
		t.Fatalf("runAgentConfigure returned error: %v", err)
	}
	if !strings.Contains(stdout.String(), "configured opencode (global, mode=server): /tmp/server [created]") {
		t.Fatalf("unexpected output %q", stdout.String())
	}
}
