package notifier

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/Digni/ding-ding/internal/config"
)

func TestSendWebhook_Success(t *testing.T) {
	var gotMethod, gotContentType string
	var gotMsg Message

	srv := setupHTTPTest(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotContentType = r.Header.Get("Content-Type")
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotMsg)
		w.WriteHeader(http.StatusOK)
	})

	cfg := config.WebhookConfig{URL: srv.URL, Method: "POST"}
	msg := Message{Title: "hello", Body: "world"}

	err := sendWebhook(cfg, msg)
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	if gotMethod != "POST" {
		t.Errorf("expected POST, got %q", gotMethod)
	}
	if gotContentType != "application/json" {
		t.Errorf("expected Content-Type application/json, got %q", gotContentType)
	}
	if gotMsg.Title != msg.Title || gotMsg.Body != msg.Body {
		t.Errorf("expected body %+v, got %+v", msg, gotMsg)
	}
}

func TestSendWebhook_CustomMethod(t *testing.T) {
	var gotMethod string

	srv := setupHTTPTest(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		w.WriteHeader(http.StatusOK)
	})

	cfg := config.WebhookConfig{URL: srv.URL, Method: "PUT"}
	msg := Message{Title: "t", Body: "b"}

	if err := sendWebhook(cfg, msg); err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	if gotMethod != "PUT" {
		t.Errorf("expected PUT, got %q", gotMethod)
	}
}

func TestSendWebhook_EmptyMethodDefaultsToPost(t *testing.T) {
	var gotMethod string

	srv := setupHTTPTest(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		w.WriteHeader(http.StatusOK)
	})

	cfg := config.WebhookConfig{URL: srv.URL, Method: ""}
	msg := Message{Title: "t", Body: "b"}

	if err := sendWebhook(cfg, msg); err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	if gotMethod != "POST" {
		t.Errorf("expected POST when method is empty, got %q", gotMethod)
	}
}

func TestSendWebhook_ServerError(t *testing.T) {
	srv := setupHTTPTest(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	cfg := config.WebhookConfig{URL: srv.URL, Method: "POST"}
	msg := Message{Title: "t", Body: "b"}

	err := sendWebhook(cfg, msg)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if want := "status 500"; !strings.Contains(err.Error(), want) {
		t.Errorf("expected error to contain %q, got %q", want, err.Error())
	}
}
