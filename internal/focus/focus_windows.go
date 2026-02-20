//go:build windows

package focus

import (
	"syscall"
	"unsafe"
)

var (
	user32              = syscall.NewLazyDLL("user32.dll")
	kernel32            = syscall.NewLazyDLL("kernel32.dll")
	getForegroundWindow = user32.NewProc("GetForegroundWindow")
	getWindowThreadPID  = user32.NewProc("GetWindowThreadProcessId")
	openProcess         = kernel32.NewProc("OpenProcess")
	ntQueryInfoProcess  = syscall.NewLazyDLL("ntdll.dll").NewProc("NtQueryInformationProcess")
	closeHandle         = kernel32.NewProc("CloseHandle")
)

const processQueryInfo = 0x0400

func processInFocusedTerminal(pid int) bool {
	hwnd, _, _ := getForegroundWindow.Call()
	if hwnd == 0 {
		return false
	}

	var focusedPID uint32
	getWindowThreadPID.Call(hwnd, uintptr(unsafe.Pointer(&focusedPID)))
	if focusedPID == 0 {
		return false
	}

	return isAncestor(int(focusedPID), pid)
}

type processBasicInfo struct {
	ExitStatus                   uintptr
	PebBaseAddress               uintptr
	AffinityMask                 uintptr
	BasePriority                 int32
	UniqueProcessID              uintptr
	InheritedFromUniqueProcessID uintptr
}

func parentPID(pid int) (int, error) {
	h, _, err := openProcess.Call(processQueryInfo, 0, uintptr(pid))
	if h == 0 {
		return 0, err
	}
	defer closeHandle.Call(h)

	var info processBasicInfo
	var returnLen uint32
	ret, _, _ := ntQueryInfoProcess.Call(h, 0, uintptr(unsafe.Pointer(&info)), unsafe.Sizeof(info), uintptr(unsafe.Pointer(&returnLen)))
	if ret != 0 {
		return 0, syscall.Errno(ret)
	}

	return int(info.InheritedFromUniqueProcessID), nil
}
