package config

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Ntfy         NtfyConfig         `yaml:"ntfy"`
	Discord      DiscordConfig      `yaml:"discord"`
	Webhook      WebhookConfig      `yaml:"webhook"`
	Idle         IdleConfig         `yaml:"idle"`
	Notification NotificationConfig `yaml:"notification"`
	Server       ServerConfig       `yaml:"server"`
	Sound        SoundConfig        `yaml:"sound"`
}

type NtfyConfig struct {
	Enabled  bool   `yaml:"enabled"`
	Server   string `yaml:"server"`
	Topic    string `yaml:"topic"`
	Token    string `yaml:"token"`
	Priority string `yaml:"priority"`
}

type DiscordConfig struct {
	Enabled    bool   `yaml:"enabled"`
	WebhookURL string `yaml:"webhook_url"`
}

type WebhookConfig struct {
	Enabled bool   `yaml:"enabled"`
	URL     string `yaml:"url"`
	Method  string `yaml:"method"`
}

type IdleConfig struct {
	ThresholdSeconds int    `yaml:"threshold_seconds"`
	FallbackPolicy   string `yaml:"fallback_policy"`
}

type NotificationConfig struct {
	// SuppressWhenFocused skips the system notification when the terminal
	// that spawned ding-ding is the focused window (user is watching).
	SuppressWhenFocused bool `yaml:"suppress_when_focused"`
}

type ServerConfig struct {
	Address string `yaml:"address"`
}

type SoundConfig struct {
	Enabled bool `yaml:"enabled"`
}

func DefaultConfig() Config {
	return Config{
		Ntfy: NtfyConfig{
			Enabled:  false,
			Server:   "https://ntfy.sh",
			Topic:    "ding-ding",
			Priority: "high",
		},
		Discord: DiscordConfig{
			Enabled: false,
		},
		Webhook: WebhookConfig{
			Enabled: false,
			Method:  "POST",
		},
		Idle: IdleConfig{
			ThresholdSeconds: 300,
			FallbackPolicy:   "active",
		},
		Notification: NotificationConfig{
			SuppressWhenFocused: true,
		},
		Server: ServerConfig{
			Address: "127.0.0.1:8228",
		},
		Sound: SoundConfig{
			Enabled: true,
		},
	}
}

// ConfigDir returns the config directory path.
func ConfigDir() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("get user config dir: %w", err)
	}
	return filepath.Join(configDir, "ding-ding"), nil
}

// ConfigPath returns the full path to the config file.
func ConfigPath() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.yaml"), nil
}

// Load reads the config from disk, falling back to defaults.
func Load() (Config, error) {
	cfg := DefaultConfig()

	path, err := ConfigPath()
	if err != nil {
		return cfg, nil // return defaults if we can't determine path
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil // no config file, use defaults
		}
		return cfg, fmt.Errorf("read config: %w", err)
	}

	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("parse config: %w", err)
	}

	switch cfg.Idle.FallbackPolicy {
	case "active", "idle":
		// valid
	default:
		log.Printf("warning: unrecognized idle.fallback_policy %q, using \"active\"", cfg.Idle.FallbackPolicy)
		cfg.Idle.FallbackPolicy = "active"
	}

	return cfg, nil
}

// Init creates a default config file if one doesn't exist.
func Init() (string, error) {
	path, err := ConfigPath()
	if err != nil {
		return "", err
	}

	if _, err := os.Stat(path); err == nil {
		return "", fmt.Errorf("config already exists at %s", path)
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create config dir: %w", err)
	}

	cfg := DefaultConfig()
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return "", fmt.Errorf("marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", fmt.Errorf("write config: %w", err)
	}

	return path, nil
}
