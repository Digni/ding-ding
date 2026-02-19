package focus

// TerminalFocused reports whether the terminal that launched this process
// is the currently focused window. Returns false if detection fails.
func TerminalFocused() bool {
	return terminalFocused()
}
