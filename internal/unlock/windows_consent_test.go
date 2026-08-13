//go:build windows

package unlock

import (
	"context"
	"strings"
	"testing"
)

func TestConsentResultErrorsAreSafe(t *testing.T) {
	cases := map[uint32]ErrorKind{
		consentVerified:         "",
		consentDeviceNotPresent: ErrorVerificationFailed,
		consentNotConfigured:    ErrorVerificationFailed,
		consentDisabledByPolicy: ErrorVerificationFailed,
		consentDeviceBusy:       ErrorDeviceBusy,
		consentRetriesExhausted: ErrorLocked,
		consentCanceled:         ErrorCanceled,
		99:                      ErrorVerificationFailed,
	}
	for result, want := range cases {
		err := consentResultError(result)
		if want == "" {
			if err != nil {
				t.Fatalf("result %d error = %v, want nil", result, err)
			}
			continue
		}
		kind, ok := ProviderErrorKindOf(err)
		if !ok || kind != want {
			t.Fatalf("result %d error = %v, want %q", result, err, want)
		}
	}
}

func TestConsentAvailabilityMapsPreRequestStates(t *testing.T) {
	cases := map[uint32]struct {
		availability Availability
		fallback     bool
	}{
		consentVerified:         {AvailabilityAvailable, false},
		consentDeviceNotPresent: {AvailabilityDeviceNotPresent, true},
		consentNotConfigured:    {AvailabilityNotConfigured, true},
		consentDisabledByPolicy: {AvailabilityDisabledByPolicy, true},
		consentDeviceBusy:       {AvailabilityDeviceBusy, false},
		99:                      {AvailabilityUnavailable, true},
	}
	for result, want := range cases {
		t.Run(string(want.availability), func(t *testing.T) {
			if got := consentAvailability(result); got != want.availability {
				t.Fatalf("availability = %q, want %q", got, want.availability)
			}
			err := consentAvailabilityError(want.availability)
			if got := IsFallbackError(err); got != want.fallback {
				t.Fatalf("fallback = %t, want %t for %q", got, want.fallback, want.availability)
			}
		})
	}
}

func TestSelectActiveApplicationWindowRejectsUnsafeCandidates(t *testing.T) {
	valid := consoleWindowCandidate{handle: 7, valid: true, visible: true}
	if got := selectActiveApplicationWindow(valid, 99); got != valid.handle {
		t.Fatalf("valid console hwnd = %d, want %d", got, valid.handle)
	}
	for name, candidate := range map[string]consoleWindowCandidate{
		"message-only": {handle: 7, valid: true, visible: false, messageOnly: true},
		"invalid":      {handle: 7, valid: false, visible: true},
		"empty":        {},
	} {
		t.Run(name, func(t *testing.T) {
			if got := selectActiveApplicationWindow(candidate, 99); got != 0 {
				t.Fatalf("unsafe console hwnd = %d, want zero", got)
			}
		})
	}
}

func TestNoActiveApplicationWindowDoesNotInvokeConsentRequest(t *testing.T) {
	called := false
	err := requestWithActiveWindow(context.Background(), func() uintptr { return 0 }, func(uintptr) error {
		called = true
		return nil
	})
	if called {
		t.Fatal("HWNDが無いのにconsent requestが呼ばれました")
	}
	kind, ok := ProviderErrorKindOf(err)
	if !ok || kind != ErrorUnsupported {
		t.Fatalf("error = %v, want unsupported", err)
	}
}

func TestWindowsCredentialTargetIsOpaque(t *testing.T) {
	route := Route{Path: `C:\private\synthetic-vault.age`, Key: strings.Repeat("a", 32)}
	target := windowsTarget(route)
	if strings.Contains(target, route.Path) || !strings.HasPrefix(target, windowsCredentialPrefix) {
		t.Fatalf("credential target = %q, want opaque route key", target)
	}
}

func TestWindowsVaultIDValidation(t *testing.T) {
	if !validWindowsVaultID(strings.Repeat("0", 32)) {
		t.Fatal("valid vault ID rejected")
	}
	for _, id := range []string{"", "not-a-vault-id", strings.Repeat("0", 31), strings.Repeat("g", 32)} {
		if validWindowsVaultID(id) {
			t.Fatalf("invalid vault ID accepted: %q", id)
		}
	}
}
