//go:build darwin

package focus

import (
	"errors"
	"testing"
)

func setupDarwinFocusStubs(t *testing.T) {
	t.Helper()

	origFrontmost := frontmostPIDFunc
	origEnv := processEnvFunc
	origList := processListFunc
	origTmuxClients := tmuxClientPIDsFn

	t.Cleanup(func() {
		frontmostPIDFunc = origFrontmost
		processEnvFunc = origEnv
		processListFunc = origList
		tmuxClientPIDsFn = origTmuxClients
	})
}

func TestProcessInFocusedTerminalState_DirectAncestor(t *testing.T) {
	setupDarwinFocusStubs(t)
	setupParentPIDStub(t, map[int]int{
		100: 200,
	})

	frontmostPIDFunc = func() (int, bool) { return 200, true }

	envCalled := false
	processEnvFunc = func(pid int) (map[string]string, error) {
		envCalled = true
		return map[string]string{}, nil
	}

	focused, known := processInFocusedTerminalState(100)
	if !focused || !known {
		t.Fatalf("got focused=%v known=%v, want focused=true known=true", focused, known)
	}
	if envCalled {
		t.Fatal("expected processEnv lookup to be skipped on direct ancestor match")
	}
}

func TestProcessInFocusedTerminalState_ZellijFocusedSession(t *testing.T) {
	setupDarwinFocusStubs(t)
	setupParentPIDStub(t, map[int]int{
		100:  50, // target process chain does not include focused PID
		50:   1,
		300:  999, // zellij client process is under focused app
		999:  1,
		3000: 1,
	})

	frontmostPIDFunc = func() (int, bool) { return 999, true }
	processEnvFunc = func(pid int) (map[string]string, error) {
		return map[string]string{"ZELLIJ_SESSION_NAME": "cyphant_homepage"}, nil
	}
	processListFunc = func() ([]processInfo, error) {
		return []processInfo{
			{PID: 3000, Comm: "zellij", Args: "zellij --server /tmp/socket"},
			{PID: 300, Comm: "zellij", Args: "zellij -s cyphant_homepage"},
		}, nil
	}

	focused, known := processInFocusedTerminalState(100)
	if !focused || !known {
		t.Fatalf("got focused=%v known=%v, want focused=true known=true", focused, known)
	}
}

func TestProcessInFocusedTerminalState_ZellijSessionMismatch(t *testing.T) {
	setupDarwinFocusStubs(t)
	setupParentPIDStub(t, map[int]int{
		100: 20,
		20:  1,
		300: 999,
		999: 1,
	})

	frontmostPIDFunc = func() (int, bool) { return 999, true }
	processEnvFunc = func(pid int) (map[string]string, error) {
		return map[string]string{"ZELLIJ_SESSION_NAME": "cyphant_homepage"}, nil
	}
	processListFunc = func() ([]processInfo, error) {
		return []processInfo{
			{PID: 300, Comm: "zellij", Args: "zellij -s other_session"},
		}, nil
	}

	focused, known := processInFocusedTerminalState(100)
	if focused || !known {
		t.Fatalf("got focused=%v known=%v, want focused=false known=true", focused, known)
	}
}

func TestProcessInFocusedTerminalState_ZellijProbeFailureIsUncertain(t *testing.T) {
	setupDarwinFocusStubs(t)
	setupParentPIDStub(t, map[int]int{
		100: 20,
		20:  1,
		999: 1,
	})

	frontmostPIDFunc = func() (int, bool) { return 999, true }
	processEnvFunc = func(pid int) (map[string]string, error) {
		return map[string]string{"ZELLIJ_SESSION_NAME": "cyphant_homepage"}, nil
	}
	processListFunc = func() ([]processInfo, error) {
		return nil, errors.New("ps failed")
	}

	focused, known := processInFocusedTerminalState(100)
	if focused || known {
		t.Fatalf("got focused=%v known=%v, want focused=false known=false", focused, known)
	}
}

func TestProcessInFocusedTerminalState_TmuxFocusedClient(t *testing.T) {
	setupDarwinFocusStubs(t)
	setupParentPIDStub(t, map[int]int{
		100: 20,
		20:  1,
		700: 999, // tmux client pid under focused app
		999: 1,
	})

	frontmostPIDFunc = func() (int, bool) { return 999, true }
	processEnvFunc = func(pid int) (map[string]string, error) {
		return map[string]string{"TMUX": "/tmp/tmux-501/default,123,0"}, nil
	}
	tmuxClientPIDsFn = func(socketPath string, sessionID string) ([]int, error) {
		if socketPath != "/tmp/tmux-501/default" {
			t.Fatalf("socketPath=%q, want %q", socketPath, "/tmp/tmux-501/default")
		}
		if sessionID != "0" {
			t.Fatalf("sessionID=%q, want %q", sessionID, "0")
		}
		return []int{700}, nil
	}

	focused, known := processInFocusedTerminalState(100)
	if !focused || !known {
		t.Fatalf("got focused=%v known=%v, want focused=true known=true", focused, known)
	}
}

func TestProcessInFocusedTerminalState_TmuxUnfocused(t *testing.T) {
	setupDarwinFocusStubs(t)
	setupParentPIDStub(t, map[int]int{
		100: 20,
		20:  1,
		700: 2, // tmux client pid is not under focused app
		2:   1,
		999: 1,
	})

	frontmostPIDFunc = func() (int, bool) { return 999, true }
	processEnvFunc = func(pid int) (map[string]string, error) {
		return map[string]string{"TMUX": "/tmp/tmux-501/default,123,0"}, nil
	}
	tmuxClientPIDsFn = func(socketPath string, sessionID string) ([]int, error) {
		return []int{700}, nil
	}

	focused, known := processInFocusedTerminalState(100)
	if focused || !known {
		t.Fatalf("got focused=%v known=%v, want focused=false known=true", focused, known)
	}
}

func TestProcessInFocusedTerminalState_TmuxQueryFailureIsUncertain(t *testing.T) {
	setupDarwinFocusStubs(t)
	setupParentPIDStub(t, map[int]int{
		100: 20,
		20:  1,
		999: 1,
	})

	frontmostPIDFunc = func() (int, bool) { return 999, true }
	processEnvFunc = func(pid int) (map[string]string, error) {
		return map[string]string{"TMUX": "/tmp/tmux-501/default,123,0"}, nil
	}
	tmuxClientPIDsFn = func(socketPath string, sessionID string) ([]int, error) {
		return nil, errors.New("tmux failed")
	}

	focused, known := processInFocusedTerminalState(100)
	if focused || known {
		t.Fatalf("got focused=%v known=%v, want focused=false known=false", focused, known)
	}
}

func TestParsePSEnvironmentOutput_ExtractsMultiplexerVars(t *testing.T) {
	out := []byte(`  PID   TT  STAT      TIME COMMAND
17223 s008  S+     0:00.00 claude ZELLIJ_SESSION_NAME=cyphant_homepage TMUX=/tmp/tmux-501/default,123,0 PATH=/usr/bin
`)

	env, err := parsePSEnvironmentOutput(out)
	if err != nil {
		t.Fatalf("parsePSEnvironmentOutput() error = %v", err)
	}

	if env["ZELLIJ_SESSION_NAME"] != "cyphant_homepage" {
		t.Fatalf("ZELLIJ_SESSION_NAME=%q, want %q", env["ZELLIJ_SESSION_NAME"], "cyphant_homepage")
	}
	if env["TMUX"] != "/tmp/tmux-501/default,123,0" {
		t.Fatalf("TMUX=%q, want %q", env["TMUX"], "/tmp/tmux-501/default,123,0")
	}
}

func TestParseProcessListOutput(t *testing.T) {
	out := []byte(`
16734 47930 zellij zellij -s cyphant_homepage
16737 1 zellij /opt/homebrew/bin/zellij --server /tmp/socket
`)

	procs, err := parseProcessListOutput(out)
	if err != nil {
		t.Fatalf("parseProcessListOutput() error = %v", err)
	}
	if len(procs) != 2 {
		t.Fatalf("process count = %d, want 2", len(procs))
	}
	if procs[0].PID != 16734 || procs[0].PPID != 47930 {
		t.Fatalf("first process = %+v, want pid=16734 ppid=47930", procs[0])
	}
	if procs[1].Args == "" {
		t.Fatal("expected args to be parsed for second process")
	}
}

func TestParseTMUXClientPIDOutput(t *testing.T) {
	pids, err := parseTMUXClientPIDOutput([]byte("$0 100\n$1 200\n$0 300\n"), "0")
	if err != nil {
		t.Fatalf("parseTMUXClientPIDOutput() error = %v", err)
	}
	if len(pids) != 2 || pids[0] != 100 || pids[1] != 300 {
		t.Fatalf("pids = %#v, want []int{100,300}", pids)
	}
}

func TestZellijSessionMatches_DoesNotAllowSubstringMatch(t *testing.T) {
	if zellijSessionMatches("zellij -s dev2", "dev") {
		t.Fatal("expected false for substring-only session match")
	}
}

func TestTMUXSocketAndSession(t *testing.T) {
	socketPath, sessionID, ok := tmuxSocketAndSession("/tmp/tmux-501/default,123,$4")
	if !ok {
		t.Fatal("expected tmux socket/session parse to succeed")
	}
	if socketPath != "/tmp/tmux-501/default" {
		t.Fatalf("socketPath=%q, want %q", socketPath, "/tmp/tmux-501/default")
	}
	if sessionID != "4" {
		t.Fatalf("sessionID=%q, want %q", sessionID, "4")
	}
}
