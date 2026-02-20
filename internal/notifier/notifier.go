package notifier

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/Digni/ding-ding/internal/config"
	"github.com/Digni/ding-ding/internal/focus"
	"github.com/Digni/ding-ding/internal/idle"
)

var httpClient = &http.Client{Timeout: 15 * time.Second}

// Message represents a notification to be sent.
type Message struct {
	Title string `json:"title"`
	Body  string `json:"body"`
	Agent string `json:"agent,omitempty"` // e.g. "claude", "opencode"
	PID   int    `json:"pid,omitempty"`   // caller's PID for focus detection in server mode
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

	idleTime, idleErr := idle.Duration()
	if idleErr != nil {
		log.Printf("idle detection failed: %v", idleErr)
	}
	threshold := time.Duration(cfg.Idle.ThresholdSeconds) * time.Second
	if threshold == 0 {
		log.Printf("warning: idle.threshold_seconds is 0, push notifications will never trigger based on idle state")
	}
	userIdle := idleErr == nil && threshold > 0 && idleTime >= threshold
	focused := cfg.Notification.SuppressWhenFocused && focus.TerminalFocused()

	// Tier 1: user is active and looking at the agent terminal — do nothing
	if !userIdle && focused {
		log.Printf("terminal focused, user active (idle %s) — suppressing notification", idleTime)
		return nil
	}

	// Tier 2 & 3: send system notification (user isn't looking at the terminal)
	if err := systemNotify(msg.Title, msg.Body); err != nil {
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

	idleTime, idleErr := idle.Duration()
	if idleErr != nil {
		log.Printf("idle detection failed: %v", idleErr)
	}
	threshold := time.Duration(cfg.Idle.ThresholdSeconds) * time.Second
	if threshold == 0 {
		log.Printf("warning: idle.threshold_seconds is 0, push notifications will never trigger based on idle state")
	}
	userIdle := idleErr == nil && threshold > 0 && idleTime >= threshold

	// If the caller sent a PID, we can check focus for their terminal
	focused := false
	if msg.PID > 0 && cfg.Notification.SuppressWhenFocused {
		focused = focus.ProcessInFocusedTerminal(msg.PID)
	}

	if !userIdle && focused {
		log.Printf("agent terminal focused (pid %d), user active (idle %s) — suppressing notification", msg.PID, idleTime)
		return nil
	}

	if err := systemNotify(msg.Title, msg.Body); err != nil {
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
	var errs []error

	if cfg.Ntfy.Enabled {
		if err := sendNtfy(cfg.Ntfy, msg); err != nil {
			errs = append(errs, fmt.Errorf("ntfy: %w", err))
		}
	}

	if cfg.Discord.Enabled {
		if err := sendDiscord(cfg.Discord, msg); err != nil {
			errs = append(errs, fmt.Errorf("discord: %w", err))
		}
	}

	if cfg.Webhook.Enabled {
		if err := sendWebhook(cfg.Webhook, msg); err != nil {
			errs = append(errs, fmt.Errorf("webhook: %w", err))
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	return nil
}
