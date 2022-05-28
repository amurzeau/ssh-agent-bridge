package log

import (
	"syscall"
	"unsafe"
)

var kernel32 = syscall.MustLoadDLL("kernel32.dll")
var outputDebugStringW = kernel32.MustFindProc("OutputDebugStringW")

func outputDebugString(s string) {
	p, err := syscall.UTF16PtrFromString("ssh-agent-bridge: " + s)
	if err == nil {
		outputDebugStringW.Call(uintptr(unsafe.Pointer(p)))
	}
}
