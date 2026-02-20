package focus

import (
	"fmt"
	"testing"
)

func setupParentPIDStub(t *testing.T, tree map[int]int) {
	t.Helper()
	orig := parentPIDFunc
	t.Cleanup(func() { parentPIDFunc = orig })
	parentPIDFunc = func(pid int) (int, error) {
		if ppid, ok := tree[pid]; ok {
			return ppid, nil
		}
		return 0, fmt.Errorf("pid %d not found", pid)
	}
}

func TestIsAncestor_DirectParent(t *testing.T) {
	setupParentPIDStub(t, map[int]int{100: 200})
	if !isAncestor(200, 100) {
		t.Error("expected 200 to be an ancestor of 100")
	}
}

func TestIsAncestor_Grandparent(t *testing.T) {
	setupParentPIDStub(t, map[int]int{100: 200, 200: 300})
	if !isAncestor(300, 100) {
		t.Error("expected 300 to be an ancestor of 100")
	}
}

func TestIsAncestor_NotAncestor(t *testing.T) {
	setupParentPIDStub(t, map[int]int{100: 200, 200: 300})
	if isAncestor(400, 100) {
		t.Error("expected 400 to not be an ancestor of 100")
	}
}

func TestIsAncestor_SelfMatch(t *testing.T) {
	setupParentPIDStub(t, map[int]int{100: 200})
	if !isAncestor(100, 100) {
		t.Error("expected 100 to be an ancestor of itself")
	}
}

func TestIsAncestor_MaxDepthExceeded(t *testing.T) {
	// Build a chain: 1000 -> 1001 -> ... -> 1033
	// That's 34 PIDs and 33 hops from 1000 to 1033.
	// maxAncestorDepth is 32, so the loop exits before reaching 1033.
	tree := make(map[int]int)
	for i := 1000; i <= 1033; i++ {
		tree[i] = i + 1
	}
	setupParentPIDStub(t, tree)

	if isAncestor(1033, 1000) {
		t.Error("expected false when chain exceeds maxAncestorDepth")
	}
}

func TestIsAncestor_ParentPIDError(t *testing.T) {
	setupParentPIDStub(t, map[int]int{})
	if isAncestor(200, 100) {
		t.Error("expected false when parentPID returns an error")
	}
}

func TestIsAncestor_PidOne(t *testing.T) {
	setupParentPIDStub(t, map[int]int{1: 0})
	// pid == 1 fails the loop condition pid > 1, so loop never executes
	if isAncestor(200, 1) {
		t.Error("expected false when starting pid is 1")
	}
}

func TestIsAncestor_PidZero(t *testing.T) {
	setupParentPIDStub(t, map[int]int{0: 0})
	// pid == 0 fails the loop condition pid > 1, so loop never executes
	if isAncestor(200, 0) {
		t.Error("expected false when starting pid is 0")
	}
}

func TestIsAncestor_ParentEqualsSelf(t *testing.T) {
	// pid 100's parent is itself (cycle guard: ppid == pid)
	setupParentPIDStub(t, map[int]int{100: 100})
	if isAncestor(200, 100) {
		t.Error("expected false when parentPID returns the same pid (cycle)")
	}
}
