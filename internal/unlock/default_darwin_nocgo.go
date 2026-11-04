//go:build darwin && !cgo

package unlock

func defaultProvider() Provider {
	return UnsupportedProvider{ProviderName: "macos-keychain-cgo-required"}
}
