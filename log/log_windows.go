package log

import (
	"syscall"
	"unsafe"
)

var kernel32 = syscall.MustLoadDLL("kernel32.dll")
var outputDebugStringW = kernel32.MustFindProc("OutputDebugStringW")
var messageBoxW = syscall.MustLoadDLL("user32.dll").MustFindProc("MessageBoxW")

const _MB_ICONERROR = 0x10

func outputDebugString(s string) {
	p, err := syscall.UTF16PtrFromString("ssh-agent-bridge: " + s)
	if err == nil {
		outputDebugStringW.Call(uintptr(unsafe.Pointer(p)))
	}
}

func messageBox(msg string) {
	titleUnicode, _ := syscall.UTF16PtrFromString("ssh-agent-bridge")
	msgUnicode, _ := syscall.UTF16PtrFromString(msg)

	messageBoxW.Call(0, uintptr(unsafe.Pointer(msgUnicode)), uintptr(unsafe.Pointer(titleUnicode)), _MB_ICONERROR)
}
