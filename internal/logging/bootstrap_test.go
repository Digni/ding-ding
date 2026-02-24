package logging

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/Digni/ding-ding/internal/config"
)

func TestBootstrap_JSONShapeAndUTCTimestamp(t *testing.T) {
	var out bytes.Buffer

	cfg := config.LoggingConfig{
		Enabled:    true,
		Level:      "debug",
		Dir:        t.TempDir(),
		MaxSizeMB:  20,
		MaxBackups: 7,
		Compress:   false,
	}

	logger := bootstrapWithOptions(cfg, RoleCLI, bootstrapOptions{
		newWriter: func(_ string, _ config.LoggingConfig) io.Writer {
			return &out
		},
		retries: 1,
		sleep:   func(time.Duration) {},
	})

	logger.Info("logging ready", slog.String("component", "tests"), slog.String("token", "abc123"), slog.String("api_key", "xyz"))

	line := strings.TrimSpace(out.String())
	if line == "" {
		t.Fatal("expected one JSON log line, got empty output")
	}

	var record map[string]any
	if err := json.Unmarshal([]byte(line), &record); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	if record["msg"] != "logging ready" {
		t.Fatalf("msg = %v, want %q", record["msg"], "logging ready")
	}
	if record["level"] != "INFO" {
		t.Fatalf("level = %v, want %q", record["level"], "INFO")
	}
	if record["component"] != "tests" {
		t.Fatalf("component = %v, want %q", record["component"], "tests")
	}
	if record["token"] != redactedValue {
		t.Fatalf("token = %v, want %q", record["token"], redactedValue)
	}
	if record["api_key"] != redactedValue {
		t.Fatalf("api_key = %v, want %q", record["api_key"], redactedValue)
	}

	timestamp, ok := record["time"].(string)
	if !ok {
		t.Fatalf("time field missing or not a string: %T", record["time"])
	}
	if _, err := time.Parse(time.RFC3339, timestamp); err != nil {
		t.Fatalf("time %q is not valid RFC3339: %v", timestamp, err)
	}
	if !strings.HasSuffix(timestamp, "Z") {
		t.Fatalf("time %q is not UTC", timestamp)
	}
}

func TestBootstrap_LevelFiltering(t *testing.T) {
	var out bytes.Buffer

	cfg := config.LoggingConfig{
		Enabled:    true,
		Level:      "warn",
		Dir:        t.TempDir(),
		MaxSizeMB:  20,
		MaxBackups: 7,
		Compress:   false,
	}

	logger := bootstrapWithOptions(cfg, RoleCLI, bootstrapOptions{
		newWriter: func(_ string, _ config.LoggingConfig) io.Writer {
			return &out
		},
		retries: 1,
		sleep:   func(time.Duration) {},
	})

	logger.Info("info skipped")
	logger.Warn("warn emitted")

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 1 {
		t.Fatalf("line count = %d, want 1", len(lines))
	}
	if !strings.Contains(lines[0], "warn emitted") {
		t.Fatalf("log line %q does not contain warning message", lines[0])
	}
}

func TestBootstrap_DisabledDoesNotEmitLogs(t *testing.T) {
	var fallback bytes.Buffer

	cfg := config.LoggingConfig{
		Enabled:    false,
		Level:      "debug",
		Dir:        t.TempDir(),
		MaxSizeMB:  20,
		MaxBackups: 7,
		Compress:   false,
	}

	logger := bootstrapWithOptions(cfg, RoleCLI, bootstrapOptions{
		fallbackWriter: &fallback,
	})

	logger.Info("should be discarded")

	if fallback.Len() != 0 {
		t.Fatalf("expected no log output when logging is disabled, got %q", fallback.String())
	}
}

func TestBootstrap_RetryThenWarnFallback(t *testing.T) {
	var warning bytes.Buffer
	var fallback bytes.Buffer

	cfg := config.LoggingConfig{
		Enabled:    true,
		Level:      "info",
		Dir:        t.TempDir(),
		MaxSizeMB:  20,
		MaxBackups: 7,
		Compress:   false,
	}

	logger := bootstrapWithOptions(cfg, RoleCLI, bootstrapOptions{
		newWriter: func(_ string, _ config.LoggingConfig) io.Writer {
			return failingWriter{err: errors.New("write failed")}
		},
		warnWriter:     &warning,
		fallbackWriter: &fallback,
		retries:        3,
		retryDelay:     0,
		sleep:          func(time.Duration) {},
	})

	logger.Error("persist failed", slog.String("password", "12345"))

	if !strings.Contains(warning.String(), "falling back to stderr") {
		t.Fatalf("warning output %q does not contain fallback warning", warning.String())
	}
	if !strings.Contains(fallback.String(), redactedValue) {
		t.Fatalf("fallback log %q does not contain redaction marker", fallback.String())
	}
	if !strings.Contains(fallback.String(), "persist failed") {
		t.Fatalf("fallback log %q does not contain original message", fallback.String())
	}
}

type failingWriter struct {
	err error
}

func (f failingWriter) Write(_ []byte) (int, error) {
	return 0, f.err
}
