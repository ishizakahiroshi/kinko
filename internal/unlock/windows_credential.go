//go:build windows

package unlock

import (
	"errors"
	"syscall"
	"unsafe"
)

const (
	credTypeGeneric         = 1
	credPersistLocalMachine = 2
	windowsErrorNotFound    = 1168
)

var (
	advapi32       = syscall.NewLazyDLL("advapi32.dll")
	procCredRead   = advapi32.NewProc("CredReadW")
	procCredWrite  = advapi32.NewProc("CredWriteW")
	procCredDelete = advapi32.NewProc("CredDeleteW")
	procCredFree   = advapi32.NewProc("CredFree")
)

type windowsFiletime struct {
	LowDateTime  uint32
	HighDateTime uint32
}

// windowsCredential mirrors CREDENTIALW. Pointer-sized fields must remain
// pointer types so the struct layout is correct on amd64 and arm64.
type windowsCredential struct {
	Flags              uint32
	Type               uint32
	TargetName         *uint16
	Comment            *uint16
	LastWritten        windowsFiletime
	CredentialBlobSize uint32
	CredentialBlob     *byte
	Persist            uint32
	AttributeCount     uint32
	Attributes         *byte
	TargetAlias        *uint16
	UserName           *uint16
}

type windowsCredentialManager struct{}

type windowsCredentialNotFound struct{}

func (windowsCredentialNotFound) Error() string { return "credential not found" }

func (windowsCredentialManager) configured(route Route) (bool, error) {
	raw, err := (windowsCredentialManager{}).read(route)
	if err != nil {
		if errors.As(err, new(windowsCredentialNotFound)) {
			return false, nil
		}
		return false, err
	}
	zeroBytes(raw)
	return true, nil
}

func (windowsCredentialManager) read(route Route) ([]byte, error) {
	target, err := syscall.UTF16PtrFromString(windowsTarget(route))
	if err != nil {
		return nil, NewProviderError(ErrorUnavailable)
	}

	var native *windowsCredential
	ret, _, callErr := procCredRead.Call(
		uintptr(unsafe.Pointer(target)),
		credTypeGeneric,
		0,
		uintptr(unsafe.Pointer(&native)),
	)
	if ret == 0 {
		if errno, ok := callErr.(syscall.Errno); ok && errno == windowsErrorNotFound {
			return nil, windowsCredentialNotFound{}
		}
		return nil, NewProviderError(ErrorUnavailable)
	}
	if native == nil {
		return nil, NewProviderError(ErrorCorruptEntry)
	}
	defer procCredFree.Call(uintptr(unsafe.Pointer(native)))

	if native.CredentialBlobSize == 0 || native.CredentialBlob == nil {
		return nil, NewProviderError(ErrorCorruptEntry)
	}
	blob := unsafe.Slice(native.CredentialBlob, native.CredentialBlobSize)
	copyOfBlob := append([]byte(nil), blob...)
	zeroBytes(blob)
	return copyOfBlob, nil
}

func (windowsCredentialManager) write(route Route, payload []byte) error {
	if len(payload) == 0 {
		return NewProviderError(ErrorCorruptEntry)
	}
	target, err := syscall.UTF16PtrFromString(windowsTarget(route))
	if err != nil {
		return NewProviderError(ErrorUnavailable)
	}
	user, err := syscall.UTF16PtrFromString("kinko")
	if err != nil {
		return NewProviderError(ErrorUnavailable)
	}
	blob := append([]byte(nil), payload...)
	defer zeroBytes(blob)

	native := windowsCredential{
		Type:               credTypeGeneric,
		TargetName:         target,
		CredentialBlobSize: uint32(len(blob)),
		CredentialBlob:     &blob[0],
		Persist:            credPersistLocalMachine,
		UserName:           user,
	}
	ret, _, _ := procCredWrite.Call(uintptr(unsafe.Pointer(&native)), 0)
	if ret == 0 {
		return NewProviderError(ErrorUnavailable)
	}
	return nil
}

func (windowsCredentialManager) delete(route Route) error {
	target, err := syscall.UTF16PtrFromString(windowsTarget(route))
	if err != nil {
		return NewProviderError(ErrorUnavailable)
	}
	ret, _, callErr := procCredDelete.Call(uintptr(unsafe.Pointer(target)), credTypeGeneric, 0)
	if ret != 0 {
		return nil
	}
	if errno, ok := callErr.(syscall.Errno); ok && errno == windowsErrorNotFound {
		return NewProviderError(ErrorNotConfigured)
	}
	return NewProviderError(ErrorUnavailable)
}
