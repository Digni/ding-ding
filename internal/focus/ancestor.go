package focus

const maxAncestorDepth = 32

// isAncestor checks whether ancestorPID is in the process tree above pid.
func isAncestor(ancestorPID, pid int) bool {
	for i := 0; pid > 1 && i < maxAncestorDepth; i++ {
		if pid == ancestorPID {
			return true
		}
		ppid, err := parentPID(pid)
		if err != nil || ppid == pid || ppid == 0 {
			return false
		}
		pid = ppid
	}
	return false
}
