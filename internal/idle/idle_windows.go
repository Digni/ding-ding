//go:build windows

package idle

import (
	"syscall"
	"time"
	"unsafe"
)

var (
	user32              = syscall.NewLazyDLL("user32.dll")
	kernel32            = syscall.NewLazyDLL("kernel32.dll")
	getLastInputInfo    = user32.NewProc("GetLastInputInfo")
	getTickCount        = kernel32.NewProc("GetTickCount")
)

type lastInputInfo struct {
	cbSize uint32
	dwTime uint32
}

func idleDuration() time.Duration {
	var info lastInputInfo
	info.cbSize = uint32(unsafe.Sizeof(info))

	ret, _, _ := getLastInputInfo.Call(uintptr(unsafe.Pointer(&info)))
	if ret == 0 {
		return 0
	}

	tick, _, _ := getTickCount.Call()
	idleMs := uint32(tick) - info.dwTime

	return time.Duration(idleMs) * time.Millisecond
}
