package logging

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Digni/ding-ding/internal/config"
	"gopkg.in/natefinch/lumberjack.v2"
)

const redactedValue = "[REDACTED]"

type Role string

const (
	RoleCLI    Role = "cli"
	RoleServer Role = "server"
)

type bootstrapOptions struct {
	newWriter      func(path string, cfg config.LoggingConfig) io.Writer
	warnWriter     io.Writer
	fallbackWriter io.Writer
	retries        int
	retryDelay     time.Duration
	sleep          func(time.Duration)
}

// Bootstrap configures and sets the process default logger.
func Bootstrap(cfg config.LoggingConfig, role Role) *slog.Logger {
	logger := bootstrapWithOptions(cfg, role, bootstrapOptions{})
	slog.SetDefault(logger)
	return logger
}

func bootstrapWithOptions(cfg config.LoggingConfig, role Role, opts bootstrapOptions) *slog.Logger {
	warnWriter := opts.warnWriter
	if warnWriter == nil {
		warnWriter = os.Stderr
	}

	fallbackWriter := opts.fallbackWriter
	if fallbackWriter == nil {
		fallbackWriter = os.Stderr
	}

	sleepFn := opts.sleep
	if sleepFn == nil {
		sleepFn = time.Sleep
	}

	retries := opts.retries
	if retries <= 0 {
		retries = 3
	}

	retryDelay := opts.retryDelay
	if retryDelay <= 0 {
		retryDelay = 50 * time.Millisecond
	}

	level := parseLevel(cfg.Level)
	handlerOpts := &slog.HandlerOptions{
		Level:       level,
		ReplaceAttr: redactAttr,
	}

	if !cfg.Enabled {
		return slog.New(slog.NewJSONHandler(fallbackWriter, handlerOpts))
	}

	filePath := filepath.Join(cfg.Dir, roleFilename(role))
	if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
		fmt.Fprintf(warnWriter, "warning: unable to initialize log directory %q: %v; falling back to stderr\n", filepath.Dir(filePath), err)
		return slog.New(slog.NewJSONHandler(fallbackWriter, handlerOpts))
	}

	writerFactory := opts.newWriter
	if writerFactory == nil {
		writerFactory = newLumberjackWriter
	}

	primaryWriter := writerFactory(filePath, cfg)
	resilient := &resilientWriter{
		primary:      primaryWriter,
		fallback:     fallbackWriter,
		warn:         warnWriter,
		retries:      retries,
		retryDelay:   retryDelay,
		sleep:        sleepFn,
		fallbackPath: filePath,
	}

	return slog.New(slog.NewJSONHandler(resilient, handlerOpts))
}

func newLumberjackWriter(path string, cfg config.LoggingConfig) io.Writer {
	return &lumberjack.Logger{
		Filename:   path,
		MaxSize:    cfg.MaxSizeMB,
		MaxBackups: cfg.MaxBackups,
		Compress:   cfg.Compress,
	}
}

func roleFilename(role Role) string {
	switch role {
	case RoleServer:
		return "server.log"
	default:
		return "cli.log"
	}
}

func parseLevel(level string) *slog.LevelVar {
	lvl := new(slog.LevelVar)
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "error":
		lvl.Set(slog.LevelError)
	case "warn":
		lvl.Set(slog.LevelWarn)
	case "debug":
		lvl.Set(slog.LevelDebug)
	default:
		lvl.Set(slog.LevelInfo)
	}
	return lvl
}

type resilientWriter struct {
	primary      io.Writer
	fallback     io.Writer
	warn         io.Writer
	retries      int
	retryDelay   time.Duration
	sleep        func(time.Duration)
	fallbackPath string
	warnOnce     sync.Once
}

func (w *resilientWriter) Write(p []byte) (int, error) {
	var lastErr error

	for attempt := 1; attempt <= w.retries; attempt++ {
		n, err := w.primary.Write(p)
		if err == nil {
			return n, nil
		}

		lastErr = err
		if attempt < w.retries {
			w.sleep(w.retryDelay)
		}
	}

	w.warnOnce.Do(func() {
		fmt.Fprintf(w.warn, "warning: log file writer %q unavailable after %d attempts (%v); falling back to stderr\n", w.fallbackPath, w.retries, lastErr)
	})

	return w.fallback.Write(p)
}

func redactAttr(_ []string, attr slog.Attr) slog.Attr {
	if attr.Key == slog.TimeKey {
		return slog.String(slog.TimeKey, attr.Value.Time().UTC().Format(time.RFC3339))
	}

	if isSensitiveKey(attr.Key) {
		return slog.String(attr.Key, redactedValue)
	}

	return attr
}

func isSensitiveKey(key string) bool {
	normalized := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(key), "-", "_"))

	sensitive := []string{"token", "key", "secret", "password", "webhook", "authorization", "auth"}
	for _, marker := range sensitive {
		if normalized == marker || strings.Contains(normalized, marker) {
			return true
		}
	}

	return false
}
