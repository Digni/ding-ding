package idle

import "time"

// Duration returns how long the user has been idle (no keyboard/mouse input).
// Returns an error if idle time cannot be determined.
func Duration() (time.Duration, error) {
	return idleDuration()
}
