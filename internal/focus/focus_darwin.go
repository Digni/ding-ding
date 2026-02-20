//go:build darwin

package focus

import (
	"os/exec"
	"strconv"
	"strings"
)

func processInFocusedTerminal(pid int) bool {
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

	return isAncestor(focusedPID, pid)
}

func parentPID(pid int) (int, error) {
	out, err := exec.Command("ps", "-o", "ppid=", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(out)))
}
