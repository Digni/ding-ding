package config

import (
	"strings"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	// Ntfy
	if cfg.Ntfy.Enabled != false {
		t.Errorf("Ntfy.Enabled: got %v, want false", cfg.Ntfy.Enabled)
	}
	if cfg.Ntfy.Server != "https://ntfy.sh" {
		t.Errorf("Ntfy.Server: got %q, want %q", cfg.Ntfy.Server, "https://ntfy.sh")
	}
	if cfg.Ntfy.Topic != "ding-ding" {
		t.Errorf("Ntfy.Topic: got %q, want %q", cfg.Ntfy.Topic, "ding-ding")
	}
	if cfg.Ntfy.Priority != "high" {
		t.Errorf("Ntfy.Priority: got %q, want %q", cfg.Ntfy.Priority, "high")
	}

	// Discord
	if cfg.Discord.Enabled != false {
		t.Errorf("Discord.Enabled: got %v, want false", cfg.Discord.Enabled)
	}

	// Webhook
	if cfg.Webhook.Enabled != false {
		t.Errorf("Webhook.Enabled: got %v, want false", cfg.Webhook.Enabled)
	}
	if cfg.Webhook.Method != "POST" {
		t.Errorf("Webhook.Method: got %q, want %q", cfg.Webhook.Method, "POST")
	}

	// Idle
	if cfg.Idle.ThresholdSeconds != 300 {
		t.Errorf("Idle.ThresholdSeconds: got %d, want 300", cfg.Idle.ThresholdSeconds)
	}
	if cfg.Idle.FallbackPolicy != "active" {
		t.Errorf("Idle.FallbackPolicy: got %q, want %q", cfg.Idle.FallbackPolicy, "active")
	}

	// Notification
	if cfg.Notification.SuppressWhenFocused != true {
		t.Errorf("Notification.SuppressWhenFocused: got %v, want true", cfg.Notification.SuppressWhenFocused)
	}

	// Server
	if cfg.Server.Address != "127.0.0.1:8228" {
		t.Errorf("Server.Address: got %q, want %q", cfg.Server.Address, "127.0.0.1:8228")
	}

	// Sound
	if cfg.Sound.Enabled != true {
		t.Errorf("Sound.Enabled: got %v, want true", cfg.Sound.Enabled)
	}
}

func TestLoadFromBytes_EmptyData(t *testing.T) {
	cfg, err := LoadFromBytes([]byte{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := DefaultConfig()
	if cfg != want {
		t.Errorf("got %+v, want %+v", cfg, want)
	}
}

func TestLoadFromBytes_PartialOverride(t *testing.T) {
	yaml := []byte(`
ntfy:
  enabled: true
  topic: my-topic
idle:
  threshold_seconds: 600
`)
	cfg, err := LoadFromBytes(yaml)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Overridden fields
	if !cfg.Ntfy.Enabled {
		t.Errorf("Ntfy.Enabled: got false, want true")
	}
	if cfg.Ntfy.Topic != "my-topic" {
		t.Errorf("Ntfy.Topic: got %q, want %q", cfg.Ntfy.Topic, "my-topic")
	}
	if cfg.Idle.ThresholdSeconds != 600 {
		t.Errorf("Idle.ThresholdSeconds: got %d, want 600", cfg.Idle.ThresholdSeconds)
	}

	// Non-overridden fields stay at defaults
	if cfg.Ntfy.Server != "https://ntfy.sh" {
		t.Errorf("Ntfy.Server: got %q, want default %q", cfg.Ntfy.Server, "https://ntfy.sh")
	}
	if cfg.Ntfy.Priority != "high" {
		t.Errorf("Ntfy.Priority: got %q, want default %q", cfg.Ntfy.Priority, "high")
	}
	if cfg.Webhook.Method != "POST" {
		t.Errorf("Webhook.Method: got %q, want default %q", cfg.Webhook.Method, "POST")
	}
	if cfg.Idle.FallbackPolicy != "active" {
		t.Errorf("Idle.FallbackPolicy: got %q, want default %q", cfg.Idle.FallbackPolicy, "active")
	}
	if cfg.Server.Address != "127.0.0.1:8228" {
		t.Errorf("Server.Address: got %q, want default %q", cfg.Server.Address, "127.0.0.1:8228")
	}
	if !cfg.Sound.Enabled {
		t.Errorf("Sound.Enabled: got false, want true")
	}
}

func TestLoadFromBytes_InvalidYAML(t *testing.T) {
	_, err := LoadFromBytes([]byte(":\tinvalid: yaml: {"))
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "parse config") {
		t.Errorf("error %q does not contain %q", err.Error(), "parse config")
	}
}

func TestLoadFromBytes_FallbackPolicy(t *testing.T) {
	tests := []struct {
		name       string
		policy     string
		wantPolicy string
	}{
		{
			name:       "active passes through",
			policy:     "active",
			wantPolicy: "active",
		},
		{
			name:       "idle passes through",
			policy:     "idle",
			wantPolicy: "idle",
		},
		{
			name:       "bogus corrected to active",
			policy:     "bogus",
			wantPolicy: "active",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			yaml := []byte("idle:\n  fallback_policy: " + tt.policy + "\n")
			cfg, err := LoadFromBytes(yaml)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if cfg.Idle.FallbackPolicy != tt.wantPolicy {
				t.Errorf("FallbackPolicy: got %q, want %q", cfg.Idle.FallbackPolicy, tt.wantPolicy)
			}
		})
	}
}
