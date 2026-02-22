//go:build darwin

package focus

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"unicode"
)

type processInfo struct {
	PID  int
	PPID int
	Comm string
	Args string
}

var (
	osascriptOutputFunc = func() ([]byte, error) {
		return exec.Command("osascript", "-e", frontmostPIDAppleScript).Output()
	}
	psEnvOutputFunc = func(pid int) ([]byte, error) {
		return exec.Command("ps", "eww", "-p", strconv.Itoa(pid)).Output()
	}
	psProcessListOutputFunc = func() ([]byte, error) {
		return exec.Command("ps", "-axo", "pid=,ppid=,comm=,args=").Output()
	}
	tmuxListClientsOutputFunc = func(socketPath string) ([]byte, error) {
		return exec.Command("tmux", "-S", socketPath, "list-clients", "-F", "#{session_id} #{client_pid}").Output()
	}
)

func processEnv(pid int) (map[string]string, error) {
	out, err := psEnvOutputFunc(pid)
	if err != nil {
		return nil, err
	}
	return parsePSEnvironmentOutput(out)
}

func parsePSEnvironmentOutput(out []byte) (map[string]string, error) {
	lines := strings.Split(string(out), "\n")
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}

		// Skip header and wrapped continuation rows that do not start with PID.
		if _, err := strconv.Atoi(fields[0]); err != nil {
			continue
		}

		env := make(map[string]string)
		for _, token := range fields[1:] {
			key, value, ok := strings.Cut(token, "=")
			if !ok || !isEnvKey(key) {
				continue
			}
			env[key] = value
		}
		return env, nil
	}

	return nil, fmt.Errorf("ps eww output missing process row")
}

func isEnvKey(key string) bool {
	if key == "" {
		return false
	}
	for i, r := range key {
		switch {
		case r == '_':
			continue
		case unicode.IsLetter(r):
			continue
		case i > 0 && unicode.IsDigit(r):
			continue
		default:
			return false
		}
	}
	return true
}

func processList() ([]processInfo, error) {
	out, err := psProcessListOutputFunc()
	if err != nil {
		return nil, err
	}
	return parseProcessListOutput(out)
}

func parseProcessListOutput(out []byte) ([]processInfo, error) {
	lines := strings.Split(string(out), "\n")
	procs := make([]processInfo, 0, len(lines))

	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}

		proc, err := parseProcessListLine(line)
		if err != nil {
			return nil, err
		}
		procs = append(procs, proc)
	}

	if len(procs) == 0 {
		return nil, fmt.Errorf("ps process list output is empty")
	}

	return procs, nil
}

func parseProcessListLine(line string) (processInfo, error) {
	pidField, rest, ok := nextField(line)
	if !ok {
		return processInfo{}, fmt.Errorf("missing pid field")
	}
	ppidField, rest, ok := nextField(rest)
	if !ok {
		return processInfo{}, fmt.Errorf("missing ppid field")
	}
	comm, rest, ok := nextField(rest)
	if !ok {
		return processInfo{}, fmt.Errorf("missing comm field")
	}

	pid, err := strconv.Atoi(pidField)
	if err != nil || pid <= 0 {
		return processInfo{}, fmt.Errorf("invalid pid %q", pidField)
	}
	ppid, err := strconv.Atoi(ppidField)
	if err != nil || ppid < 0 {
		return processInfo{}, fmt.Errorf("invalid ppid %q", ppidField)
	}

	return processInfo{
		PID:  pid,
		PPID: ppid,
		Comm: comm,
		Args: strings.TrimSpace(rest),
	}, nil
}

func nextField(input string) (field string, rest string, ok bool) {
	s := strings.TrimLeft(input, " \t")
	if s == "" {
		return "", "", false
	}

	i := 0
	for i < len(s) && s[i] != ' ' && s[i] != '\t' {
		i++
	}
	if i == 0 {
		return "", "", false
	}

	return s[:i], s[i:], true
}

func tmuxClientPIDs(socketPath string, sessionID string) ([]int, error) {
	out, err := tmuxListClientsOutputFunc(socketPath)
	if err != nil {
		return nil, err
	}
	return parseTMUXClientPIDOutput(out, sessionID)
}

func parseTMUXClientPIDOutput(out []byte, sessionID string) ([]int, error) {
	sessionID = normalizeTMUXSessionID(sessionID)
	if sessionID == "" {
		return nil, fmt.Errorf("tmux session id is empty")
	}

	trimmed := strings.TrimSpace(string(out))
	if trimmed == "" {
		return nil, nil
	}

	lines := strings.Split(trimmed, "\n")
	pids := make([]int, 0, len(lines))
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) != 2 {
			return nil, fmt.Errorf("invalid tmux client row %q", line)
		}

		rowSessionID := normalizeTMUXSessionID(fields[0])
		pid, err := strconv.Atoi(fields[1])
		if err != nil || pid <= 0 {
			return nil, fmt.Errorf("invalid tmux client pid %q", fields[1])
		}

		if rowSessionID != sessionID {
			continue
		}
		pids = append(pids, pid)
	}

	return pids, nil
}

func parentPID(pid int) (int, error) {
	out, err := exec.Command("ps", "-o", "ppid=", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(out)))
}
