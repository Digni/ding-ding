package notifier

import (
	"fmt"
	"log"
	"time"

	"github.com/Digni/ding-ding/internal/config"
	"github.com/Digni/ding-ding/internal/idle"
)

// Message represents a notification to be sent.
type Message struct {
	Title string `json:"title"`
	Body  string `json:"body"`
	Agent string `json:"agent,omitempty"` // e.g. "claude", "opencode"
}

// Notify sends a system notification and, if the user is idle, also pushes
// via configured remote backends (ntfy, Discord, webhook).
func Notify(cfg config.Config, msg Message) error {
	if msg.Title == "" {
		msg.Title = "ding ding!"
	}

	// Always send system notification
	if err := systemNotify(msg.Title, msg.Body); err != nil {
		log.Printf("system notification failed: %v", err)
	}

	// Check if user is idle
	idleTime := idle.Duration()
	threshold := time.Duration(cfg.Idle.ThresholdSeconds) * time.Second
	userIdle := threshold > 0 && idleTime >= threshold

	if !userIdle {
		log.Printf("user active (idle %s, threshold %s) — skipping push", idleTime, threshold)
		return nil
	}

	log.Printf("user idle for %s (threshold %s) — sending push notifications", idleTime, threshold)

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

// Push sends to all configured remote backends regardless of idle state.
func Push(cfg config.Config, msg Message) error {
	if msg.Title == "" {
		msg.Title = "ding ding!"
	}

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
