//go:build darwin

package focus

import (
	"os"
	"os/exec"
	"strconv"
	"strings"
)

func terminalFocused() bool {
	// Get PID of the frontmost application
	out, err := exec.Command("osascript", "-e",
		`tell application "System Events" to get unix id of first process whose frontmost is true`,
	).Output()
	if err != nil {
		return false
	}

	focusedPID, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil || focusedPID <= 0 {
		return false
	}

	return isAncestor(focusedPID, os.Getpid())
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
	out, err := exec.Command("ps", "-o", "ppid=", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(out)))
}
