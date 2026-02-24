package cmd

import (
	"bytes"
	"errors"
	"log/slog"
	"os"
	"strings"
	"testing"

	"github.com/Digni/ding-ding/internal/config"
	"github.com/Digni/ding-ding/internal/logging"
	"github.com/Digni/ding-ding/internal/notifier"
	"github.com/spf13/cobra"
)

func TestHasMistypedTestLocalArg(t *testing.T) {
	tests := []struct {
		name string
		argv []string
		want bool
	}{
		{
			name: "correct long flag",
			argv: []string{"notify", "--test-local", "-m", "hi"},
			want: false,
		},
		{
			name: "mistyped single dash",
			argv: []string{"notify", "-test-local", "-m", "hi"},
			want: true,
		},
		{
			name: "message value looks like flag",
			argv: []string{"notify", "-m", "-test-local"},
			want: false,
		},
		{
			name: "mistyped with equals",
			argv: []string{"notify", "-test-local=true", "-m", "hi"},
			want: true,
		},
		{
			name: "positional after double dash",
			argv: []string{"notify", "--", "-test-local"},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasMistypedTestLocalArg(tt.argv)
			if got != tt.want {
				t.Fatalf("hasMistypedTestLocalArg(%v) = %v, want %v", tt.argv, got, tt.want)
			}
		})
	}
}

func TestIsBestEffortNotifyError(t *testing.T) {
	if isBestEffortNotifyError(nil) {
		t.Fatal("expected nil error to be non-best-effort")
	}

	deliveryErr := &notifyDeliveryError{err: errors.New("push backend unavailable")}
	if !isBestEffortNotifyError(deliveryErr) {
		t.Fatal("expected notifyDeliveryError to be best-effort")
	}

	wrapped := errors.New("wrap: " + deliveryErr.Error())
	if isBestEffortNotifyError(wrapped) {
		t.Fatal("expected plain wrapped string error to be non-best-effort")
	}

	wrappedTyped := errors.Join(errors.New("prefix"), deliveryErr)
	if !isBestEffortNotifyError(wrappedTyped) {
		t.Fatal("expected joined typed delivery error to be best-effort")
	}
}

func TestNotifyRunE_InitializesLoggingBeforeDispatch(t *testing.T) {
	origBootstrap := commandLoggingBootstrap
	origNotifyWithOptions := notifyWithOptions
	origNotifyLoadConfig := notifyLoadConfig
	defer func() {
		commandLoggingBootstrap = origBootstrap
		notifyWithOptions = origNotifyWithOptions
		notifyLoadConfig = origNotifyLoadConfig
	}()

	notifyTitle = ""
	notifyMessage = ""
	notifyAgent = ""
	forcePush = false
	testLocal = false
	notifyLoadConfig = func() (config.LoadResult, error) {
		cfg := config.DefaultConfig()
		cfg.Logging.Level = "debug"
		return config.LoadResult{Config: cfg}, nil
	}

	var callOrder []string
	commandLoggingBootstrap = func(cfg config.LoggingConfig, role logging.Role) error {
		if role != logging.RoleCLI {
			t.Fatalf("role = %q, want %q", role, logging.RoleCLI)
		}
		if cfg.Level != "debug" {
			t.Fatalf("logging level = %q, want %q", cfg.Level, "debug")
		}
		callOrder = append(callOrder, "bootstrap")
		return nil
	}
	notifyWithOptions = func(cfg config.Config, msg notifier.Message, opts notifier.NotifyOptions) error {
		callOrder = append(callOrder, "notify")
		if msg.Body != "hello world" {
			t.Fatalf("msg.Body = %q, want %q", msg.Body, "hello world")
		}
		return nil
	}

	origArgs := os.Args
	os.Args = []string{"ding-ding", "notify"}
	defer func() { os.Args = origArgs }()

	cmd := &cobra.Command{}
	if err := notifyCmd.RunE(cmd, []string{"hello", "world"}); err != nil {
		t.Fatalf("RunE returned error: %v", err)
	}

	if len(callOrder) != 2 {
		t.Fatalf("call count = %d, want 2", len(callOrder))
	}
	if callOrder[0] != "bootstrap" || callOrder[1] != "notify" {
		t.Fatalf("call order = %v, want [bootstrap notify]", callOrder)
	}
}

func TestNotifyRunE_ContinuesWhenLoggingBootstrapFails(t *testing.T) {
	origBootstrap := commandLoggingBootstrap
	origNotifyWithOptions := notifyWithOptions
	origNotifyLoadConfig := notifyLoadConfig
	defer func() {
		commandLoggingBootstrap = origBootstrap
		notifyWithOptions = origNotifyWithOptions
		notifyLoadConfig = origNotifyLoadConfig
	}()

	notifyTitle = ""
	notifyMessage = ""
	notifyAgent = ""
	forcePush = false
	testLocal = false
	notifyLoadConfig = func() (config.LoadResult, error) {
		cfg := config.DefaultConfig()
		return config.LoadResult{Config: cfg}, nil
	}

	commandLoggingBootstrap = func(config.LoggingConfig, logging.Role) error {
		return errors.New("sink init failed")
	}

	notifyCalled := false
	notifyWithOptions = func(_ config.Config, _ notifier.Message, _ notifier.NotifyOptions) error {
		notifyCalled = true
		return nil
	}

	origArgs := os.Args
	os.Args = []string{"ding-ding", "notify"}
	defer func() { os.Args = origArgs }()

	var stderr bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetErr(&stderr)

	if err := notifyCmd.RunE(cmd, []string{"hello"}); err != nil {
		t.Fatalf("RunE returned error: %v", err)
	}
	if !notifyCalled {
		t.Fatal("expected notifier to be called despite bootstrap failure")
	}
	if !strings.Contains(stderr.String(), "warning: unable to initialize persistent logging") {
		t.Fatalf("stderr %q does not contain bootstrap warning", stderr.String())
	}
}

func TestNotifyRunE_DisabledLoggingSuppressesStructuredOutput(t *testing.T) {
	origBootstrap := commandLoggingBootstrap
	origNotifyWithOptions := notifyWithOptions
	origNotifyLoadConfig := notifyLoadConfig
	defer func() {
		commandLoggingBootstrap = origBootstrap
		notifyWithOptions = origNotifyWithOptions
		notifyLoadConfig = origNotifyLoadConfig
	}()

	notifyTitle = ""
	notifyMessage = ""
	notifyAgent = ""
	forcePush = false
	testLocal = false

	notifyLoadConfig = func() (config.LoadResult, error) {
		cfg := config.DefaultConfig()
		cfg.Logging.Enabled = false
		return config.LoadResult{Config: cfg}, nil
	}

	notifyWithOptions = func(_ config.Config, _ notifier.Message, _ notifier.NotifyOptions) error {
		slog.Default().Info("probe")
		return nil
	}

	origArgs := os.Args
	os.Args = []string{"ding-ding", "notify"}
	defer func() { os.Args = origArgs }()

	var stderr bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetErr(&stderr)

	if err := notifyCmd.RunE(cmd, []string{"hello"}); err != nil {
		t.Fatalf("RunE returned error: %v", err)
	}

	if got := strings.TrimSpace(stderr.String()); got != "" {
		t.Fatalf("expected no structured log output when logging is disabled, got %q", got)
	}
}
