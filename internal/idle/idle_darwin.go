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
	// ioreg reports HIDIdleTime in nanoseconds
	out, err := exec.Command("bash", "-c",
		`ioreg -c IOHIDSystem | awk '/HIDIdleTime/ {print $NF; exit}'`,
	).Output()
	if err != nil {
		return 0, fmt.Errorf("ioreg: %w", err)
	}

	ns, err := strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse ioreg output: %w", err)
	}

	return time.Duration(ns) * time.Nanosecond, nil
}
