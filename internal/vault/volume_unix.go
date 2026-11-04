//go:build linux || darwin

package vault

import (
	"fmt"
	"syscall"
)

const pathsCaseInsensitive = false

// volumeID returns the device ID of the filesystem that holds path, or "" when
// it cannot be determined. It identifies a filesystem, not a physical disk.
func volumeID(path string) string {
	var st syscall.Stat_t
	if err := syscall.Stat(path, &st); err != nil {
		return ""
	}
	return fmt.Sprintf("dev:%d", st.Dev)
}
