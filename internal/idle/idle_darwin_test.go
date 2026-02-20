//go:build darwin

package idle

import (
	"strings"
	"testing"
	"time"
)

func TestParseHIDIdleTimeSuccess(t *testing.T) {
	input := []byte(`
| |   "HIDIdleTime" = 123456789
`)

	idle, err := parseHIDIdleTime(input)
	if err != nil {
		t.Fatalf("parseHIDIdleTime() error = %v", err)
	}

	if idle != 123456789*time.Nanosecond {
		t.Fatalf("parseHIDIdleTime() = %v, want %v", idle, 123456789*time.Nanosecond)
	}
}

func TestParseHIDIdleTimeMissing(t *testing.T) {
	_, err := parseHIDIdleTime([]byte(`
| |   "SomeOtherKey" = 123
`))
	if err == nil {
		t.Fatal("parseHIDIdleTime() error = nil, want error")
	}

	if !strings.Contains(err.Error(), "HIDIdleTime not found") {
		t.Fatalf("parseHIDIdleTime() error = %q, want missing HIDIdleTime", err)
	}
}

func TestParseHIDIdleTimeMalformedNumber(t *testing.T) {
	_, err := parseHIDIdleTime([]byte(`
| |   "HIDIdleTime" = nope
`))
	if err == nil {
		t.Fatal("parseHIDIdleTime() error = nil, want error")
	}

	if !strings.Contains(err.Error(), "parse HIDIdleTime") {
		t.Fatalf("parseHIDIdleTime() error = %q, want parse error", err)
	}
}

func TestParseHIDIdleTimeSpacingVariation(t *testing.T) {
	input := []byte(`
| |   HIDIdleTime	=	42
`)

	idle, err := parseHIDIdleTime(input)
	if err != nil {
		t.Fatalf("parseHIDIdleTime() error = %v", err)
	}

	if idle != 42*time.Nanosecond {
		t.Fatalf("parseHIDIdleTime() = %v, want %v", idle, 42*time.Nanosecond)
	}
}
