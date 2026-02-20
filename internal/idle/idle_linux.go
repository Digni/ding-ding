//go:build linux

package idle

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

func idleDuration() (time.Duration, error) {
	// Try xprintidle (X11) — returns milliseconds
	out, err := exec.Command("xprintidle").Output()
	if err == nil {
		ms, err := strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64)
		if err == nil {
			return time.Duration(ms) * time.Millisecond, nil
		}
	}

	// Try GNOME/Mutter idle monitor via dbus
	out, err = exec.Command(
		"dbus-send", "--print-reply", "--dest=org.gnome.Mutter.IdleMonitor",
		"/org/gnome/Mutter/IdleMonitor/Core",
		"org.gnome.Mutter.IdleMonitor.GetIdletime",
	).Output()
	if err == nil {
		// Output contains "uint64 <milliseconds>"
		for _, line := range strings.Split(string(out), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "uint64") {
				parts := strings.Fields(line)
				if len(parts) >= 2 {
					ms, err := strconv.ParseInt(parts[1], 10, 64)
					if err == nil {
						return time.Duration(ms) * time.Millisecond, nil
					}
				}
			}
		}
	}

	return 0, fmt.Errorf("idle detection unavailable: xprintidle and dbus-send both failed")
}
