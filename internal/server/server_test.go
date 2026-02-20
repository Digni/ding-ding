package server_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Digni/ding-ding/internal/config"
	"github.com/Digni/ding-ding/internal/notifier"
	"github.com/Digni/ding-ding/internal/server"
)

func setupTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	cfg := config.DefaultConfig()

	// Stub notifier dependencies
	origIdle := notifier.IdleDurationFunc
	origFocused := notifier.TerminalFocusedFunc
	origProcess := notifier.ProcessInFocusedTerminalFunc
	origSystem := notifier.SystemNotifyFunc

	t.Cleanup(func() {
		notifier.IdleDurationFunc = origIdle
		notifier.TerminalFocusedFunc = origFocused
		notifier.ProcessInFocusedTerminalFunc = origProcess
		notifier.SystemNotifyFunc = origSystem
	})

	// User is active + unfocused → Tier 2 (system notify only, no push)
	notifier.IdleDurationFunc = func() (time.Duration, error) { return 0, nil }
	notifier.TerminalFocusedFunc = func() bool { return false }
	notifier.ProcessInFocusedTerminalFunc = func(pid int) bool { return false }
	notifier.SystemNotifyFunc = func(title, body string) error { return nil }

	mux := server.NewMux(cfg)
	return httptest.NewServer(mux)
}

// --- POST /notify ---

func TestPostNotify_ValidJSON(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	resp, err := ts.Client().Post(
		ts.URL+"/notify",
		"application/json",
		strings.NewReader(`{"title":"hello","body":"world"}`),
	)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "ok") {
		t.Errorf("expected response body to contain %q, got %q", "ok", string(body))
	}
}

func TestPostNotify_TitleOnly(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	resp, err := ts.Client().Post(
		ts.URL+"/notify",
		"application/json",
		strings.NewReader(`{"title":"hello"}`),
	)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestPostNotify_BodyOnly(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	resp, err := ts.Client().Post(
		ts.URL+"/notify",
		"application/json",
		strings.NewReader(`{"body":"world"}`),
	)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestPostNotify_EmptyTitleAndBody(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	resp, err := ts.Client().Post(
		ts.URL+"/notify",
		"application/json",
		strings.NewReader(`{}`),
	)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestPostNotify_InvalidJSON(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	resp, err := ts.Client().Post(
		ts.URL+"/notify",
		"application/json",
		strings.NewReader(`"not json"`),
	)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestPostNotify_OversizeBody(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	// Build a JSON payload larger than 64KB
	large := strings.Repeat("x", 1<<16+1)
	payload := `{"title":"` + large + `"}`

	resp, err := ts.Client().Post(
		ts.URL+"/notify",
		"application/json",
		strings.NewReader(payload),
	)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusRequestEntityTooLarge {
		t.Errorf("expected 413, got %d", resp.StatusCode)
	}
}

// --- GET /notify ---

func TestGetNotify_WithParams(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	resp, err := ts.Client().Get(ts.URL + "/notify?title=hello&message=world")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestGetNotify_NoParams(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	resp, err := ts.Client().Get(ts.URL + "/notify")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestGetNotify_TitleOnly(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	resp, err := ts.Client().Get(ts.URL + "/notify?title=hello")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

// --- GET /health ---

func TestHealth_Returns200(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	resp, err := ts.Client().Get(ts.URL + "/health")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Errorf("expected Content-Type application/json, got %q", ct)
	}

	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "ok") {
		t.Errorf("expected response body to contain %q, got %q", "ok", string(body))
	}
}

// --- Routing ---

func TestRouting_UnknownPath(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	resp, err := ts.Client().Get(ts.URL + "/unknown")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

func TestRouting_WrongMethod_Notify(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	req, err := http.NewRequest(http.MethodDelete, ts.URL+"/notify", nil)
	if err != nil {
		t.Fatalf("failed to build request: %v", err)
	}

	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", resp.StatusCode)
	}
}

func TestRouting_WrongMethod_Health(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	resp, err := ts.Client().Post(ts.URL+"/health", "application/json", strings.NewReader("{}"))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", resp.StatusCode)
	}
}
