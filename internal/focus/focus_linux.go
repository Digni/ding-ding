//go:build linux

package focus

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

func processInFocusedTerminal(pid int) bool {
	focusedPID, ok := focusedWindowPID()
	if !ok {
		return false
	}

	// Check if the focused window's PID is an ancestor of the given process.
	// Chain: terminal → shell → agent → ding-ding (or agent → curl for server)
	return isAncestor(focusedPID, pid)
}

func focusedWindowPID() (int, bool) {
	// Try xdotool (X11)
	out, err := exec.Command("xdotool", "getactivewindow", "getwindowpid").Output()
	if err == nil {
		pid, err := strconv.Atoi(strings.TrimSpace(string(out)))
		if err == nil && pid > 0 {
			return pid, true
		}
	}

	// Try kdotool (Wayland/KDE)
	out, err = exec.Command("kdotool", "getactivewindow", "getwindowpid").Output()
	if err == nil {
		pid, err := strconv.Atoi(strings.TrimSpace(string(out)))
		if err == nil && pid > 0 {
			return pid, true
		}
	}

	// Try gdbus (Wayland/GNOME)
	out, err = exec.Command(
		"gdbus",
		"call",
		"--session",
		"--dest", "org.gnome.Shell",
		"--object-path", "/org/gnome/Shell",
		"--method", "org.gnome.Shell.Eval",
		"global.display.focus_window ? global.display.focus_window.get_pid().toString() : '0'",
	).Output()
	if err == nil {
		if pid, ok := parseGnomeShellEvalPID(string(out)); ok {
			return pid, true
		}
	}

	return 0, false
}

func parseGnomeShellEvalPID(out string) (int, bool) {
	s := strings.TrimSpace(out)
	if !strings.HasPrefix(s, "(") || !strings.HasSuffix(s, ")") {
		return 0, false
	}

	s = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(s, "("), ")"))
	parts := strings.SplitN(s, ",", 2)
	if len(parts) != 2 {
		return 0, false
	}

	if strings.TrimSpace(parts[0]) != "true" {
		return 0, false
	}

	value := strings.TrimSpace(parts[1])
	if len(value) < 2 {
		return 0, false
	}

	quote := value[0]
	if (quote != '\'' && quote != '"') || value[len(value)-1] != quote {
		return 0, false
	}

	rawPID := strings.TrimSpace(value[1 : len(value)-1])
	if rawPID == "" {
		return 0, false
	}

	pid, err := strconv.Atoi(rawPID)
	if err != nil || pid <= 0 {
		return 0, false
	}

	return pid, true
}

func parentPID(pid int) (int, error) {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return 0, err
	}

	// /proc/PID/stat format: "pid (comm) state ppid ..."
	// comm can contain spaces and parens, so find the last ')' first
	s := string(data)
	idx := strings.LastIndex(s, ")")
	if idx < 0 {
		return 0, fmt.Errorf("malformed stat")
	}

	fields := strings.Fields(s[idx+2:]) // skip ") "
	if len(fields) < 2 {
		return 0, fmt.Errorf("too few fields")
	}

	return strconv.Atoi(fields[1]) // ppid is field index 1 after state
}
