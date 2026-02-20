//go:build linux

package focus

import "testing"

func TestParseGnomeShellEvalPID(t *testing.T) {
	tests := []struct {
		name  string
		input string
		pid   int
		ok    bool
	}{
		{name: "valid single quoted output", input: "(true, '1234')", pid: 1234, ok: true},
		{name: "valid output with whitespace", input: "  ( true , '42' )\n", pid: 42, ok: true},
		{name: "valid double quoted output", input: `(true, "9001")`, pid: 9001, ok: true},
		{name: "eval false", input: "(false, '1234')", pid: 0, ok: false},
		{name: "non numeric pid", input: "(true, 'abc')", pid: 0, ok: false},
		{name: "empty pid", input: "(true, '')", pid: 0, ok: false},
		{name: "zero pid", input: "(true, '0')", pid: 0, ok: false},
		{name: "negative pid", input: "(true, '-1')", pid: 0, ok: false},
		{name: "mismatched quotes", input: "(true, '123\")", pid: 0, ok: false},
		{name: "malformed output", input: "garbage", pid: 0, ok: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pid, ok := parseGnomeShellEvalPID(tt.input)
			if ok != tt.ok {
				t.Fatalf("ok = %v, want %v", ok, tt.ok)
			}
			if pid != tt.pid {
				t.Fatalf("pid = %d, want %d", pid, tt.pid)
			}
		})
	}
}
