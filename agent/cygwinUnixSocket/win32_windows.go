package cygwinUnixSocket

import (
	"syscall"
	"unsafe"
)

const (
	_FILE_ATTRIBUTE_READONLY = 0x01
	_FILE_ATTRIBUTE_SYSTEM   = 0x04
)

var (
	winSetFileAttributes = winAPI("kernel32.dll", "SetFileAttributesW")
)

func winAPI(dllName, funcName string) func(...uintptr) (uintptr, uintptr, error) {
	proc := syscall.MustLoadDLL(dllName).MustFindProc(funcName)
	return func(a ...uintptr) (uintptr, uintptr, error) { return proc.Call(a...) }
}

func setFileAttributes(fileName string, flags uint32) error {
	fileNameUnicode, _ := syscall.UTF16PtrFromString(fileName)

	result, _, err := winSetFileAttributes(uintptr(unsafe.Pointer(fileNameUnicode)), uintptr(flags))
	if result != 0 {
		return nil
	} else {
		return err
	}
}
