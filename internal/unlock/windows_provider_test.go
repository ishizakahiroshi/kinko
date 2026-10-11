//go:build windows

package unlock

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

type fakeWindowsVerifier struct {
	build             uint32
	availabilityValue Availability
	availabilityErr   error
	verifyErr         error
	availabilityCalls int
	verifyCalls       int
}

func (v *fakeWindowsVerifier) supported(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return providerErrorForContext(err)
	}
	return windowsBuildGate(v.build, func() error { return nil })
}

func (v *fakeWindowsVerifier) availability(ctx context.Context) (Availability, error) {
	v.availabilityCalls++
	if err := ctx.Err(); err != nil {
		return AvailabilityTimeout, providerErrorForContext(err)
	}
	if v.availabilityErr != nil {
		return AvailabilityUnavailable, v.availabilityErr
	}
	return v.availabilityValue, nil
}

func (v *fakeWindowsVerifier) verify(ctx context.Context) error {
	v.verifyCalls++
	if err := ctx.Err(); err != nil {
		return providerErrorForContext(err)
	}
	return v.verifyErr
}

type countingWindowsCredentialStore struct {
	configuredCalls int
	readCalls       int
	writeCalls      int
	deleteCalls     int
}

func (s *countingWindowsCredentialStore) configured(Route) (bool, error) {
	s.configuredCalls++
	return false, nil
}

func (s *countingWindowsCredentialStore) read(Route) ([]byte, error) {
	s.readCalls++
	return nil, NewProviderError(ErrorNotConfigured)
}

func (s *countingWindowsCredentialStore) write(Route, []byte) error {
	s.writeCalls++
	return nil
}

func (s *countingWindowsCredentialStore) delete(Route) error {
	s.deleteCalls++
	return nil
}

func requireWindowsProviderErrorKind(t *testing.T, err error, want ErrorKind) {
	t.Helper()
	kind, ok := ProviderErrorKindOf(err)
	if !ok || kind != want {
		t.Fatalf("error = %v, want %q", err, want)
	}
}

func TestWindowsBuildBoundary(t *testing.T) {
	if windowsBuildSupported(21999) {
		t.Fatal("build 21999 must be unsupported")
	}
	if !windowsBuildSupported(22000) {
		t.Fatal("build 22000 must continue to later availability judgment")
	}
}

func TestWindowsConsentAvailabilityRejectsOldBuildBeforeNativeCall(t *testing.T) {
	verifier := windowsConsentVerifier{
		buildNumber: func() (uint32, error) { return 21999, nil },
	}
	availability, err := verifier.availability(context.Background())
	if availability != AvailabilityUnsupported {
		t.Fatalf("availability = %q, want unsupported", availability)
	}
	requireWindowsProviderErrorKind(t, err, ErrorUnsupported)
}

func TestWindowsBuild21999DoesNotCallCredentialManager(t *testing.T) {
	verifier := &fakeWindowsVerifier{
		build:             21999,
		availabilityValue: AvailabilityAvailable,
	}
	store := &countingWindowsCredentialStore{}
	provider := &WindowsProvider{verifier: verifier, store: store}
	route := Route{Key: strings.Repeat("a", 32)}

	requireWindowsProviderErrorKind(t,
		provider.Setup(context.Background(), route, strings.Repeat("0", 32), "synthetic-password", false),
		ErrorUnsupported)
	_, err := provider.Unlock(context.Background(), route)
	requireWindowsProviderErrorKind(t, err, ErrorUnsupported)
	status := provider.Status(context.Background(), route)
	if status.Availability != AvailabilityUnsupported || status.Configured || status.Available {
		t.Fatalf("status = %+v, want unsupported and unconfigured", status)
	}
	requireWindowsProviderErrorKind(t, provider.Disable(context.Background(), route), ErrorUnsupported)

	if verifier.availabilityCalls != 0 || verifier.verifyCalls != 0 {
		t.Fatalf("Hello calls = availability:%d verify:%d, want zero", verifier.availabilityCalls, verifier.verifyCalls)
	}
	if store.configuredCalls != 0 || store.readCalls != 0 || store.writeCalls != 0 || store.deleteCalls != 0 {
		t.Fatalf("Credential Manager calls = configured:%d read:%d write:%d delete:%d, want zero",
			store.configuredCalls, store.readCalls, store.writeCalls, store.deleteCalls)
	}
}

func TestWindowsBuild22000ContinuesToAvailabilityJudgment(t *testing.T) {
	verifier := &fakeWindowsVerifier{
		build:           22000,
		availabilityErr: NewProviderError(ErrorNotConfigured),
	}
	store := &countingWindowsCredentialStore{}
	provider := &WindowsProvider{verifier: verifier, store: store}
	route := Route{Key: strings.Repeat("b", 32)}

	err := provider.Setup(context.Background(), route, strings.Repeat("1", 32), "synthetic-password", false)
	requireWindowsProviderErrorKind(t, err, ErrorNotConfigured)
	if verifier.availabilityCalls != 1 {
		t.Fatalf("availability calls = %d, want 1 after supported build gate", verifier.availabilityCalls)
	}
	if store.configuredCalls != 0 || store.writeCalls != 0 {
		t.Fatalf("Credential Manager calls = configured:%d write:%d, want zero before availability succeeds",
			store.configuredCalls, store.writeCalls)
	}
}

type failingWindowsCredentialStore struct {
	countingWindowsCredentialStore
	readErr error
}

func (s *failingWindowsCredentialStore) read(Route) ([]byte, error) {
	s.readCalls++
	return nil, s.readErr
}

func TestWindowsPostVerificationReadFailureNeverFallsBack(t *testing.T) {
	for _, readErr := range []error{
		NewProviderError(ErrorUnavailable),
		NewProviderError(ErrorNotConfigured),
		windowsCredentialNotFound{},
		errors.New("synthetic-private-detail"),
	} {
		verifier := &fakeWindowsVerifier{build: 22000}
		store := &failingWindowsCredentialStore{readErr: readErr}
		provider := &WindowsProvider{verifier: verifier, store: store}
		readerCalled := false
		result, err := OpenVault(context.Background(), filepath.Join(t.TempDir(), "vault.age"), OpenOptions{
			Provider:    provider,
			Interactive: true,
			ReadPassword: func(string) (string, error) {
				readerCalled = true
				return "synthetic-password", nil
			},
		})
		requireWindowsProviderErrorKind(t, err, ErrorCredentialReadFailed)
		if result != nil || readerCalled || verifier.verifyCalls != 1 || store.readCalls != 1 {
			t.Fatal("post-verification failure crossed the fallback boundary")
		}
		if strings.Contains(err.Error(), "synthetic-private-detail") {
			t.Fatal("native store detail reached diagnostics")
		}
	}
}

func TestWindowsCanceledVerificationNeverReadsCredential(t *testing.T) {
	verifier := &fakeWindowsVerifier{build: 22000, verifyErr: NewProviderError(ErrorCanceled)}
	store := &countingWindowsCredentialStore{}
	provider := &WindowsProvider{verifier: verifier, store: store}
	_, err := provider.Unlock(context.Background(), Route{Key: strings.Repeat("a", 32)})
	requireWindowsProviderErrorKind(t, err, ErrorCanceled)
	if store.readCalls != 0 || store.configuredCalls != 0 {
		t.Fatal("canceled verification read credential storage")
	}
}
