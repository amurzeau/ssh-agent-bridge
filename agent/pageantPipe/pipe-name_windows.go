package pageantPipe

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"syscall"
	"unsafe"

	"github.com/amurzeau/ssh-agent-bridge/log"
)

const (
	_CRYPTPROTECTMEMORY_CROSS_PROCESS = 1
	_CRYPTPROTECTMEMORY_BLOCK_SIZE    = 16
	_NameUserPrincipal                = 8
)

var (
	winCryptProtectMemory = winAPI("crypt32.dll", "CryptProtectMemory")
	winGetUserNameEx      = winAPI("Secur32.dll", "GetUserNameExA")
	winGetUserName        = winAPI("Advapi32.dll", "GetUserNameA")
)

func winAPI(dllName, funcName string) *syscall.LazyProc {
	proc := syscall.NewLazyDLL(dllName).NewProc(funcName)
	return proc
}

// from https://github.com/github/putty/blob/7003b43963aef6cdf2841c2a882a684025f1d806/windows/wincapi.c#L54
func capiObfuscateString(realname string) string {
	cryptlen := len(realname) + 1
	cryptlen += _CRYPTPROTECTMEMORY_BLOCK_SIZE - 1
	cryptlen /= _CRYPTPROTECTMEMORY_BLOCK_SIZE
	cryptlen *= _CRYPTPROTECTMEMORY_BLOCK_SIZE

	// Add 4 for the cryptlen which will be hashed too (but not encrypted)
	cryptdata := make([]byte, cryptlen)
	copy(cryptdata, realname)

	if winCryptProtectMemory.Find() == nil {
		result, _, err := winCryptProtectMemory.Call(
			uintptr(unsafe.Pointer(&cryptdata[0])),
			uintptr(cryptlen),
			uintptr(_CRYPTPROTECTMEMORY_CROSS_PROCESS))

		if result == 0 {
			log.Errorf("%s: CryptProtectMemory error: %v", PackageName, err)
		}
	} else {
		log.Errorf("%s: CryptProtectMemory is not available (skipping error)", PackageName)
	}

	rawLenBigEndian := make([]byte, 4)
	binary.BigEndian.PutUint32(rawLenBigEndian, uint32(cryptlen))

	sha256Instance := sha256.New()
	sha256Instance.Write(rawLenBigEndian)
	sha256Instance.Write(cryptdata)
	hash := sha256Instance.Sum([]byte{})

	return hex.EncodeToString(hash[:])
}

func getUserNameFromUserPrincipal() (string, error) {
	if winGetUserNameEx.Find() != nil {
		return "", fmt.Errorf("%s: can't get username, function GetUserNameEx not available", PackageName)
	}

	var nameSize uint32 = 0
	result, _, err := winGetUserNameEx.Call(_NameUserPrincipal, 0, uintptr(unsafe.Pointer(&nameSize)))

	log.Debugf("%s: first GetUserNameEx: %d, %s, namesize: %d", PackageName, result, err, nameSize)

	if nameSize == 0 {
		// Unsupported ? We get 0 on Windows 8.1.
		return "", fmt.Errorf("%s: GetUserNameEx returned a null usernamed", PackageName)
	}

	username := make([]uint8, nameSize)
	result, _, err = winGetUserNameEx.Call(
		_NameUserPrincipal,
		uintptr(unsafe.Pointer(&username[0])),
		uintptr(unsafe.Pointer(&nameSize)))

	log.Debugf("%s: second GetUserNameEx: %d, %s, namesize: %d", PackageName, result, err, nameSize)

	if result == 0 {
		return "", fmt.Errorf("%s: failed to get username: %w", PackageName, err)
	}

	// Remove after @ or \0
	var i uint32
	for i = 0; i < nameSize; i++ {
		if username[i] == 0 || username[i] == '@' {
			break
		}
	}
	username = username[:i]

	return string(username), nil
}

func getUserName() (string, error) {
	var nameSize uint32 = 0
	result, _, _ := winGetUserName.Call(0, uintptr(unsafe.Pointer(&nameSize)))
	if result == 0 || nameSize == 0 {
		// when GetUserName can't be called like this
		nameSize = 256
	}

	username := make([]uint8, nameSize)
	result, _, err := winGetUserName.Call(
		uintptr(unsafe.Pointer(&username[0])),
		uintptr(unsafe.Pointer(&nameSize)))

	if result == 0 {
		return "", fmt.Errorf("%s: failed to get username: %w", PackageName, err)
	}
	if nameSize < 1 {
		return "", fmt.Errorf("%s: GetUserName returned an empty username: %w", PackageName, err)
	}

	return string(username[:nameSize-1]), nil
}

func getPageantPipePath() (string, error) {
	suffix := capiObfuscateString("Pageant")

	username, err := getUserNameFromUserPrincipal()
	if err != nil {
		username, err = getUserName()
	}
	if err != nil {
		return "", err
	}

	pipePath := fmt.Sprintf("\\\\.\\pipe\\pageant.%s.%s", username, suffix)

	return pipePath, nil
}
