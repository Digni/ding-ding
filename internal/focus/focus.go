package focus

import "os"

// State describes focus detection output.
// Known=false means focus detection could not determine a result.
type State struct {
	Focused bool
	Known   bool
}

// TerminalFocused reports whether the terminal that launched this process
// is the currently focused window. Returns false if detection fails.
func TerminalFocused() bool {
	state := TerminalFocusState()
	return state.Known && state.Focused
}

// ProcessInFocusedTerminal reports whether the given PID's terminal
// (anywhere in its process ancestry) is the currently focused window.
// Useful for checking a remote process's focus when the caller (e.g.
// an HTTP server) is not in the same process tree.
func ProcessInFocusedTerminal(pid int) bool {
	state := ProcessFocusState(pid)
	return state.Known && state.Focused
}

// TerminalFocusState reports focus and whether detection is known.
func TerminalFocusState() State {
	return ProcessFocusState(os.Getpid())
}

// ProcessFocusState reports focus and detection certainty for a PID's terminal.
func ProcessFocusState(pid int) State {
	focused, known := processInFocusedTerminalState(pid)
	return State{Focused: focused, Known: known}
}
