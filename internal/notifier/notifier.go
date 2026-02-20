package notifier

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/Digni/ding-ding/internal/config"
	"github.com/Digni/ding-ding/internal/focus"
	"github.com/Digni/ding-ding/internal/idle"
)

var httpClient = &http.Client{Timeout: 15 * time.Second}

// Test hooks — exported for cross-package test stubbing (internal/ boundary prevents public leakage).
var IdleDurationFunc = idle.Duration
var TerminalFocusedFunc = focus.TerminalFocused
var ProcessInFocusedTerminalFunc = focus.ProcessInFocusedTerminal
var SystemNotifyFunc = systemNotify

// Message represents a notification to be sent.
type Message struct {
	Title string `json:"title"`
	Body  string `json:"body"`
	Agent string `json:"agent,omitempty"` // e.g. "claude", "opencode"
	PID   int    `json:"pid,omitempty"`   // caller's PID for focus detection in server mode
}

// resolveIdleState determines whether the user is idle using the configured
// threshold. If idle detection fails, FallbackPolicy governs the result:
// "idle" treats the user as idle; anything else (including "active") treats
// the user as active. Returns (userIdle, idleTime).
func resolveIdleState(cfg config.Config) (userIdle bool, idleTime time.Duration) {
	threshold := time.Duration(cfg.Idle.ThresholdSeconds) * time.Second
	if threshold == 0 {
		log.Printf("warning: idle.threshold_seconds is 0, push notifications will never trigger based on idle state")
		return false, 0
	}

	dur, err := IdleDurationFunc()
	if err != nil {
		switch cfg.Idle.FallbackPolicy {
		case "idle":
			log.Printf("idle detection failed (%v), fallback_policy=idle — treating as idle", err)
			return true, 0
		default:
			log.Printf("idle detection failed (%v), fallback_policy=active — treating as active", err)
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
	if msg.Title == "" {
		msg.Title = "ding ding!"
	}

	userIdle, idleTime := resolveIdleState(cfg)
	threshold := time.Duration(cfg.Idle.ThresholdSeconds) * time.Second
	focused := cfg.Notification.SuppressWhenFocused && TerminalFocusedFunc()

	// Tier 1: user is active and looking at the agent terminal — do nothing
	if !userIdle && focused {
		log.Printf("terminal focused, user active (idle %s) — suppressing notification", idleTime)
		return nil
	}

	// Tier 2 & 3: send system notification (user isn't looking at the terminal)
	if err := SystemNotifyFunc(msg.Title, msg.Body); err != nil {
		log.Printf("system notification failed: %v", err)
	}

	// Tier 2: user is active but on a different window — no push needed
	if !userIdle {
		log.Printf("user active (idle %s, threshold %s) — skipping push", idleTime, threshold)
		return nil
	}

	// Tier 3: user is idle — send push notifications
	log.Printf("user idle for %s (threshold %s) — sending push notifications", idleTime, threshold)
	return pushAll(cfg, msg)
}

// NotifyRemote handles HTTP server invocations. If the caller provides a PID,
// focus detection uses that PID's process tree to check if the agent's
// terminal is focused. Without a PID, focus detection is skipped and a
// system notification is always sent.
func NotifyRemote(cfg config.Config, msg Message) error {
	if msg.Title == "" {
		msg.Title = "ding ding!"
	}

	userIdle, idleTime := resolveIdleState(cfg)
	threshold := time.Duration(cfg.Idle.ThresholdSeconds) * time.Second

	// If the caller sent a PID, we can check focus for their terminal
	focused := false
	if msg.PID > 0 && cfg.Notification.SuppressWhenFocused {
		focused = ProcessInFocusedTerminalFunc(msg.PID)
	}

	if !userIdle && focused {
		log.Printf("agent terminal focused (pid %d), user active (idle %s) — suppressing notification", msg.PID, idleTime)
		return nil
	}

	if err := SystemNotifyFunc(msg.Title, msg.Body); err != nil {
		log.Printf("system notification failed: %v", err)
	}

	if !userIdle {
		log.Printf("user active (idle %s, threshold %s) — skipping push", idleTime, threshold)
		return nil
	}

	log.Printf("user idle for %s (threshold %s) — sending push notifications", idleTime, threshold)
	return pushAll(cfg, msg)
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
