package notifier

import (
	"fmt"
	"log"
	"time"

	"github.com/Digni/ding-ding/internal/config"
	"github.com/Digni/ding-ding/internal/focus"
	"github.com/Digni/ding-ding/internal/idle"
)

// Message represents a notification to be sent.
type Message struct {
	Title string `json:"title"`
	Body  string `json:"body"`
	Agent string `json:"agent,omitempty"` // e.g. "claude", "opencode"
}

// Notify uses 3-tier logic to decide what to send:
//
//	Active + terminal focused  → skip system notification, skip push
//	Active + terminal unfocused → send system notification, skip push
//	Idle                        → send system notification + push
func Notify(cfg config.Config, msg Message) error {
	if msg.Title == "" {
		msg.Title = "ding ding!"
	}

	idleTime := idle.Duration()
	threshold := time.Duration(cfg.Idle.ThresholdSeconds) * time.Second
	userIdle := threshold > 0 && idleTime >= threshold
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
		return fmt.Errorf("push errors: %v", errs)
	}

	return nil
}
