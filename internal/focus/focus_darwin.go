//go:build darwin

package focus

import (
	"os/exec"
	"strconv"
	"strings"
)

func processInFocusedTerminalState(pid int) (bool, bool) {
	// Get PID of the frontmost application
	out, err := exec.Command("osascript", "-e",
		`tell application "System Events" to get unix id of first process whose frontmost is true`,
	).Output()
	if err != nil {
		return false, false
	}

	focusedPID, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil || focusedPID <= 0 {
		return false, false
	}

	return isAncestor(focusedPID, pid), true
}

func parentPID(pid int) (int, error) {
	out, err := exec.Command("ps", "-o", "ppid=", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(out)))
}
