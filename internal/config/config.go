package config

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"

	"gopkg.in/yaml.v3"
)

const EnvConfigPath = "DING_DING_CONFIG"

type SourceType string

const (
	SourceExplicitFlag SourceType = "explicit-flag"
	SourceConfigFile   SourceType = "config-file"
	SourceEnvironment  SourceType = "environment"
	SourceDefaults     SourceType = "defaults"
)

type SourceSelection struct {
	Type   SourceType
	Path   string
	Reason string
}

type LoadResult struct {
	Config Config
	Source SourceSelection
}

type LoadOptions struct {
	ExplicitPath string
	EnvPath      string
	Warn         func(string)
	Resolve      ResolveOptions
}

type Config struct {
	Ntfy         NtfyConfig         `yaml:"ntfy"`
	Discord      DiscordConfig      `yaml:"discord"`
	Webhook      WebhookConfig      `yaml:"webhook"`
	Idle         IdleConfig         `yaml:"idle"`
	Notification NotificationConfig `yaml:"notification"`
	Server       ServerConfig       `yaml:"server"`
	Sound        SoundConfig        `yaml:"sound"`
	Logging      LoggingConfig      `yaml:"logging"`
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

type LoggingConfig struct {
	Enabled    bool   `yaml:"enabled"`
	Level      string `yaml:"level"`
	Dir        string `yaml:"dir"`
	MaxSizeMB  int    `yaml:"max_size_mb"`
	MaxBackups int    `yaml:"max_backups"`
	Compress   bool   `yaml:"compress"`
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
		Logging: LoggingConfig{
			Enabled:    false,
			Level:      "info",
			Dir:        "logs",
			MaxSizeMB:  20,
			MaxBackups: 7,
			Compress:   false,
		},
	}
}

// ConfigDir returns the config directory path.
func ConfigDir() (string, error) {
	if runtime.GOOS == "darwin" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("get user home dir: %w", err)
		}
		return filepath.Join(home, ".config", "ding-ding"), nil
	}

	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("get user config dir: %w", err)
	}
	return filepath.Join(configDir, "ding-ding"), nil
}

func LegacyDarwinConfigPath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("get user config dir: %w", err)
	}
	return filepath.Join(configDir, "ding-ding", "config.yaml"), nil
}

// ConfigPath returns the full path to the config file.
func ConfigPath() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.yaml"), nil
}

// LoadFromBytes parses raw YAML config bytes, overlaying on defaults and validating.
func LoadFromBytes(data []byte) (Config, error) {
	cfg := DefaultConfig()
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

func loadFromSource(source SourceSelection) (LoadResult, error) {
	if source.Type == SourceDefaults {
		return LoadResult{Config: DefaultConfig(), Source: source}, nil
	}

	data, err := os.ReadFile(source.Path)
	if err != nil {
		return LoadResult{}, fmt.Errorf("read %s config %q: %w", source.Type, source.Path, err)
	}

	cfg, err := LoadFromBytes(data)
	if err != nil {
		return LoadResult{}, fmt.Errorf("parse %s config %q: %w", source.Type, source.Path, err)
	}

	if err := Validate(cfg); err != nil {
		return LoadResult{}, fmt.Errorf("validate %s config %q: %w", source.Type, source.Path, err)
	}

	return LoadResult{Config: cfg, Source: source}, nil
}

func warnf(warn func(string), format string, args ...any) {
	if warn == nil {
		return
	}
	warn(fmt.Sprintf(format, args...))
}

// LoadWithOptions reads config with deterministic source selection and metadata.
func LoadWithOptions(opts LoadOptions) (LoadResult, error) {
	resolveOpts := opts.Resolve
	if opts.EnvPath != "" {
		resolveOpts.EnvPath = opts.EnvPath
	}
	if resolveOpts.EnvPath == "" {
		resolveOpts.EnvPath = os.Getenv(EnvConfigPath)
	}

	if opts.ExplicitPath != "" {
		explicit := SourceSelection{
			Type:   SourceExplicitFlag,
			Path:   opts.ExplicitPath,
			Reason: "selected by --config flag",
		}

		result, err := loadFromSource(explicit)
		if err == nil {
			return result, nil
		}

		warnf(opts.Warn, "warning: unable to load explicit config path %q: %v", opts.ExplicitPath, err)
	}

	source, err := ResolveConfigSource(resolveOpts)
	if err != nil {
		return LoadResult{}, err
	}

	return loadFromSource(source)
}

// Load reads the config from disk, falling back to defaults.
func Load() (Config, error) {
	result, err := LoadWithOptions(LoadOptions{})
	if err != nil {
		return Config{}, err
	}
	return result.Config, nil
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
