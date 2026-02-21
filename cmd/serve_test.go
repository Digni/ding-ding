package cmd

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/Digni/ding-ding/internal/config"
	"github.com/Digni/ding-ding/internal/logging"
	"github.com/spf13/cobra"
)

func TestServeRunE_InitializesLoggingBeforeServerStart(t *testing.T) {
	origBootstrap := commandLoggingBootstrap
	origStartServer := startServer
	origServeLoadConfig := serveLoadConfig
	defer func() {
		commandLoggingBootstrap = origBootstrap
		startServer = origStartServer
		serveLoadConfig = origServeLoadConfig
	}()

	serveAddress = ""
	serveLoadConfig = func() (config.LoadResult, error) {
		cfg := config.DefaultConfig()
		cfg.Logging.Level = "warn"
		return config.LoadResult{Config: cfg}, nil
	}

	var callOrder []string
	commandLoggingBootstrap = func(cfg config.LoggingConfig, role logging.Role) error {
		if role != logging.RoleServer {
			t.Fatalf("role = %q, want %q", role, logging.RoleServer)
		}
		if cfg.Level != "warn" {
			t.Fatalf("logging level = %q, want %q", cfg.Level, "warn")
		}
		callOrder = append(callOrder, "bootstrap")
		return nil
	}

	startServer = func(cfg config.Config) error {
		callOrder = append(callOrder, "start")
		if cfg.Server.Address != "127.0.0.1:8228" {
			t.Fatalf("server address = %q, want %q", cfg.Server.Address, "127.0.0.1:8228")
		}
		return nil
	}

	cmd := &cobra.Command{}
	if err := serveCmd.RunE(cmd, nil); err != nil {
		t.Fatalf("RunE returned error: %v", err)
	}

	if len(callOrder) != 2 {
		t.Fatalf("call count = %d, want 2", len(callOrder))
	}
	if callOrder[0] != "bootstrap" || callOrder[1] != "start" {
		t.Fatalf("call order = %v, want [bootstrap start]", callOrder)
	}
}

func TestServeRunE_ContinuesWhenLoggingBootstrapFails(t *testing.T) {
	origBootstrap := commandLoggingBootstrap
	origStartServer := startServer
	origServeLoadConfig := serveLoadConfig
	defer func() {
		commandLoggingBootstrap = origBootstrap
		startServer = origStartServer
		serveLoadConfig = origServeLoadConfig
	}()

	serveAddress = ""
	serveLoadConfig = func() (config.LoadResult, error) {
		cfg := config.DefaultConfig()
		return config.LoadResult{Config: cfg}, nil
	}

	commandLoggingBootstrap = func(config.LoggingConfig, logging.Role) error {
		return errors.New("writer unavailable")
	}

	started := false
	startServer = func(config.Config) error {
		started = true
		return nil
	}

	var stderr bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetErr(&stderr)

	if err := serveCmd.RunE(cmd, nil); err != nil {
		t.Fatalf("RunE returned error: %v", err)
	}
	if !started {
		t.Fatal("expected server start to run despite bootstrap failure")
	}
	if !strings.Contains(stderr.String(), "warning: unable to initialize persistent logging") {
		t.Fatalf("stderr %q does not contain bootstrap warning", stderr.String())
	}
}
