//go:build windows

package vault

import (
	"fmt"
	"syscall"
	"unsafe"
)

const pathsCaseInsensitive = true

var (
	kernel32                  = syscall.NewLazyDLL("kernel32.dll")
	procGetVolumePathNameW    = kernel32.NewProc("GetVolumePathNameW")
	procGetVolumeInformationW = kernel32.NewProc("GetVolumeInformationW")
)

// volumeID returns the serial number of the volume that holds path, or "" when
// it cannot be determined. It identifies a volume, not a physical disk: two
// partitions of one disk have different serials.
func volumeID(path string) string {
	p, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return ""
	}
	root := make([]uint16, 1024)
	ret, _, _ := procGetVolumePathNameW.Call(
		uintptr(unsafe.Pointer(p)),
		uintptr(unsafe.Pointer(&root[0])),
		uintptr(len(root)),
	)
	if ret == 0 {
		return ""
	}
	var serial uint32
	ret, _, _ = procGetVolumeInformationW.Call(
		uintptr(unsafe.Pointer(&root[0])),
		0, 0,
		uintptr(unsafe.Pointer(&serial)),
		0, 0, 0, 0,
	)
	if ret == 0 {
		return ""
	}
	return fmt.Sprintf("win:%08x", serial)
}
