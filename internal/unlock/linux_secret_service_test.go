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

func TestLinuxSecretServiceNoPromptSentinel(t *testing.T) {
	if err := secretServicePromptResult(dbus.ObjectPath("/")); err != nil {
		t.Fatalf("successful no-prompt sentinel rejected: %v", err)
	}
	for _, prompt := range []dbus.ObjectPath{"", "/org/freedesktop/secrets/prompt/example", "invalid"} {
		err := secretServicePromptResult(prompt)
		kind, ok := ProviderErrorKindOf(err)
		if !ok || (kind != ErrorUnavailable && kind != ErrorCorruptEntry) {
			t.Fatal("a real or malformed prompt was accepted")
		}
	}
}

func TestLinuxSecretServiceObjectPathsRejectEmptySentinels(t *testing.T) {
	for _, path := range []dbus.ObjectPath{"", "/", "invalid", "//", "/invalid-path"} {
		if validSecretObjectPath(path) {
			t.Fatal("invalid session, item, or collection path accepted")
		}
	}
	for _, path := range []dbus.ObjectPath{
		"/org/freedesktop/secrets/session/example",
		"/org/freedesktop/secrets/collection/example",
		"/org/freedesktop/secrets/collection/example/item",
	} {
		if !validSecretObjectPath(path) {
			t.Fatal("valid Secret Service object path rejected")
		}
	}
}
