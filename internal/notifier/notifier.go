package notifier

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/Digni/ding-ding/internal/config"
	"github.com/Digni/ding-ding/internal/focus"
	"github.com/Digni/ding-ding/internal/idle"
	"github.com/Digni/ding-ding/internal/logging"
)

var httpClient = &http.Client{Timeout: 15 * time.Second}

var errForcePushNoBackends = errors.New("force push requested but no push backends are enabled (ntfy, discord, webhook)")

// Test hooks — exported for cross-package test stubbing (internal/ boundary prevents public leakage).
var IdleDurationFunc = idle.Duration
var TerminalFocusedFunc = focus.TerminalFocused
var ProcessInFocusedTerminalFunc = focus.ProcessInFocusedTerminal
var TerminalFocusStateFunc = focus.TerminalFocusState
var ProcessFocusStateFunc = focus.ProcessFocusState
var SystemNotifyFunc = systemNotify
var DefaultLoggerFunc = slog.Default

// Message represents a notification to be sent.
type Message struct {
	Title       string `json:"title"`
	Body        string `json:"body"`
	Agent       string `json:"agent,omitempty"`        // e.g. "claude", "opencode"
	PID         int    `json:"pid,omitempty"`          // caller's PID for focus detection in server mode
	RequestID   string `json:"request_id,omitempty"`   // server correlation id for request-scoped tracing
	OperationID string `json:"operation_id,omitempty"` // lifecycle correlation id shared across components
}

// NotifyOptions controls forced notification behavior.
type NotifyOptions struct {
	// ForcePush sends configured push backends regardless of idle/focus state.
	ForcePush bool
	// ForceLocal sends the local/system notification even when focus suppression
	// would normally silence it.
	ForceLocal bool
}

// resolveIdleState determines whether the user is idle using the configured
// threshold. If idle detection fails, FallbackPolicy governs the result:
// "idle" treats the user as idle; anything else (including "active") treats
// the user as active. Returns (userIdle, idleTime).
func resolveIdleState(cfg config.Config, logger *slog.Logger) (userIdle bool, idleTime time.Duration) {
	threshold := time.Duration(cfg.Idle.ThresholdSeconds) * time.Second
	if threshold == 0 {
		logger.Warn("notifier.idle.threshold_zero")
		return false, 0
	}

	dur, err := IdleDurationFunc()
	if err != nil {
		switch cfg.Idle.FallbackPolicy {
		case "idle":
			logger.Warn("notifier.idle.detect_failed", "fallback_policy", "idle", "error", err)
			return true, 0
		default:
			logger.Warn("notifier.idle.detect_failed", "fallback_policy", "active", "error", err)
			return false, 0
		}
	}

	return dur >= threshold, dur
}

// Notify handles CLI invocations with 3-tier logic:
//
//	Active + terminal focused  → skip (user sees agent output directly)
//	Active + terminal unfocused → system notification only
//	Idle                        → system notification + push
func Notify(cfg config.Config, msg Message) error {
	return NotifyWithOptions(cfg, msg, NotifyOptions{})
}

// NotifyWithOptions handles CLI invocations with optional force behavior.
func NotifyWithOptions(cfg config.Config, msg Message, opts NotifyOptions) error {
	start := time.Now()
	operationID := logging.NewOperationID()
	logger := DefaultLoggerFunc().With("operation_id", operationID, "entrypoint", "cli", "agent", msg.Agent)

	msg.OperationID = operationID
	if msg.Title == "" {
		msg.Title = "ding ding!"
	}
	logger.Info("notifier.notify.started", messageMetadata(msg)...)

	userIdle, idleTime := resolveIdleState(cfg, logger)
	focused := false
	if cfg.Notification.SuppressWhenFocused {
		focusState := TerminalFocusStateFunc()
		focused = focusState.Focused || !focusState.Known
	}
	logger.Info("notifier.notify.routing", "user_idle", userIdle, "idle_ms", idleTime.Milliseconds(), "focused", focused, "force_push", opts.ForcePush, "force_local", opts.ForceLocal, "suppress_when_focused", cfg.Notification.SuppressWhenFocused)

	err := dispatchNotification(cfg, msg, userIdle, idleTime, focused, opts, logger)
	status := "ok"
	if err != nil {
		status = "error"
		logger.Error("notifier.notify.error", "status", status, "duration_ms", time.Since(start).Milliseconds(), "error", err)
	}
	logger.Info("notifier.notify.completed", "status", status, "duration_ms", time.Since(start).Milliseconds())

	return err
}

// NotifyRemote handles HTTP server invocations. If the caller provides a PID,
// focus detection uses that PID's process tree to check if the agent's
// terminal is focused. Without a PID, focus detection is skipped and a
// system notification is always sent.
func NotifyRemote(cfg config.Config, msg Message) error {
	start := time.Now()
	requestID := logging.EnsureRequestID(msg.RequestID)
	operationID := strings.TrimSpace(msg.OperationID)
	if operationID == "" {
		operationID = logging.NewOperationID()
	}
	logger := DefaultLoggerFunc().With("operation_id", operationID, "request_id", requestID, "entrypoint", "http", "agent", msg.Agent, "request_pid", msg.PID)

	msg.RequestID = requestID
	msg.OperationID = operationID

	if msg.Title == "" {
		msg.Title = "ding ding!"
	}
	logger.Info("notifier.notify.started", messageMetadata(msg)...)

	userIdle, idleTime := resolveIdleState(cfg, logger)

	// If the caller sent a PID, we can check focus for their terminal
	focused := false
	if msg.PID > 0 && cfg.Notification.SuppressWhenFocused {
		focusState := ProcessFocusStateFunc(msg.PID)
		focused = focusState.Focused || !focusState.Known
	}
	logger.Info("notifier.notify.routing", "user_idle", userIdle, "idle_ms", idleTime.Milliseconds(), "focused", focused, "force_push", false, "force_local", false, "suppress_when_focused", cfg.Notification.SuppressWhenFocused)

	err := dispatchNotification(cfg, msg, userIdle, idleTime, focused, NotifyOptions{}, logger)
	status := "ok"
	if err != nil {
		status = "error"
		logger.Error("notifier.notify.error", "status", status, "duration_ms", time.Since(start).Milliseconds(), "error", err)
	}
	logger.Info("notifier.notify.completed", "status", status, "duration_ms", time.Since(start).Milliseconds())

	return err
}

func dispatchNotification(cfg config.Config, msg Message, userIdle bool, idleTime time.Duration, focused bool, opts NotifyOptions, logger *slog.Logger) error {
	threshold := time.Duration(cfg.Idle.ThresholdSeconds) * time.Second
	var localErr error
	forcePushNoBackends := opts.ForcePush && !hasEnabledPushBackends(cfg)

	// Tier 1: user is active and looking at the agent terminal — do nothing
	if !userIdle && focused && !opts.ForceLocal {
		if !opts.ForcePush {
			logger.Info("notifier.notify.suppressed", "reason", "focused_active", "idle_ms", idleTime.Milliseconds())
			return nil
		}

		if forcePushNoBackends {
			return errForcePushNoBackends
		}

		logger.Info("notifier.notify.force_push", "reason", "focused_active", "idle_ms", idleTime.Milliseconds())
		return pushAll(cfg, msg)
	}

	shouldSendLocal := !opts.ForcePush || opts.ForceLocal

	// Tier 2 & 3: send system notification (user isn't looking at the terminal)
	if shouldSendLocal {
		if err := SystemNotifyFunc(msg.Title, msg.Body); err != nil {
			logger.Warn("notifier.notify.system_failed", "error", err)
			if opts.ForceLocal {
				localErr = fmt.Errorf("system notification: %w", err)
			}
		}
	}

	// Tier 2: user is active but on a different window — no push needed unless forced
	if !userIdle && !opts.ForcePush {
		logger.Info("notifier.notify.push_skipped", "reason", "user_active", "idle_ms", idleTime.Milliseconds(), "threshold_ms", threshold.Milliseconds())
		return localErr
	}

	if forcePushNoBackends {
		if localErr != nil {
			return errors.Join(localErr, errForcePushNoBackends)
		}
		return errForcePushNoBackends
	}

	if opts.ForcePush && !userIdle {
		logger.Info("notifier.notify.force_push", "reason", "user_active", "idle_ms", idleTime.Milliseconds(), "threshold_ms", threshold.Milliseconds())
	} else {
		// Tier 3: user is idle — send push notifications
		logger.Info("notifier.notify.push_idle", "idle_ms", idleTime.Milliseconds(), "threshold_ms", threshold.Milliseconds())
	}

	pushErr := pushAll(cfg, msg)
	if localErr != nil {
		if pushErr != nil {
			return errors.Join(localErr, pushErr)
		}
		return localErr
	}

	return pushErr
}

func messageMetadata(msg Message) []any {
	return []any{
		"title_present", msg.Title != "",
		"body_present", msg.Body != "",
		"title_bytes", len(msg.Title),
		"body_bytes", len(msg.Body),
		"message_pid", msg.PID,
		"request_id_present", strings.TrimSpace(msg.RequestID) != "",
		"operation_id_present", strings.TrimSpace(msg.OperationID) != "",
	}
}

func hasEnabledPushBackends(cfg config.Config) bool {
	return cfg.Ntfy.Enabled || cfg.Discord.Enabled || cfg.Webhook.Enabled
}

// Push sends to all configured remote backends regardless of idle/focus state.
func Push(cfg config.Config, msg Message) error {
	if msg.Title == "" {
		msg.Title = "ding ding!"
	}
	return pushAll(cfg, msg)
}

func pushAll(cfg config.Config, msg Message) error {
	type pushTarget struct {
		label string
		send  func() error
	}

	var targets []pushTarget
	if cfg.Ntfy.Enabled {
		targets = append(targets, pushTarget{
			label: "ntfy",
			send:  func() error { return sendNtfy(cfg.Ntfy, msg) },
		})
	}
	if cfg.Discord.Enabled {
		targets = append(targets, pushTarget{
			label: "discord",
			send:  func() error { return sendDiscord(cfg.Discord, msg) },
		})
	}
	if cfg.Webhook.Enabled {
		targets = append(targets, pushTarget{
			label: "webhook",
			send:  func() error { return sendWebhook(cfg.Webhook, msg) },
		})
	}

	errCh := make(chan error, len(targets))
	var wg sync.WaitGroup
	for _, target := range targets {
		target := target
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := target.send(); err != nil {
				errCh <- fmt.Errorf("%s: %w", target.label, err)
			}
		}()
	}

	wg.Wait()
	close(errCh)

	var errs []error
	for err := range errCh {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	return nil
}
