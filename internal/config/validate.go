package config

import "fmt"

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

	return nil
}
