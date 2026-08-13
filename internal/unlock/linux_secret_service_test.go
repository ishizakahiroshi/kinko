//go:build linux

package unlock

import (
	"strings"
	"testing"

	"github.com/godbus/dbus/v5"
)

func TestLinuxSecretServicePayloadValidation(t *testing.T) {
	if !validLinuxVaultID(strings.Repeat("0", 32)) {
		t.Fatal("valid vault ID rejected")
	}
	for _, id := range []string{"", "not-a-vault-id", strings.Repeat("0", 31), strings.Repeat("g", 32)} {
		if validLinuxVaultID(id) {
			t.Fatalf("invalid vault ID accepted: %q", id)
		}
	}
}

func TestLinuxProviderErrorDoesNotExposeDBusDetail(t *testing.T) {
	err := linuxProviderError(&dbus.Error{
		Name: "org.freedesktop.Secret.Error.IsLocked",
		Body: []any{"synthetic-account-id"},
	})
	kind, ok := ProviderErrorKindOf(err)
	if !ok || kind != ErrorLocked {
		t.Fatalf("error = %v, want locked provider error", err)
	}
	if strings.Contains(err.Error(), "synthetic-account-id") {
		t.Fatalf("error contains dbus detail: %v", err)
	}
}
