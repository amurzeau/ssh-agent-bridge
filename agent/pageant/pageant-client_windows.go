package pageant

import (
	"encoding/binary"
	"errors"
	"fmt"
	"log"
	"sync"
	"syscall"
	"unsafe"

	"github.com/amurzeau/ssh-agent-bridge/agent"
)

var (
	// ErrPageantNotFound returns when pageant process not found
	ErrPageantNotFound = errors.New("pageant process not found")
	// ErrSendMessage returns when message to pageant cannt be sent
	ErrSendMessage = errors.New("error sending message")

	// ErrMessageTooLong returns when message is too long (see MaxMessageLen)
	ErrMessageTooLong = errors.New("message too long")
	// ErrInvalidMessageFormat returns when message have invalid fomat
	ErrInvalidMessageFormat = errors.New("invalid message format")
	// ErrResponseTooLong returns when response from pageant is too long
	ErrResponseTooLong = errors.New("response too long")
)

/////////////////////////

const (
	agentCopydataID = 0x804e50ba
	wmCopydata      = 74
)

type _COPYDATASTRUCT struct {
	dwData uintptr
	cbData uint32
	lpData unsafe.Pointer
}

var (
	lock sync.Mutex

	winFindWindow         = winAPI("user32.dll", "FindWindowW")
	winGetCurrentThreadID = winAPI("kernel32.dll", "GetCurrentThreadId")
	winSendMessage        = winAPI("user32.dll", "SendMessageW")
)

func winAPI(dllName, funcName string) func(...uintptr) (uintptr, uintptr, error) {
	proc := syscall.MustLoadDLL(dllName).MustFindProc(funcName)
	return func(a ...uintptr) (uintptr, uintptr, error) { return proc.Call(a...) }
}

// Query sends message msg to Pageant and returns response or error.
// 'msg' is raw agent request with length prefix
// Response is raw agent response with length prefix
func query(msg []byte) ([]byte, error) {
	if len(msg) > agent.MAX_AGENT_MESSAGE_SIZE {
		return nil, ErrMessageTooLong
	}

	msgLen := binary.BigEndian.Uint32(msg[:4])
	if len(msg) != int(msgLen)+4 {
		return nil, ErrInvalidMessageFormat
	}

	lock.Lock()
	defer lock.Unlock()

	paWin := pageantWindow()

	if paWin == 0 {
		return nil, ErrPageantNotFound
	}

	thID, _, _ := winGetCurrentThreadID()
	mapName := fmt.Sprintf("PageantRequest%08x", thID)
	pMapName, _ := syscall.UTF16PtrFromString(mapName)

	mmap, err := syscall.CreateFileMapping(syscall.InvalidHandle, nil, syscall.PAGE_READWRITE, 0, agent.MAX_AGENT_MESSAGE_SIZE, pMapName)
	if err != nil {
		return nil, err
	}
	defer syscall.CloseHandle(mmap)

	ptr, err := syscall.MapViewOfFile(mmap, syscall.FILE_MAP_WRITE, 0, 0, 0)
	if err != nil {
		return nil, err
	}
	defer syscall.UnmapViewOfFile(ptr)

	mmSlice := (*(*[agent.MAX_AGENT_MESSAGE_SIZE]byte)(unsafe.Pointer(ptr)))[:]

	copy(mmSlice, msg)

	mapNameBytesZ := append([]byte(mapName), 0)

	cds := _COPYDATASTRUCT{
		dwData: agentCopydataID,
		cbData: uint32(len(mapNameBytesZ)),
		lpData: unsafe.Pointer(&(mapNameBytesZ[0])),
	}

	resp, _, _ := winSendMessage(paWin, wmCopydata, 0, uintptr(unsafe.Pointer(&cds)))

	if resp == 0 {
		return nil, ErrSendMessage
	}

	respLen := binary.BigEndian.Uint32(mmSlice[:4])
	if respLen > agent.MAX_AGENT_MESSAGE_SIZE-4 {
		return nil, ErrResponseTooLong
	}

	respData := make([]byte, respLen+4)
	copy(respData, mmSlice)

	return respData, nil
}

func pageantWindow() uintptr {
	nameP, _ := syscall.UTF16PtrFromString("Pageant")
	h, _, _ := winFindWindow(uintptr(unsafe.Pointer(nameP)), uintptr(unsafe.Pointer(nameP)))
	return h
}

// isPageantAvailable returns true if Pageant is started
func isPageantAvailable() bool { return pageantWindow() != 0 }

func ClientPageant(queryChannel chan agent.AgentMessageQuery) error {
	log.Printf("%s: forwarding to pageant", PackageName)

	if !isPageantAvailable() {
		return fmt.Errorf("%s: error: pageant is not available ! run pageant, else queries will fail", PackageName)
	}

	for message := range queryChannel {
		reply, err := query(message.Data)
		if err != nil {
			log.Printf("%s: query error: %v\n", PackageName, err)
			message.ReplyChannel <- agent.AGENT_MESSAGE_ERROR_REPLY
		} else {
			message.ReplyChannel <- agent.AgentMessageReply{Data: reply}
		}
	}

	return nil
}
