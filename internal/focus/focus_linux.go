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

	return 0, false
}

// isAncestor checks whether ancestorPID is in the process tree above pid.
func isAncestor(ancestorPID, pid int) bool {
	for pid > 1 {
		if pid == ancestorPID {
			return true
		}
		ppid, err := parentPID(pid)
		if err != nil || ppid == pid {
			return false
		}
		pid = ppid
	}
	return false
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
