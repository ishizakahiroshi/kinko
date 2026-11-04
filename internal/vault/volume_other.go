//go:build !windows && !linux && !darwin

package vault

const pathsCaseInsensitive = false

// volumeID is unknown on this OS; destinations are then reported as
// "disk identity unknown" rather than as being off this PC.
func volumeID(string) string { return "" }
