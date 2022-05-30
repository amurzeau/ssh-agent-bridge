package pageant

import (
	"encoding/binary"
	"errors"
	"fmt"
	"syscall"
	"unsafe"

	"github.com/amurzeau/ssh-agent-bridge/agent"
	"github.com/amurzeau/ssh-agent-bridge/log"
)

var (
	// ErrPageantNotFound returns when pageant process not found
	ErrPageantExists = errors.New("pageant is already existing, can't listen for pageant requests")
)

type pageantServerContext struct {
	ctx          *agent.AgentContext
	ReplyChannel chan agent.AgentMessageReply
}

var globalPageantState pageantServerContext = pageantServerContext{}

func (p *pageantServerContext) processPageantQuery(mapNameZ []byte) error {
	if len(mapNameZ) < 1 || mapNameZ[len(mapNameZ)-1] != 0 {
		return fmt.Errorf("%s: bad map name, should end with \\0", PackageName)
	}

	mapName := (string)(mapNameZ[:len(mapNameZ)-1])

	pMapName, _ := syscall.UTF16PtrFromString(mapName)

	log.Debugf("%s: opening memory map at %s", PackageName, mapName)

	mmap, _, err := winOpenFileMapping(syscall.FILE_MAP_WRITE, 0, (uintptr)(unsafe.Pointer(pMapName)))
	if mmap == _NULL {
		return fmt.Errorf("%s: failed to open memory map at \"%s\" (%d bytes) (OpenFileMapping): %w",
			PackageName,
			mapName,
			len(mapName),
			err)
	}
	defer syscall.CloseHandle((syscall.Handle)(mmap))

	var memoryBasicInformation _MEMORY_BASIC_INFORMATION

	mbiSize, _, err := winVirtualQuery(mmap,
		(uintptr)(unsafe.Pointer(&memoryBasicInformation)),
		unsafe.Sizeof(memoryBasicInformation))

	if mbiSize < unsafe.Sizeof(memoryBasicInformation) {
		return fmt.Errorf("%s: failed to get memory map size (VirtualQuery): %w", PackageName, err)
	}

	log.Debugf("%s: map size: %d", PackageName, memoryBasicInformation.RegionSize)

	ptr, err := syscall.MapViewOfFile((syscall.Handle)(mmap), syscall.FILE_MAP_WRITE, 0, 0, 0)
	if ptr == _NULL {
		return fmt.Errorf("%s: failed map view of memory (MapViewOfFile): %w", PackageName, err)
	}
	defer syscall.UnmapViewOfFile(ptr)

	memoryMapSize := memoryBasicInformation.RegionSize
	mmSlice := unsafe.Slice((*byte)(unsafe.Pointer(ptr)), memoryMapSize)

	agentMessageSize := binary.BigEndian.Uint32(mmSlice[:4])
	if agentMessageSize > agent.MAX_AGENT_MESSAGE_SIZE-4 {
		return fmt.Errorf("%s: received agent message too long: %d > %d",
			PackageName,
			agentMessageSize,
			agent.MAX_AGENT_MESSAGE_SIZE-4)
	}

	msg := make([]byte, agentMessageSize+4)
	copy(msg, mmSlice)

	p.ctx.QueryChannel <- agent.AgentMessageQuery{Data: msg, ReplyChannel: p.ReplyChannel}

	agentMessageQuery := <-p.ReplyChannel

	copy(mmSlice, agentMessageQuery.Data)

	return nil
}

func handlerPageantWindowProc(hwnd uintptr, msg uint32, wParam uintptr, lParam uintptr) uintptr {
	var result uintptr = 0

	switch msg {
	case _WM_COPYDATA:
		copyData := *(*_COPYDATASTRUCT)(unsafe.Pointer(lParam))
		if copyData.dwData == agentCopydataID {
			if err := globalPageantState.processPageantQuery(unsafe.Slice((*byte)(copyData.lpData), copyData.cbData)); err != nil {
				log.Errorf("%s: received bad message: %v", PackageName, err)
			} else {
				result = 1
			}
		}
	default:
		result, _, _ = winDefWindowProc(hwnd, uintptr(msg), wParam, lParam)
	}
	return result
}

func (p *pageantServerContext) handlerPageantMessages(hInstance uintptr, nameP *uint16, hwndPageant uintptr) {
	var msg _MSG

	p.ReplyChannel = make(chan agent.AgentMessageReply)
	defer close(p.ReplyChannel)

	defer winUnregisterClass(uintptr(unsafe.Pointer(nameP)), hInstance)
	defer winDestroyWindow(hwndPageant)

	go func() {
		<-p.ctx.Done()
		log.Debugf("%s: stopping", PackageName)
		winPostMessage(hwndPageant, _WM_QUIT, 0, 0)
	}()

	for {
		result, _, err := winGetMessage(uintptr(unsafe.Pointer(&msg)), hwndPageant, 0, 0)

		log.Debugf("%s: received window message %d: %d", PackageName, result, msg.message)

		if int32(result) == 0 {
			break
		} else if int32(result) == -1 {
			log.Errorf("%s: error while processing pageant messages: %v", PackageName, err)
			break
		}

		winTranslateMessage(uintptr(unsafe.Pointer(&msg)))
		winDispatchMessage(uintptr(unsafe.Pointer(&msg)))
	}

	log.Debugf("%s: stopped", PackageName)
}

func (p *pageantServerContext) createPageantWindow() error {
	// based on https://github.com/github/putty/blob/7003b43963aef6cdf2841c2a882a684025f1d806/windows/winpgnt.c#L1178

	windowNameUnicode, _ := syscall.UTF16PtrFromString("Pageant")
	hInstance, _, _ := winGetModuleHandle(0)

	atom, _, err := winRegisterClass(uintptr(unsafe.Pointer(&_WNDCLASS{
		Style:         0,
		LpfnWndProc:   syscall.NewCallback(handlerPageantWindowProc),
		CbClsExtra:    0,
		CbWndExtra:    0,
		HInstance:     hInstance,
		HIcon:         0,
		HCursor:       0,
		HbrBackground: 0,
		LpszMenuName:  windowNameUnicode,
		LpszClassName: windowNameUnicode,
	})))

	if atom == 0 {
		return fmt.Errorf("%s: RegisterClass failed: %w", PackageName, err)
	}

	defer func() {
		if atom != 0 {
			winUnregisterClass(uintptr(unsafe.Pointer(windowNameUnicode)), hInstance)
		}
	}()

	hwndPageant, _, err := winCreateWindowEx(
		0, // dwExStyle
		uintptr(unsafe.Pointer(windowNameUnicode)), // lpClassName
		uintptr(unsafe.Pointer(windowNameUnicode)), // lpWindowName
		0,              // dwStyle
		_CW_USEDEFAULT, // x
		_CW_USEDEFAULT, // y
		100,            // nWidth
		100,            // nHeight
		_NULL,          // hWndParent
		_NULL,          // hMenu
		hInstance,      // hInstance
		_NULL,          // lpParam
	)

	if hwndPageant == 0 {
		return fmt.Errorf("%s: CreateWindow failed: %w", PackageName, err)
	}

	winShowWindow(hwndPageant, _SW_HIDE)

	atom = 0 // disable UnregisterClass in defer
	p.handlerPageantMessages(hInstance, windowNameUnicode, hwndPageant)

	return nil
}

func ServePageant(ctx *agent.AgentContext) {
	if isPageantAvailable() {
		log.Errorf("%s: error: a pageant is already existing, can't listen for pageant requests", PackageName)
		return
	}

	log.Infof("%s: listening for pageant requests\n", PackageName)

	globalPageantState.ctx = ctx

	err := globalPageantState.createPageantWindow()
	if err != nil {
		log.Debugf("%s: failed to create pageant window: %v", PackageName, err)
	}
}
