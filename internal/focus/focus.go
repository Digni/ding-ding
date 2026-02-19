package focus

import "os"

// TerminalFocused reports whether the terminal that launched this process
// is the currently focused window. Returns false if detection fails.
func TerminalFocused() bool {
	return ProcessInFocusedTerminal(os.Getpid())
}

// ProcessInFocusedTerminal reports whether the given PID's terminal
// (anywhere in its process ancestry) is the currently focused window.
// Useful for checking a remote process's focus when the caller (e.g.
// an HTTP server) is not in the same process tree.
func ProcessInFocusedTerminal(pid int) bool {
	return processInFocusedTerminal(pid)
}
