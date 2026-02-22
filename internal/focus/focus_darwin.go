//go:build darwin

package focus

import (
	"strconv"
	"strings"
)

const frontmostPIDAppleScript = `tell application "System Events" to get unix id of first process whose frontmost is true`

var (
	frontmostPIDFunc = frontmostPID
	processEnvFunc   = processEnv
	processListFunc  = processList
	tmuxClientPIDsFn = tmuxClientPIDs
)

func processInFocusedTerminalState(pid int) (bool, bool) {
	focusedPID, ok := frontmostPIDFunc()
	if !ok {
		return false, false
	}

	// Fast path: traditional direct ancestry still works for non-multiplexer
	// shells and remains the first check.
	if isAncestor(focusedPID, pid) {
		return true, true
	}

	env, err := processEnvFunc(pid)
	if err != nil {
		// If we cannot inspect the process environment, keep prior behavior:
		// active + unfocused is known/unfocused.
		return false, true
	}

	if sessionName := strings.TrimSpace(env["ZELLIJ_SESSION_NAME"]); sessionName != "" {
		return zellijFocusState(sessionName, focusedPID)
	}

	if tmuxEnv := strings.TrimSpace(env["TMUX"]); tmuxEnv != "" {
		return tmuxFocusState(tmuxEnv, focusedPID)
	}

	return false, true
}

func frontmostPID() (int, bool) {
	out, err := osascriptOutputFunc()
	if err != nil {
		return 0, false
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil || pid <= 0 {
		return 0, false
	}

	return pid, true
}
