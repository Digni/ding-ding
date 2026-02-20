//go:build darwin

package idle

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

func idleDuration() (time.Duration, error) {
	out, err := exec.Command("ioreg", "-c", "IOHIDSystem").Output()
	if err != nil {
		return 0, fmt.Errorf("ioreg: %w", err)
	}

	idle, err := parseHIDIdleTime(out)
	if err != nil {
		return 0, fmt.Errorf("parse ioreg output: %w", err)
	}

	return idle, nil
}

func parseHIDIdleTime(out []byte) (time.Duration, error) {
	for _, line := range strings.Split(string(out), "\n") {
		if !strings.Contains(line, "HIDIdleTime") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			return 0, fmt.Errorf("HIDIdleTime line missing '='")
		}

		value := strings.TrimSpace(parts[1])
		if value == "" {
			return 0, fmt.Errorf("HIDIdleTime value missing")
		}

		value = strings.Trim(value, "\"")
		fields := strings.Fields(value)
		if len(fields) == 0 {
			return 0, fmt.Errorf("HIDIdleTime value missing")
		}

		ns, err := strconv.ParseInt(fields[0], 0, 64)
		if err != nil {
			return 0, fmt.Errorf("parse HIDIdleTime %q: %w", fields[0], err)
		}

		return time.Duration(ns) * time.Nanosecond, nil
	}

	return 0, fmt.Errorf("HIDIdleTime not found in ioreg output")
}
