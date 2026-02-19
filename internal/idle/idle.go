package idle

import "time"

// Duration returns how long the user has been idle (no keyboard/mouse input).
// Returns 0 if idle time cannot be determined.
func Duration() time.Duration {
	return idleDuration()
}
