package server_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Digni/ding-ding/internal/config"
	"github.com/Digni/ding-ding/internal/focus"
	"github.com/Digni/ding-ding/internal/notifier"
	"github.com/Digni/ding-ding/internal/server"
)

type errorPayload struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func decodeErrorPayload(t *testing.T, resp *http.Response) errorPayload {
	t.Helper()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed reading response body: %v", err)
	}

	var payload errorPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("expected json error payload, got %q (err=%v)", string(body), err)
	}

	if payload.Code == "" || payload.Message == "" {
		t.Fatalf("expected non-empty code/message, got %+v", payload)
	}

	return payload
}

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

	payload := decodeErrorPayload(t, resp)
	if payload.Code != "missing_content" {
		t.Errorf("expected missing_content code, got %q", payload.Code)
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

	payload := decodeErrorPayload(t, resp)
	if payload.Code != "invalid_request_body" {
		t.Errorf("expected invalid_request_body code, got %q", payload.Code)
	}
}

func TestPostNotify_OversizeBody(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	// Build a JSON payload larger than 64KB
	large := strings.Repeat("x", 1<<16+1)
	requestPayload := `{"title":"` + large + `"}`

	resp, err := ts.Client().Post(
		ts.URL+"/notify",
		"application/json",
		strings.NewReader(requestPayload),
	)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusRequestEntityTooLarge {
		t.Errorf("expected 413, got %d", resp.StatusCode)
	}

	errorPayload := decodeErrorPayload(t, resp)
	if errorPayload.Code != "request_too_large" {
		t.Errorf("expected request_too_large code, got %q", errorPayload.Code)
	}
}

func TestPostNotify_DeliveryFailureStructuredJSON(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Idle.ThresholdSeconds = 1
	cfg.Ntfy.Enabled = true
	cfg.Ntfy.Server = "http://127.0.0.1:1"
	cfg.Ntfy.Topic = "test"

	origIdle := notifier.IdleDurationFunc
	origFocused := notifier.TerminalFocusedFunc
	origProcess := notifier.ProcessInFocusedTerminalFunc
	origFocusState := notifier.TerminalFocusStateFunc
	origProcessState := notifier.ProcessFocusStateFunc
	origSystem := notifier.SystemNotifyFunc
	t.Cleanup(func() {
		notifier.IdleDurationFunc = origIdle
		notifier.TerminalFocusedFunc = origFocused
		notifier.ProcessInFocusedTerminalFunc = origProcess
		notifier.TerminalFocusStateFunc = origFocusState
		notifier.ProcessFocusStateFunc = origProcessState
		notifier.SystemNotifyFunc = origSystem
	})

	notifier.IdleDurationFunc = func() (time.Duration, error) { return 10 * time.Second, nil }
	notifier.TerminalFocusedFunc = func() bool { return false }
	notifier.ProcessInFocusedTerminalFunc = func(pid int) bool { return false }
	notifier.TerminalFocusStateFunc = func() focus.State { return focus.State{Focused: false, Known: true} }
	notifier.ProcessFocusStateFunc = func(pid int) focus.State { return focus.State{Focused: false, Known: true} }
	notifier.SystemNotifyFunc = func(title, body string) error { return nil }

	ts := httptest.NewServer(server.NewMux(cfg))
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

	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", resp.StatusCode)
	}

	payload := decodeErrorPayload(t, resp)
	if payload.Code != "notification_delivery_failed" {
		t.Errorf("expected notification_delivery_failed code, got %q", payload.Code)
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
