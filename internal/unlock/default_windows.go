//go:build windows

package unlock

func defaultProvider() Provider { return NewWindowsProvider() }
