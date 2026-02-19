//go:build darwin

package idle

import (
	"os/exec"
	"strconv"
	"strings"
	"time"
)

func idleDuration() time.Duration {
	// ioreg reports HIDIdleTime in nanoseconds
	out, err := exec.Command("bash", "-c",
		`ioreg -c IOHIDSystem | awk '/HIDIdleTime/ {print $NF; exit}'`,
	).Output()
	if err != nil {
		return 0
	}

	ns, err := strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64)
	if err != nil {
		return 0
	}

	return time.Duration(ns) * time.Nanosecond
}
