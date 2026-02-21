package config

import (
	"fmt"
	"strings"
)

// Validate enforces required values for enabled integrations.
func Validate(cfg Config) error {
	if cfg.Ntfy.Enabled {
		if cfg.Ntfy.Server == "" {
			return fmt.Errorf("ntfy.server is required when ntfy.enabled is true")
		}
		if cfg.Ntfy.Topic == "" {
			return fmt.Errorf("ntfy.topic is required when ntfy.enabled is true")
		}
	}

	if cfg.Discord.Enabled && cfg.Discord.WebhookURL == "" {
		return fmt.Errorf("discord.webhook_url is required when discord.enabled is true")
	}

	if cfg.Webhook.Enabled && cfg.Webhook.URL == "" {
		return fmt.Errorf("webhook.url is required when webhook.enabled is true")
	}

	if cfg.Server.Address == "" {
		return fmt.Errorf("server.address is required")
	}

	if err := validateLogging(cfg.Logging); err != nil {
		return err
	}

	return nil
}

func validateLogging(logging LoggingConfig) error {
	switch strings.ToLower(logging.Level) {
	case "error", "warn", "info", "debug":
		// valid
	default:
		return fmt.Errorf("logging.level must be one of error, warn, info, debug")
	}

	if logging.MaxSizeMB <= 0 {
		return fmt.Errorf("logging.max_size_mb must be greater than 0")
	}

	if logging.MaxBackups <= 0 {
		return fmt.Errorf("logging.max_backups must be greater than 0")
	}

	if strings.TrimSpace(logging.Dir) == "" {
		return fmt.Errorf("logging.dir is required")
	}

	return nil
}
