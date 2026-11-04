//go:build !windows && !linux && !darwin

package unlock

import "runtime"

func defaultProvider() Provider {
	return UnsupportedProvider{ProviderName: runtime.GOOS + "-pending"}
}
