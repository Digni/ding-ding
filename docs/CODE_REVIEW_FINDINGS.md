# Code Review Findings

Multi-agent code review performed 2026-02-20. All findings below are tracked for resolution.

## Legend

- **Status**: `[ ]` todo, `[x]` done
- **Effort**: `easy` (< 30 min), `medium` (30 min–2 hrs), `hard` (> 2 hrs / design needed)

---

## Critical

### C1: Windows PowerShell injection
- **File:** `internal/notifier/system.go:18-26`
- **Effort:** easy
- **Status:** `[x]`
- **Issue:** `title` and `body` interpolated via `%s` into PowerShell script. Remotely exploitable via GET `/notify`.
- **Fix:** XML-escape title and body before interpolation, or pass as separate PowerShell arguments.

### C2: macOS AppleScript injection
- **File:** `internal/notifier/system.go:14-15`
- **Effort:** easy
- **Status:** `[x]`
- **Issue:** Go's `%q` is not valid AppleScript escaping. A body containing `"` breaks out of the AppleScript string.
- **Fix:** Strip/replace double quotes, or pass values via `osascript` argv instead of embedding in script.

### C3: HTTP server binds to all interfaces
- **File:** `internal/config/config.go:80`
- **Effort:** easy
- **Status:** `[x]`
- **Issue:** Default address `:8228` exposes server to network. Zero authentication.
- **Fix:** Default to `127.0.0.1:8228`.

### C4: No HTTP server timeouts
- **File:** `internal/server/server.go:72`
- **Effort:** easy
- **Status:** `[x]`
- **Issue:** `http.ListenAndServe` with no read/write/idle timeouts. Slow clients exhaust goroutines.
- **Fix:** Use `&http.Server{}` with explicit timeouts.

### C5: No request body size limit
- **File:** `internal/server/server.go:19`
- **Effort:** easy
- **Status:** `[x]`
- **Issue:** Unbounded `json.NewDecoder(r.Body)`. A 1GB POST exhausts memory.
- **Fix:** `r.Body = http.MaxBytesReader(w, r.Body, 1<<16)` before decoding.

### C6: No HTTP client timeouts
- **File:** `internal/notifier/ntfy.go:33`, `discord.go:25`, `webhook.go:29`
- **Effort:** easy
- **Status:** `[x]`
- **Issue:** All outbound senders use `http.DefaultClient` (no timeout). Hung backend blocks goroutine forever. `pushAll` calls serially so latency compounds.
- **Fix:** Define shared `&http.Client{Timeout: 15 * time.Second}` in notifier package.

---

## Warnings

### W1: `--version` always prints "dev"
- **File:** `cmd/root.go:15`
- **Effort:** easy
- **Status:** `[x]`
- **Issue:** `rootCmd` struct literal captures `Version` at package init time (always `"dev"`).
- **Fix:** Set `rootCmd.Version = Version` inside `Execute()` before `rootCmd.Execute()`.

### W2: `isAncestor` duplicated across 3 platform files
- **File:** `focus_darwin.go`, `focus_linux.go`, `focus_windows.go`
- **Effort:** easy
- **Status:** `[x]`
- **Issue:** Identical function in all three files. No max-depth guard.
- **Fix:** Extract to `focus_common.go`. Add `const maxDepth = 32` guard.

### W3: Silent idle detection failures
- **File:** `idle_darwin.go`, `idle_linux.go`
- **Effort:** medium
- **Status:** `[x]`
- **Issue:** If `bash`/`ioreg`/`xprintidle`/`dbus-send` fails, returns 0 (user appears always active). Push notifications silently suppressed. Headless servers never send pushes.
- **Fix:** `idle.Duration()` now returns `(time.Duration, error)`. Added `idle.fallback_policy` config (`"active"`/`"idle"`). `resolveIdleState()` helper in notifier uses fallback policy when detection fails.

### W4: GNOME Wayland focus detection fails
- **File:** `internal/focus/focus_linux.go`
- **Effort:** hard
- **Status:** `[ ]`
- **Issue:** Neither `xdotool` nor `kdotool` works on GNOME Wayland (most common modern Linux desktop). Focus detection silently returns false.
- **Fix:** Research Wayland-native focus detection (e.g., `wlrctl`, compositor-specific DBus APIs). May need a fallback strategy.

### W5: `threshold_seconds: 0` silently disables idle detection
- **File:** `internal/notifier/notifier.go:33`
- **Effort:** easy
- **Status:** `[x]`
- **Issue:** `threshold > 0 && idleTime >= threshold` means 0 = never push (not "always push").
- **Fix:** Validate at config load time. Log warning or treat 0 as "always idle".

### W6: stdin read truncation
- **File:** `cmd/notify.go:53-58`
- **Effort:** easy
- **Status:** `[x]`
- **Issue:** `os.Stdin.Read(buf)` reads at most 4096 bytes, silently truncates. Error discarded.
- **Fix:** Use `io.ReadAll` with a size limit, surface truncation warning.

### W7: Error details leaked in HTTP responses
- **File:** `internal/server/server.go:19`, `server.go:31`
- **Effort:** easy
- **Status:** `[x]`
- **Issue:** JSON decode errors and push failures returned verbatim to clients.
- **Fix:** Return generic error messages. Log details server-side.

### W8: `pushAll` runs backends serially
- **File:** `internal/notifier/notifier.go:103`
- **Effort:** medium
- **Status:** `[x]`
- **Issue:** With all three backends enabled, latency compounds.
- **Fix:** Run senders concurrently with `errgroup` or goroutines.

### W9: Inconsistent Discord sender pattern
- **File:** `internal/notifier/discord.go:25`
- **Effort:** easy
- **Status:** `[x]`
- **Issue:** Uses `http.Post` convenience function while ntfy/webhook use `http.NewRequest` + `client.Do`.
- **Fix:** Align to `http.NewRequest` + shared client pattern.

---

## Info / Suggestions

### I1: Use `errors.Join` for error aggregation
- **File:** `internal/notifier/notifier.go:124`
- **Effort:** easy
- **Status:** `[x]`
- **Fix:** Replace `fmt.Errorf("push errors: %v", errs)` with `errors.Join(errs...)`.

### I2: Use `bytes.NewReader` instead of `strings.NewReader`
- **File:** `internal/notifier/discord.go:25`, `webhook.go:23`
- **Effort:** easy
- **Status:** `[x]`
- **Fix:** `bytes.NewReader(payload)` avoids `[]byte→string→[]byte` copy.

### I3: Extract shared 3-tier dispatch logic
- **File:** `internal/notifier/notifier.go`
- **Effort:** medium
- **Status:** `[ ]`
- **Fix:** `Notify` and `NotifyRemote` repeat the 3-tier pattern. Extract `dispatch(cfg, msg, focused)` helper.

### I4: `config.Init()` prints to stdout
- **File:** `internal/config/config.go:157`
- **Effort:** easy
- **Status:** `[x]`
- **Fix:** Return path, let `cmd/` layer print.

### I5: Replace `bash -c` pipeline for idle detection
- **File:** `internal/idle/idle_darwin.go:14`
- **Effort:** medium
- **Status:** `[x]`
- **Fix:** Parse `ioreg` output in Go directly. Eliminates bash/awk dependency.

### I6: Set `Content-Type` on JSON response
- **File:** `internal/server/server.go:38`
- **Effort:** easy
- **Status:** `[x]`
- **Fix:** `w.Header().Set("Content-Type", "application/json")` before writing response.

### I7: Add tests
- **File:** entire repo
- **Effort:** hard
- **Status:** `[x]`
- **Priority targets:** 3-tier dispatch logic, config loading, HTTP handler routing.
- **Done:** 65 tests across 8 files covering config (7), notifier dispatch (24), HTTP backends (13), server routing (13), xmlEscape (9), focus ancestor (9). Minimal refactoring for testability: `LoadFromBytes`, `NewMux`, function variables for idle/focus/systemNotify stubbing.
