package notifier

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/Digni/ding-ding/internal/config"
)

func TestSendDiscord_Success(t *testing.T) {
	var gotMethod, gotContentType string
	var gotPayload map[string]string

	srv := setupHTTPTest(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotContentType = r.Header.Get("Content-Type")
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotPayload)
		w.WriteHeader(http.StatusOK)
	})

	cfg := config.DiscordConfig{WebhookURL: srv.URL}
	msg := Message{Title: "hello", Body: "world"}

	err := sendDiscord(cfg, msg)
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	if gotMethod != "POST" {
		t.Errorf("expected POST, got %q", gotMethod)
	}
	if gotContentType != "application/json" {
		t.Errorf("expected Content-Type application/json, got %q", gotContentType)
	}
	wantContent := "**hello**\nworld"
	if gotPayload["content"] != wantContent {
		t.Errorf("expected content %q, got %q", wantContent, gotPayload["content"])
	}
}

func TestSendDiscord_WithAgent(t *testing.T) {
	var gotPayload map[string]string

	srv := setupHTTPTest(t, func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotPayload)
		w.WriteHeader(http.StatusOK)
	})

	cfg := config.DiscordConfig{WebhookURL: srv.URL}
	msg := Message{Title: "hello", Body: "world", Agent: "claude"}

	if err := sendDiscord(cfg, msg); err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	content := gotPayload["content"]
	if !strings.Contains(content, "(claude)") {
		t.Errorf("expected content to contain %q, got %q", "(claude)", content)
	}
}

func TestSendDiscord_ServerError(t *testing.T) {
	srv := setupHTTPTest(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	cfg := config.DiscordConfig{WebhookURL: srv.URL}
	msg := Message{Title: "t", Body: "b"}

	err := sendDiscord(cfg, msg)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if want := "status 500"; !strings.Contains(err.Error(), want) {
		t.Errorf("expected error to contain %q, got %q", want, err.Error())
	}
}
