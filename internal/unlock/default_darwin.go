//go:build darwin && cgo

package unlock

func defaultProvider() Provider { return NewMacOSProvider() }
