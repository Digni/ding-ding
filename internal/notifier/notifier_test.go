package notifier

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Digni/ding-ding/internal/config"
	"github.com/Digni/ding-ding/internal/focus"
)

// stubState records what happened during a test's systemNotify calls.
type stubState struct {
	systemNotifyCalled bool
	systemNotifyCalls  int
	systemNotifyTitle  string
	systemNotifyBody   string
}

// setupStubs replaces the package-level function vars with controllable stubs
// and restores originals via t.Cleanup.
func setupStubs(t *testing.T, idleDur time.Duration, idleErr error, focused bool) *stubState {
	return setupStubsWithFocusState(t, idleDur, idleErr, focused, true)
}

func setupStubsWithFocusState(t *testing.T, idleDur time.Duration, idleErr error, focused bool, known bool) *stubState {
	t.Helper()
	state := &stubState{}

	origIdle := IdleDurationFunc
	origFocused := TerminalFocusedFunc
	origProcess := ProcessInFocusedTerminalFunc
	origFocusState := TerminalFocusStateFunc
	origProcessState := ProcessFocusStateFunc
	origSystem := SystemNotifyFunc
	origHTTP := httpClient

	t.Cleanup(func() {
		IdleDurationFunc = origIdle
		TerminalFocusedFunc = origFocused
		ProcessInFocusedTerminalFunc = origProcess
		TerminalFocusStateFunc = origFocusState
		ProcessFocusStateFunc = origProcessState
		SystemNotifyFunc = origSystem
		httpClient = origHTTP
	})

	IdleDurationFunc = func() (time.Duration, error) { return idleDur, idleErr }
	TerminalFocusedFunc = func() bool { return focused }
	ProcessInFocusedTerminalFunc = func(pid int) bool { return focused }
	TerminalFocusStateFunc = func() focus.State { return focus.State{Focused: focused, Known: known} }
	ProcessFocusStateFunc = func(pid int) focus.State { return focus.State{Focused: focused, Known: known} }
	SystemNotifyFunc = func(title, body string) error {
		state.systemNotifyCalled = true
		state.systemNotifyCalls++
		state.systemNotifyTitle = title
		state.systemNotifyBody = body
		return nil
	}

	return state
}

func testConfig() config.Config {
	cfg := config.DefaultConfig()
	cfg.Idle.ThresholdSeconds = 300
	cfg.Notification.SuppressWhenFocused = true
	return cfg
}

func captureDefaultLogger(t *testing.T) *bytes.Buffer {
	t.Helper()
	previous := DefaultLoggerFunc
	var out bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&out, nil))
	DefaultLoggerFunc = func() *slog.Logger { return logger }
	t.Cleanup(func() {
		DefaultLoggerFunc = previous
	})
	return &out
}

func decodeLogLines(t *testing.T, raw string) []map[string]any {
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
			t.Fatalf("failed to unmarshal log line %q: %v", line, err)
		}
		records = append(records, record)
	}

	return records
}

func findLogRecord(records []map[string]any, msg string) map[string]any {
	for _, record := range records {
		if got, ok := record["msg"].(string); ok && got == msg {
			return record
		}
	}
	return nil
}

// ─── resolveIdleState ────────────────────────────────────────────────────────

func TestResolveIdleState_ZeroThreshold(t *testing.T) {
	setupStubs(t, 100*time.Second, nil, false)
	cfg := testConfig()
	cfg.Idle.ThresholdSeconds = 0

	idle, dur := resolveIdleState(cfg, slog.Default())
	if idle {
		t.Error("expected userIdle=false for zero threshold")
	}
	if dur != 0 {
		t.Errorf("expected idleTime=0, got %s", dur)
	}
}

func TestResolveIdleState_BelowThreshold(t *testing.T) {
	setupStubs(t, 100*time.Second, nil, false)
	cfg := testConfig() // threshold=300s

	idle, dur := resolveIdleState(cfg, slog.Default())
	if idle {
		t.Error("expected userIdle=false when below threshold")
	}
	if dur != 100*time.Second {
		t.Errorf("expected idleTime=100s, got %s", dur)
	}
}

func TestResolveIdleState_AtThreshold(t *testing.T) {
	setupStubs(t, 300*time.Second, nil, false)
	cfg := testConfig() // threshold=300s

	idle, dur := resolveIdleState(cfg, slog.Default())
	if !idle {
		t.Error("expected userIdle=true when at threshold (inclusive)")
	}
	if dur != 300*time.Second {
		t.Errorf("expected idleTime=300s, got %s", dur)
	}
}

func TestResolveIdleState_AboveThreshold(t *testing.T) {
	setupStubs(t, 600*time.Second, nil, false)
	cfg := testConfig() // threshold=300s

	idle, dur := resolveIdleState(cfg, slog.Default())
	if !idle {
		t.Error("expected userIdle=true when above threshold")
	}
	if dur != 600*time.Second {
		t.Errorf("expected idleTime=600s, got %s", dur)
	}
}

func TestResolveIdleState_ErrorFallbackActive(t *testing.T) {
	setupStubs(t, 0, errors.New("detection failed"), false)
	cfg := testConfig()
	cfg.Idle.FallbackPolicy = "active"

	idle, dur := resolveIdleState(cfg, slog.Default())
	if idle {
		t.Error("expected userIdle=false for fallback=active")
	}
	if dur != 0 {
		t.Errorf("expected idleTime=0, got %s", dur)
	}
}

func TestResolveIdleState_ErrorFallbackIdle(t *testing.T) {
	setupStubs(t, 0, errors.New("detection failed"), false)
	cfg := testConfig()
	cfg.Idle.FallbackPolicy = "idle"

	idle, dur := resolveIdleState(cfg, slog.Default())
	if !idle {
		t.Error("expected userIdle=true for fallback=idle")
	}
	if dur != 0 {
		t.Errorf("expected idleTime=0, got %s", dur)
	}
}

// ─── Notify (3-tier dispatch) ────────────────────────────────────────────────

func TestNotify_Tier1_ActiveAndFocused(t *testing.T) {
	state := setupStubs(t, 10*time.Second, nil, true)
	cfg := testConfig() // SuppressWhenFocused=true, threshold=300s

	err := Notify(cfg, Message{Title: "test", Body: "body"})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if state.systemNotifyCalled {
		t.Error("expected systemNotify NOT called for Tier 1 (active + focused)")
	}
}

func TestNotify_Tier2_ActiveAndUnfocused(t *testing.T) {
	state := setupStubs(t, 10*time.Second, nil, false)
	cfg := testConfig()

	err := Notify(cfg, Message{Title: "test", Body: "body"})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if !state.systemNotifyCalled {
		t.Error("expected systemNotify called for Tier 2 (active + unfocused)")
	}
}

func TestNotify_Tier3_Idle(t *testing.T) {
	state := setupStubs(t, 600*time.Second, nil, false)
	cfg := testConfig()
	// No backends enabled, so pushAll returns nil.

	err := Notify(cfg, Message{Title: "test", Body: "body"})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if !state.systemNotifyCalled {
		t.Error("expected systemNotify called for Tier 3")
	}
}

func TestNotify_Tier3_IdleOverridesFocus(t *testing.T) {
	state := setupStubs(t, 600*time.Second, nil, true)
	cfg := testConfig()

	err := Notify(cfg, Message{Title: "test", Body: "body"})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if !state.systemNotifyCalled {
		t.Error("expected systemNotify called even when focused but idle")
	}
}

func TestNotify_SuppressFocusDisabled(t *testing.T) {
	// Even though focused=true, SuppressWhenFocused=false means focused evaluates false.
	state := setupStubs(t, 10*time.Second, nil, true)
	cfg := testConfig()
	cfg.Notification.SuppressWhenFocused = false

	err := Notify(cfg, Message{Title: "test", Body: "body"})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	// Should reach Tier 2 (active + focus=false → system notify called)
	if !state.systemNotifyCalled {
		t.Error("expected systemNotify called when SuppressWhenFocused=false")
	}
}

func TestNotify_ActiveUncertainFocus_TreatedAsFocused(t *testing.T) {
	state := setupStubsWithFocusState(t, 10*time.Second, nil, false, false)
	cfg := testConfig()

	err := Notify(cfg, Message{Title: "test", Body: "body"})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if state.systemNotifyCalled {
		t.Error("expected systemNotify NOT called when focus is uncertain")
	}
}

func TestNotify_LogsCorrelationStatusAndDuration(t *testing.T) {
	state := setupStubs(t, 10*time.Second, nil, true)
	logOut := captureDefaultLogger(t)
	cfg := testConfig()

	err := Notify(cfg, Message{Title: "test", Body: "body", Agent: "claude"})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if state.systemNotifyCalled {
		t.Fatal("expected focused active path to remain suppressed")
	}

	records := decodeLogLines(t, logOut.String())
	if len(records) == 0 {
		t.Fatal("expected structured notifier logs")
	}

	start := findLogRecord(records, "notifier.notify.started")
	if start == nil {
		t.Fatal("expected notifier.notify.started record")
	}
	if _, ok := start["operation_id"].(string); !ok {
		t.Fatalf("expected operation_id in started record, got %v", start["operation_id"])
	}

	complete := findLogRecord(records, "notifier.notify.completed")
	if complete == nil {
		t.Fatal("expected notifier.notify.completed record")
	}
	if complete["status"] != "ok" {
		t.Fatalf("expected completion status ok, got %v", complete["status"])
	}
	if _, ok := complete["duration_ms"].(float64); !ok {
		t.Fatalf("expected numeric duration_ms, got %T", complete["duration_ms"])
	}
}

func TestNotifyWithOptions_ForcePush_DoesNotDoubleSendWhenIdle(t *testing.T) {
	state := setupStubs(t, 600*time.Second, nil, false)

	pushCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pushCount++
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := testConfig()
	cfg.Ntfy.Enabled = true
	cfg.Ntfy.Server = srv.URL
	cfg.Ntfy.Topic = "test"
	httpClient = srv.Client()

	err := NotifyWithOptions(cfg, Message{Title: "test", Body: "body"}, NotifyOptions{ForcePush: true})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if pushCount != 1 {
		t.Fatalf("expected one push call, got %d", pushCount)
	}
	if state.systemNotifyCalled {
		t.Fatal("expected ForcePush to remain remote-only during idle")
	}
}

func TestNotifyWithOptions_ForcePush_ActiveFocusedSkipsLocal(t *testing.T) {
	state := setupStubs(t, 10*time.Second, nil, true)

	pushCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pushCount++
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := testConfig()
	cfg.Ntfy.Enabled = true
	cfg.Ntfy.Server = srv.URL
	cfg.Ntfy.Topic = "test"
	httpClient = srv.Client()

	err := NotifyWithOptions(cfg, Message{Title: "test", Body: "body"}, NotifyOptions{ForcePush: true})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if state.systemNotifyCalled {
		t.Fatal("expected system notification to stay suppressed without ForceLocal")
	}
	if state.systemNotifyCalls != 0 {
		t.Fatalf("expected zero system notifications, got %d", state.systemNotifyCalls)
	}
	if pushCount != 1 {
		t.Fatalf("expected one push call, got %d", pushCount)
	}
}

func TestNotifyWithOptions_ForcePush_ActiveUnfocusedSendsPushOnly(t *testing.T) {
	state := setupStubs(t, 10*time.Second, nil, false)

	pushCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pushCount++
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := testConfig()
	cfg.Ntfy.Enabled = true
	cfg.Ntfy.Server = srv.URL
	cfg.Ntfy.Topic = "test"
	httpClient = srv.Client()

	err := NotifyWithOptions(cfg, Message{Title: "test", Body: "body"}, NotifyOptions{ForcePush: true})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if state.systemNotifyCalled {
		t.Fatal("expected ForcePush to be remote-only for active + unfocused")
	}
	if state.systemNotifyCalls != 0 {
		t.Fatalf("expected zero system notifications, got %d", state.systemNotifyCalls)
	}
	if pushCount != 1 {
		t.Fatalf("expected one push call, got %d", pushCount)
	}
}

func TestNotifyWithOptions_ForcePush_NoBackendsEnabledReturnsError(t *testing.T) {
	state := setupStubs(t, 10*time.Second, nil, false)
	cfg := testConfig()

	err := NotifyWithOptions(cfg, Message{Title: "test", Body: "body"}, NotifyOptions{ForcePush: true})
	if err == nil {
		t.Fatal("expected error when ForcePush is set without enabled push backends")
	}
	if !strings.Contains(err.Error(), "force push requested but no push backends are enabled") {
		t.Fatalf("expected clear no-backends ForcePush error, got %q", err.Error())
	}
	if state.systemNotifyCalled {
		t.Fatal("expected no local/system notification for ForcePush-only path")
	}
}

func TestNotifyWithOptions_ForcePushAndForceLocal_NoBackendsEnabledReturnsJoinedError(t *testing.T) {
	setupStubs(t, 10*time.Second, nil, false)

	SystemNotifyFunc = func(title, body string) error {
		return errors.New("system notify failed")
	}

	cfg := testConfig()
	err := NotifyWithOptions(cfg, Message{Title: "test", Body: "body"}, NotifyOptions{ForcePush: true, ForceLocal: true})
	if err == nil {
		t.Fatal("expected error when ForcePush is set without enabled push backends")
	}
	if !strings.Contains(err.Error(), "system notification") {
		t.Fatalf("expected joined error to include system notification, got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "force push requested but no push backends are enabled") {
		t.Fatalf("expected joined error to include no-backends message, got %q", err.Error())
	}
}

func TestNotifyWithOptions_ForceLocal_ActiveFocusedSendsSystemOnly(t *testing.T) {
	state := setupStubs(t, 10*time.Second, nil, true)
	cfg := testConfig()

	err := NotifyWithOptions(cfg, Message{Title: "test", Body: "body"}, NotifyOptions{ForceLocal: true})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if !state.systemNotifyCalled {
		t.Fatal("expected forced local/system notification")
	}
	if state.systemNotifyCalls != 1 {
		t.Fatalf("expected one system notification, got %d", state.systemNotifyCalls)
	}
}

func TestNotifyWithOptions_ForceLocal_SystemNotifyErrorPropagated(t *testing.T) {
	setupStubs(t, 10*time.Second, nil, true)

	SystemNotifyFunc = func(title, body string) error {
		return errors.New("system notify failed")
	}

	cfg := testConfig()
	err := NotifyWithOptions(cfg, Message{Title: "test", Body: "body"}, NotifyOptions{ForceLocal: true})
	if err == nil {
		t.Fatal("expected error when ForceLocal is set and system notify fails")
	}
	if !strings.Contains(err.Error(), "system notification") {
		t.Fatalf("expected wrapped system notification error, got %q", err.Error())
	}
}

func TestNotifyWithOptions_ForcePushAndForceLocal_ActiveFocusedSendsBoth(t *testing.T) {
	state := setupStubs(t, 10*time.Second, nil, true)

	pushCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pushCount++
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := testConfig()
	cfg.Ntfy.Enabled = true
	cfg.Ntfy.Server = srv.URL
	cfg.Ntfy.Topic = "test"
	httpClient = srv.Client()

	err := NotifyWithOptions(cfg, Message{Title: "test", Body: "body"}, NotifyOptions{ForcePush: true, ForceLocal: true})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if !state.systemNotifyCalled {
		t.Fatal("expected forced local/system notification")
	}
	if state.systemNotifyCalls != 1 {
		t.Fatalf("expected one system notification, got %d", state.systemNotifyCalls)
	}
	if pushCount != 1 {
		t.Fatalf("expected one push call, got %d", pushCount)
	}
}

func TestNotify_DefaultTitle(t *testing.T) {
	state := setupStubs(t, 10*time.Second, nil, false)
	cfg := testConfig()

	err := Notify(cfg, Message{Title: "", Body: "body"})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if state.systemNotifyTitle != "ding ding!" {
		t.Errorf("expected default title 'ding ding!', got %q", state.systemNotifyTitle)
	}
}

func TestNotify_SystemNotifyErrorDoesNotBlock(t *testing.T) {
	setupStubs(t, 600*time.Second, nil, false)

	SystemNotifyFunc = func(title, body string) error {
		return errors.New("system notify failed")
	}

	pushCalled := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pushCalled = true
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	cfg := testConfig()
	cfg.Ntfy.Enabled = true
	cfg.Ntfy.Server = srv.URL
	cfg.Ntfy.Topic = "test"
	httpClient = srv.Client()

	err := Notify(cfg, Message{Title: "test", Body: "body"})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if !pushCalled {
		t.Error("expected pushAll to be called even when systemNotify returns error")
	}
}

func TestNotify_PushError_Propagated(t *testing.T) {
	setupStubs(t, 600*time.Second, nil, false)
	logOut := captureDefaultLogger(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	cfg := testConfig()
	cfg.Ntfy.Enabled = true
	cfg.Ntfy.Server = srv.URL
	cfg.Ntfy.Topic = "test"
	httpClient = srv.Client()

	err := Notify(cfg, Message{Title: "test", Body: "body"})
	if err == nil {
		t.Fatal("expected error when ntfy returns 500")
	}
	if !strings.Contains(err.Error(), "ntfy") {
		t.Errorf("expected error to contain 'ntfy', got %q", err.Error())
	}

	records := decodeLogLines(t, logOut.String())
	errorRecord := findLogRecord(records, "notifier.notify.error")
	if errorRecord == nil {
		t.Fatal("expected notifier.notify.error record")
	}
	if errorRecord["status"] != "error" {
		t.Fatalf("expected error status, got %v", errorRecord["status"])
	}
	if _, ok := errorRecord["duration_ms"].(float64); !ok {
		t.Fatalf("expected numeric duration_ms in error record, got %T", errorRecord["duration_ms"])
	}
}

// ─── NotifyRemote ────────────────────────────────────────────────────────────

func TestNotifyRemote_Tier1_ActiveFocusedWithPID(t *testing.T) {
	state := setupStubs(t, 10*time.Second, nil, true)
	cfg := testConfig()

	err := NotifyRemote(cfg, Message{Title: "test", Body: "body", PID: 1234})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if state.systemNotifyCalled {
		t.Error("expected systemNotify NOT called for Tier 1 remote (active + focused PID)")
	}
}

func TestNotifyRemote_Tier2_NoPID(t *testing.T) {
	// PID=0 → focus check skipped → not focused → systemNotify called
	state := setupStubs(t, 10*time.Second, nil, true)
	cfg := testConfig()

	err := NotifyRemote(cfg, Message{Title: "test", Body: "body", PID: 0})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if !state.systemNotifyCalled {
		t.Error("expected systemNotify called when PID=0 (focus check skipped)")
	}
}

func TestNotifyRemote_Tier3_Idle(t *testing.T) {
	state := setupStubs(t, 600*time.Second, nil, false)
	cfg := testConfig()

	err := NotifyRemote(cfg, Message{Title: "test", Body: "body", PID: 1234})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if !state.systemNotifyCalled {
		t.Error("expected systemNotify called for Tier 3 remote")
	}
}

func TestNotifyRemote_SuppressFocusDisabled(t *testing.T) {
	// SuppressWhenFocused=false → focus is never checked → not focused → Tier 2
	state := setupStubs(t, 10*time.Second, nil, true)
	cfg := testConfig()
	cfg.Notification.SuppressWhenFocused = false

	err := NotifyRemote(cfg, Message{Title: "test", Body: "body", PID: 1234})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if !state.systemNotifyCalled {
		t.Error("expected systemNotify called when SuppressWhenFocused=false")
	}
}

func TestNotifyRemote_ActiveUncertainFocusWithPID_TreatedAsFocused(t *testing.T) {
	state := setupStubsWithFocusState(t, 10*time.Second, nil, false, false)
	cfg := testConfig()

	err := NotifyRemote(cfg, Message{Title: "test", Body: "body", PID: 1234})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if state.systemNotifyCalled {
		t.Error("expected systemNotify NOT called when remote focus is uncertain")
	}
}

func TestNotifyRemote_DefaultTitle(t *testing.T) {
	state := setupStubs(t, 10*time.Second, nil, false)
	cfg := testConfig()

	err := NotifyRemote(cfg, Message{Title: "", Body: "body", PID: 1234})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if state.systemNotifyTitle != "ding ding!" {
		t.Errorf("expected default title 'ding ding!', got %q", state.systemNotifyTitle)
	}
}

func TestNotifyRemote_NoPID_DoesNotCallProcessFocusCheck(t *testing.T) {
	state := setupStubs(t, 10*time.Second, nil, true)
	cfg := testConfig()

	tests := []struct {
		name string
		pid  int
	}{
		{name: "zero pid", pid: 0},
		{name: "negative pid", pid: -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			processFocusCalled := false
			ProcessFocusStateFunc = func(pid int) focus.State {
				processFocusCalled = true
				return focus.State{Focused: true, Known: true}
			}

			state.systemNotifyCalled = false

			err := NotifyRemote(cfg, Message{Title: "test", Body: "body", PID: tt.pid})
			if err != nil {
				t.Fatalf("expected nil error, got %v", err)
			}
			if processFocusCalled {
				t.Fatalf("expected ProcessInFocusedTerminalFunc not called for PID=%d", tt.pid)
			}
			if !state.systemNotifyCalled {
				t.Fatalf("expected systemNotify called for PID=%d", tt.pid)
			}
		})
	}
}

// ─── Push ────────────────────────────────────────────────────────────────────

func TestPush_BypassesIdleFocus(t *testing.T) {
	// Push should call pushAll regardless of idle/focus state.
	// Stubs: active + focused — Push must still reach pushAll.
	setupStubs(t, 10*time.Second, nil, true)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := testConfig()
	cfg.Ntfy.Enabled = true
	cfg.Ntfy.Server = srv.URL
	cfg.Ntfy.Topic = "test"
	httpClient = srv.Client()

	err := Push(cfg, Message{Title: "test", Body: "body"})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func TestPush_DefaultTitle(t *testing.T) {
	setupStubs(t, 10*time.Second, nil, false)

	gotTitle := ""
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotTitle = r.Header.Get("Title")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := testConfig()
	cfg.Ntfy.Enabled = true
	cfg.Ntfy.Server = srv.URL
	cfg.Ntfy.Topic = "test"
	httpClient = srv.Client()

	err := Push(cfg, Message{Title: "", Body: "body"})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if gotTitle != "ding ding!" {
		t.Errorf("expected ntfy Title header 'ding ding!', got %q", gotTitle)
	}
}

// ─── pushAll ─────────────────────────────────────────────────────────────────

func TestPushAll_NoBackends(t *testing.T) {
	setupStubs(t, 0, nil, false)
	cfg := testConfig()
	// All backends disabled by default.

	err := pushAll(cfg, Message{Title: "test", Body: "body"})
	if err != nil {
		t.Fatalf("expected nil when no backends enabled, got %v", err)
	}
}

func TestPushAll_SingleBackendSuccess(t *testing.T) {
	setupStubs(t, 0, nil, false)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := testConfig()
	cfg.Ntfy.Enabled = true
	cfg.Ntfy.Server = srv.URL
	cfg.Ntfy.Topic = "test"
	httpClient = srv.Client()

	err := pushAll(cfg, Message{Title: "test", Body: "body"})
	if err != nil {
		t.Fatalf("expected nil for successful ntfy, got %v", err)
	}
}

func TestPushAll_EnabledBackendsRunConcurrently(t *testing.T) {
	setupStubs(t, 0, nil, false)

	started := make(chan string, 2)
	release := make(chan struct{})
	var releaseOnce sync.Once
	releaseBackends := func() {
		releaseOnce.Do(func() {
			close(release)
		})
	}
	t.Cleanup(releaseBackends)

	ntfySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started <- "ntfy"
		<-release
		w.WriteHeader(http.StatusOK)
	}))
	defer ntfySrv.Close()

	discordSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started <- "discord"
		<-release
		w.WriteHeader(http.StatusOK)
	}))
	defer discordSrv.Close()

	cfg := testConfig()
	cfg.Ntfy.Enabled = true
	cfg.Ntfy.Server = ntfySrv.URL
	cfg.Ntfy.Topic = "test"
	cfg.Discord.Enabled = true
	cfg.Discord.WebhookURL = discordSrv.URL
	httpClient = &http.Client{Timeout: time.Second}

	done := make(chan error, 1)
	go func() {
		done <- pushAll(cfg, Message{Title: "test", Body: "body"})
	}()

	seen := map[string]bool{}
	for len(seen) < 2 {
		select {
		case backend := <-started:
			seen[backend] = true
		case <-time.After(2 * time.Second):
			t.Fatalf("expected both backends to start before release; saw %v", seen)
		}
	}

	releaseBackends()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("pushAll did not complete after releasing backends")
	}
}

func TestPushAll_PartialFailure(t *testing.T) {
	setupStubs(t, 0, nil, false)

	// ntfy returns 500; discord returns 200.
	ntfyFailed := false
	discordSucceeded := false

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// ntfy uses POST with plain body; discord uses POST with JSON.
		// Distinguish by Content-Type header.
		ct := r.Header.Get("Content-Type")
		if ct == "application/json" {
			// Discord or webhook request.
			discordSucceeded = true
			w.WriteHeader(http.StatusOK)
		} else {
			// ntfy request.
			ntfyFailed = true
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer srv.Close()

	cfg := testConfig()
	cfg.Ntfy.Enabled = true
	cfg.Ntfy.Server = srv.URL
	cfg.Ntfy.Topic = "test"
	cfg.Discord.Enabled = true
	cfg.Discord.WebhookURL = fmt.Sprintf("%s/discord", srv.URL)
	httpClient = srv.Client()

	err := pushAll(cfg, Message{Title: "test", Body: "body"})
	if err == nil {
		t.Fatal("expected error when ntfy fails")
	}
	if !strings.Contains(err.Error(), "ntfy") {
		t.Errorf("expected error to contain 'ntfy', got %q", err.Error())
	}
	if strings.Contains(err.Error(), "discord") {
		t.Errorf("expected error NOT to contain 'discord', got %q", err.Error())
	}
	if !ntfyFailed {
		t.Error("ntfy handler was not called")
	}
	if !discordSucceeded {
		t.Error("discord handler was not called")
	}
}

func TestPushAll_AllFail(t *testing.T) {
	setupStubs(t, 0, nil, false)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	cfg := testConfig()
	cfg.Ntfy.Enabled = true
	cfg.Ntfy.Server = srv.URL
	cfg.Ntfy.Topic = "test"
	cfg.Discord.Enabled = true
	cfg.Discord.WebhookURL = fmt.Sprintf("%s/discord", srv.URL)
	cfg.Webhook.Enabled = true
	cfg.Webhook.URL = fmt.Sprintf("%s/webhook", srv.URL)
	httpClient = srv.Client()

	err := pushAll(cfg, Message{Title: "test", Body: "body"})
	if err == nil {
		t.Fatal("expected error when all backends fail")
	}
	for _, name := range []string{"ntfy", "discord", "webhook"} {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("expected error to contain %q, got %q", name, err.Error())
		}
	}
}
