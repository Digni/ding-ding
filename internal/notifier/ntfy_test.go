package notifier

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Digni/ding-ding/internal/config"
)

// setupHTTPTest starts a test HTTP server using handler, swaps the package-level
// httpClient to the server's client (so requests are routed to it), and restores
// both on test cleanup.
func setupHTTPTest(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(handler)
	origClient := httpClient
	httpClient = srv.Client()
	t.Cleanup(func() {
		httpClient = origClient
		srv.Close()
	})
	return srv
}

func TestSendNtfy_Success(t *testing.T) {
	var gotMethod, gotPath, gotTitle, gotBody string

	srv := setupHTTPTest(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotTitle = r.Header.Get("Title")
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.WriteHeader(http.StatusOK)
	})

	cfg := config.NtfyConfig{
		Server: srv.URL,
		Topic:  "topic",
	}
	msg := Message{Title: "hello", Body: "world"}

	err := sendNtfy(cfg, msg)
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	if gotMethod != "POST" {
		t.Errorf("expected POST, got %q", gotMethod)
	}
	if gotPath != "/topic" {
		t.Errorf("expected path /topic, got %q", gotPath)
	}
	if gotTitle != msg.Title {
		t.Errorf("expected Title header %q, got %q", msg.Title, gotTitle)
	}
	if gotBody != msg.Body {
		t.Errorf("expected body %q, got %q", msg.Body, gotBody)
	}
}

func TestSendNtfy_WithPriority(t *testing.T) {
	var gotPriority string

	srv := setupHTTPTest(t, func(w http.ResponseWriter, r *http.Request) {
		gotPriority = r.Header.Get("Priority")
		w.WriteHeader(http.StatusOK)
	})

	cfg := config.NtfyConfig{
		Server:   srv.URL,
		Topic:    "topic",
		Priority: "high",
	}
	msg := Message{Title: "t", Body: "b"}

	if err := sendNtfy(cfg, msg); err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	if gotPriority != "high" {
		t.Errorf("expected Priority header %q, got %q", "high", gotPriority)
	}
}

func TestSendNtfy_WithToken(t *testing.T) {
	var gotAuth string

	srv := setupHTTPTest(t, func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	})

	cfg := config.NtfyConfig{
		Server: srv.URL,
		Topic:  "topic",
		Token:  "secret",
	}
	msg := Message{Title: "t", Body: "b"}

	if err := sendNtfy(cfg, msg); err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	if gotAuth != "Bearer secret" {
		t.Errorf("expected Authorization header %q, got %q", "Bearer secret", gotAuth)
	}
}

func TestSendNtfy_WithAgent(t *testing.T) {
	var gotTags string

	srv := setupHTTPTest(t, func(w http.ResponseWriter, r *http.Request) {
		gotTags = r.Header.Get("Tags")
		w.WriteHeader(http.StatusOK)
	})

	cfg := config.NtfyConfig{
		Server: srv.URL,
		Topic:  "topic",
	}
	msg := Message{Title: "t", Body: "b", Agent: "claude"}

	if err := sendNtfy(cfg, msg); err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	if gotTags != "claude" {
		t.Errorf("expected Tags header %q, got %q", "claude", gotTags)
	}
}

func TestSendNtfy_ServerError(t *testing.T) {
	srv := setupHTTPTest(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	cfg := config.NtfyConfig{
		Server: srv.URL,
		Topic:  "topic",
	}
	msg := Message{Title: "t", Body: "b"}

	err := sendNtfy(cfg, msg)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if want := "status 500"; !strings.Contains(err.Error(), want) {
		t.Errorf("expected error to contain %q, got %q", want, err.Error())
	}
}

func TestSendNtfy_NoOptionalHeaders(t *testing.T) {
	var gotPriority, gotAuth, gotTags string
	var priorityPresent, authPresent, tagsPresent bool

	srv := setupHTTPTest(t, func(w http.ResponseWriter, r *http.Request) {
		_, priorityPresent = r.Header["Priority"]
		_, authPresent = r.Header["Authorization"]
		_, tagsPresent = r.Header["Tags"]
		gotPriority = r.Header.Get("Priority")
		gotAuth = r.Header.Get("Authorization")
		gotTags = r.Header.Get("Tags")
		w.WriteHeader(http.StatusOK)
	})

	cfg := config.NtfyConfig{
		Server:   srv.URL,
		Topic:    "topic",
		Priority: "",
		Token:    "",
	}
	msg := Message{Title: "t", Body: "b", Agent: ""}

	if err := sendNtfy(cfg, msg); err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	if priorityPresent {
		t.Errorf("expected Priority header to be absent, got %q", gotPriority)
	}
	if authPresent {
		t.Errorf("expected Authorization header to be absent, got %q", gotAuth)
	}
	if tagsPresent {
		t.Errorf("expected Tags header to be absent, got %q", gotTags)
	}
}
