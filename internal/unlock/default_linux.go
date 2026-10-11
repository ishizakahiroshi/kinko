//go:build linux

package unlock

func defaultProvider() Provider { return NewLinuxProvider() }
