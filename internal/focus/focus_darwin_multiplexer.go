//go:build darwin

package focus

import "strings"

func zellijFocusState(sessionName string, focusedPID int) (bool, bool) {
	procs, err := processListFunc()
	if err != nil {
		return false, false
	}

	for _, proc := range procs {
		if !isZellijClientProcess(proc) {
			continue
		}
		if !zellijSessionMatches(proc.Args, sessionName) {
			continue
		}
		if isAncestor(focusedPID, proc.PID) {
			return true, true
		}
	}

	return false, true
}

func isZellijClientProcess(proc processInfo) bool {
	if !strings.EqualFold(proc.Comm, "zellij") {
		return false
	}
	return !isZellijServerProcess(proc.Args)
}

func isZellijServerProcess(args string) bool {
	for _, token := range strings.Fields(args) {
		if token == "--server" || strings.HasPrefix(token, "--server=") {
			return true
		}
	}
	return false
}

func zellijSessionMatches(args string, sessionName string) bool {
	if sessionName == "" {
		return false
	}

	fields := strings.Fields(args)
	for i := 0; i < len(fields); i++ {
		token := fields[i]
		switch {
		case token == "-s" || token == "--session":
			if i+1 < len(fields) && fields[i+1] == sessionName {
				return true
			}
		case strings.HasPrefix(token, "-s="):
			if strings.TrimPrefix(token, "-s=") == sessionName {
				return true
			}
		case strings.HasPrefix(token, "--session="):
			if strings.TrimPrefix(token, "--session=") == sessionName {
				return true
			}
		case (token == "attach" || token == "a") && i+1 < len(fields):
			if fields[i+1] == sessionName {
				return true
			}
		}
	}

	// Strict fallback: avoid substring matching session names (e.g. "dev" in
	// "dev2"), which can incorrectly mark unrelated sessions as focused.
	for _, token := range fields {
		if token == sessionName {
			return true
		}
	}

	return false
}

func tmuxFocusState(tmuxEnv string, focusedPID int) (bool, bool) {
	socketPath, sessionID, ok := tmuxSocketAndSession(tmuxEnv)
	if !ok {
		return false, false
	}

	clientPIDs, err := tmuxClientPIDsFn(socketPath, sessionID)
	if err != nil {
		return false, false
	}

	for _, clientPID := range clientPIDs {
		if isAncestor(focusedPID, clientPID) {
			return true, true
		}
	}

	return false, true
}

func tmuxSocketAndSession(tmuxEnv string) (string, string, bool) {
	parts := strings.Split(strings.TrimSpace(tmuxEnv), ",")
	if len(parts) < 3 {
		return "", "", false
	}

	socketPath := strings.TrimSpace(parts[0])
	sessionID := normalizeTMUXSessionID(parts[2])
	if socketPath == "" || sessionID == "" {
		return "", "", false
	}

	return socketPath, sessionID, true
}

func normalizeTMUXSessionID(sessionID string) string {
	return strings.TrimPrefix(strings.TrimSpace(sessionID), "$")
}
