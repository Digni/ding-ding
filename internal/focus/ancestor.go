package focus

const maxAncestorDepth = 32

var parentPIDFunc = parentPID

// isAncestor checks whether ancestorPID is in the process tree above pid.
func isAncestor(ancestorPID, pid int) bool {
	for i := 0; pid > 1 && i < maxAncestorDepth; i++ {
		if pid == ancestorPID {
			return true
		}
		ppid, err := parentPIDFunc(pid)
		if err != nil || ppid == pid || ppid == 0 {
			return false
		}
		pid = ppid
	}
	return false
}
