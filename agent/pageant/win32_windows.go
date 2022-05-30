package pageant

import (
	"syscall"
	"unsafe"
)

const (
	agentCopydataID = 0x804e50ba
	_WM_QUIT        = 0x0012
	_WM_COPYDATA    = 74

	_CW_USEDEFAULT = 0x80000000
	_NULL          = 0
	_SW_HIDE       = 0
)

type _COPYDATASTRUCT struct {
	dwData uintptr
	cbData uint32
	lpData unsafe.Pointer
}

type _WNDCLASS struct {
	Style         uint32
	LpfnWndProc   uintptr
	CbClsExtra    int32
	CbWndExtra    int32
	HInstance     uintptr
	HIcon         uintptr
	HCursor       uintptr
	HbrBackground uintptr
	LpszMenuName  *uint16
	LpszClassName *uint16
}

type _MSG struct {
	hwnd     uintptr
	message  int32
	wParam   uintptr
	lParam   uintptr
	time     uint32
	ptX      int32
	ptY      int32
	lPrivate uint32
}

type _MEMORY_BASIC_INFORMATION struct {
	BaseAddress       uintptr
	AllocationBase    uintptr
	AllocationProtect uint32
	PartitionId       uint16
	RegionSize        uintptr
	State             uint32
	Protect           uint32
	Type              uint32
}

var (
	winFindWindow         = winAPI("user32.dll", "FindWindowW")
	winSendMessage        = winAPI("user32.dll", "SendMessageW")
	winCreateWindowEx     = winAPI("user32.dll", "CreateWindowExW")
	winDestroyWindow      = winAPI("user32.dll", "DestroyWindow")
	winShowWindow         = winAPI("user32.dll", "ShowWindow")
	winGetMessage         = winAPI("user32.dll", "GetMessageW")
	winTranslateMessage   = winAPI("user32.dll", "TranslateMessage")
	winDispatchMessage    = winAPI("user32.dll", "DispatchMessageW")
	winRegisterClass      = winAPI("user32.dll", "RegisterClassW")
	winUnregisterClass    = winAPI("user32.dll", "UnregisterClassW")
	winDefWindowProc      = winAPI("user32.dll", "DefWindowProcW")
	winGetCurrentThreadID = winAPI("kernel32.dll", "GetCurrentThreadId")
	winGetModuleHandle    = winAPI("kernel32.dll", "GetModuleHandleW")
	winOpenFileMapping    = winAPI("kernel32.dll", "OpenFileMappingW")
	winVirtualQuery       = winAPI("kernel32.dll", "VirtualQuery")
)

func winAPI(dllName, funcName string) func(...uintptr) (uintptr, uintptr, error) {
	proc := syscall.MustLoadDLL(dllName).MustFindProc(funcName)
	return func(a ...uintptr) (uintptr, uintptr, error) { return proc.Call(a...) }
}
