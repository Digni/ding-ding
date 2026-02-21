package server_test

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
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

func captureServerLogs(t *testing.T) (*bytes.Buffer, *slog.Logger) {
	t.Helper()
	var out bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&out, nil))
	return &out, logger
}

func parseLogRecords(t *testing.T, raw string) []map[string]any {
	t.Helper()
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil
	}

	lines := strings.Split(trimmed, "\n")
	records := make([]map[string]any, 0, len(lines))
	for _, line := range lines {
		var record map[string]any
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			t.Fatalf("invalid JSON log line %q: %v", line, err)
		}
		records = append(records, record)
	}
	return records
}

func recordByMessage(records []map[string]any, msg string) map[string]any {
	for _, record := range records {
		if value, ok := record["msg"].(string); ok && value == msg {
			return record
		}
	}
	return nil
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

func setupTestServer(t *testing.T, logger *slog.Logger) *httptest.Server {
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

	mux := server.NewMux(cfg, logger)
	return httptest.NewServer(mux)
}

// --- POST /notify ---

func TestPostNotify_ValidJSON(t *testing.T) {
	ts := setupTestServer(t, slog.Default())
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
	ts := setupTestServer(t, slog.Default())
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
	ts := setupTestServer(t, slog.Default())
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
	ts := setupTestServer(t, slog.Default())
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
	ts := setupTestServer(t, slog.Default())
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
	ts := setupTestServer(t, slog.Default())
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

	ts := httptest.NewServer(server.NewMux(cfg, slog.Default()))
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

func TestPostNotify_LogsCorrelationAndRedactsPayload(t *testing.T) {
	logs, logger := captureServerLogs(t)
	ts := setupTestServer(t, logger)
	defer ts.Close()

	secret := "raw-super-secret-token"
	body := `{"title":"hello","body":"` + secret + `","agent":"claude"}`
	req, err := http.NewRequest(http.MethodPost, ts.URL+"/notify", strings.NewReader(body))
	if err != nil {
		t.Fatalf("request build failed: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Request-Id", "req-test-123")

	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	if strings.Contains(logs.String(), secret) {
		t.Fatalf("logs should not contain raw request payload value %q", secret)
	}

	records := parseLogRecords(t, logs.String())
	completed := recordByMessage(records, "server.notify.request.completed")
	if completed == nil {
		t.Fatal("expected server.notify.request.completed record")
	}
	if completed["request_id"] != "req-test-123" {
		t.Fatalf("request_id = %v, want req-test-123", completed["request_id"])
	}
	if completed["status"] != "ok" {
		t.Fatalf("status = %v, want ok", completed["status"])
	}
	if _, ok := completed["duration_ms"].(float64); !ok {
		t.Fatalf("duration_ms should be numeric, got %T", completed["duration_ms"])
	}
}

func TestGetNotify_LogsPayloadMetadataOnly(t *testing.T) {
	logs, logger := captureServerLogs(t)
	ts := setupTestServer(t, logger)
	defer ts.Close()

	resp, err := ts.Client().Get(ts.URL + "/notify?title=hello&message=secret-message-value&agent=claude")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	if strings.Contains(logs.String(), "secret-message-value") {
		t.Fatal("expected logs to exclude raw query payload values")
	}

	records := parseLogRecords(t, logs.String())
	payload := recordByMessage(records, "server.notify.request.payload")
	if payload == nil {
		t.Fatal("expected server.notify.request.payload record")
	}
	if payload["payload_transport"] != "query" {
		t.Fatalf("payload_transport = %v, want query", payload["payload_transport"])
	}
	if payload["payload_field_count"] != float64(3) {
		t.Fatalf("payload_field_count = %v, want 3", payload["payload_field_count"])
	}
}

// --- GET /notify ---

func TestGetNotify_WithParams(t *testing.T) {
	ts := setupTestServer(t, slog.Default())
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
	ts := setupTestServer(t, slog.Default())
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
	ts := setupTestServer(t, slog.Default())
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
	ts := setupTestServer(t, slog.Default())
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
	ts := setupTestServer(t, slog.Default())
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
	ts := setupTestServer(t, slog.Default())
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
	ts := setupTestServer(t, slog.Default())
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
