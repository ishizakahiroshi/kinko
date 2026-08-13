package unlock

import (
	"context"
	"errors"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/ishizakahiroshi/kinko/internal/vault"
)

const testMasterPassword = "synthetic-master-password"

func TestNewRouteUsesStableOpaqueKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.age")
	first, err := NewRoute(path)
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewRoute(path)
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatalf("同じpathのrouteが一致しません: %#v %#v", first, second)
	}
	if strings.Contains(first.Key, "vault") || strings.Contains(first.Key, path) {
		t.Fatalf("route keyがpathを含んでいます: %#v", first)
	}
	if runtime.GOOS == "windows" {
		upper, err := NewRoute(`C:\Synthetic\Vault.age`)
		if err != nil {
			t.Fatal(err)
		}
		lower, err := NewRoute(`c:\synthetic\vault.age`)
		if err != nil {
			t.Fatal(err)
		}
		if upper.Key != lower.Key {
			t.Fatalf("Windows path caseでroute keyが変わります: %q %q", upper.Key, lower.Key)
		}
	}
}

func TestOpenVaultAutomationSkipsProvider(t *testing.T) {
	path, id := createTestVault(t)
	provider := &FakeProvider{
		UnlockErr: NewProviderError(ErrorUnavailable),
	}
	readerCalled := false
	result, err := OpenVault(context.Background(), path, OpenOptions{
		Provider:           provider,
		Interactive:        true,
		AutomationPassword: testMasterPassword,
		ReadPassword: func(string) (string, error) {
			readerCalled = true
			return "", errors.New("reader should not be called")
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Session.Source() != SourceAutomation {
		t.Fatalf("source = %q, want %q", result.Session.Source(), SourceAutomation)
	}
	if result.Session.Vault().VaultID() != id {
		t.Fatalf("vault ID = %q, want %q", result.Session.Vault().VaultID(), id)
	}
	if provider.UnlockCalls != 0 {
		t.Fatalf("automationでproviderが呼ばれました: %d", provider.UnlockCalls)
	}
	if readerCalled {
		t.Fatal("automationでpassword readerが呼ばれました")
	}
}

func TestOpenVaultNonInteractiveSkipsProvider(t *testing.T) {
	path, _ := createTestVault(t)
	provider := &FakeProvider{
		UnlockValue: Credential{Password: testMasterPassword, VaultID: "unused"},
	}
	result, err := OpenVault(context.Background(), path, OpenOptions{
		Provider:    provider,
		Interactive: false,
		ReadPassword: func(string) (string, error) {
			return testMasterPassword, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Session.Source() != SourcePassword {
		t.Fatalf("source = %q, want %q", result.Session.Source(), SourcePassword)
	}
	if provider.UnlockCalls != 0 {
		t.Fatalf("non-TTY相当でproviderが呼ばれました: %d", provider.UnlockCalls)
	}
}

func TestOpenVaultProviderSuccessChecksVaultID(t *testing.T) {
	path, id := createTestVault(t)
	provider := &FakeProvider{
		UnlockValue: Credential{Password: testMasterPassword, VaultID: id},
	}
	result, err := OpenVault(context.Background(), path, OpenOptions{
		Provider:    provider,
		Interactive: true,
		ReadPassword: func(string) (string, error) {
			return "", errors.New("provider success must not prompt")
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Session.Source() != SourceProvider {
		t.Fatalf("source = %q, want %q", result.Session.Source(), SourceProvider)
	}
	if provider.UnlockCalls != 1 {
		t.Fatalf("provider calls = %d, want 1", provider.UnlockCalls)
	}
}

func TestOpenVaultProviderFailuresFallbackToPassword(t *testing.T) {
	cases := []struct {
		name string
		kind ErrorKind
	}{
		{name: "not configured", kind: ErrorNotConfigured},
		{name: "unsupported", kind: ErrorUnsupported},
		{name: "unavailable", kind: ErrorUnavailable},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			path, _ := createTestVault(t)
			provider := &FakeProvider{UnlockErr: NewProviderError(test.kind)}
			result, err := OpenVault(context.Background(), path, OpenOptions{
				Provider:    provider,
				Interactive: true,
				ReadPassword: func(string) (string, error) {
					return testMasterPassword, nil
				},
			})
			if err != nil {
				t.Fatal(err)
			}
			if result.FallbackReason != test.kind {
				t.Fatalf("fallback reason = %q, want %q", result.FallbackReason, test.kind)
			}
			if result.Session.Source() != SourcePassword {
				t.Fatalf("source = %q, want %q", result.Session.Source(), SourcePassword)
			}
		})
	}
}

func TestOpenVaultProviderPromptFailuresStopWithoutPasswordFallback(t *testing.T) {
	for _, test := range []struct {
		name string
		kind ErrorKind
	}{
		{name: "cancel", kind: ErrorCanceled},
		{name: "locked", kind: ErrorLocked},
		{name: "corrupt", kind: ErrorCorruptEntry},
		{name: "timeout", kind: ErrorTimeout},
		{name: "device busy", kind: ErrorDeviceBusy},
		{name: "verification failed", kind: ErrorVerificationFailed},
	} {
		t.Run(test.name, func(t *testing.T) {
			path, _ := createTestVault(t)
			provider := &FakeProvider{UnlockErr: NewProviderError(test.kind)}
			readerCalled := false
			_, err := OpenVault(context.Background(), path, OpenOptions{
				Provider:    provider,
				Interactive: true,
				ReadPassword: func(string) (string, error) {
					readerCalled = true
					return testMasterPassword, nil
				},
			})
			kind, ok := ProviderErrorKindOf(err)
			if !ok || kind != test.kind {
				t.Fatalf("error = %v, want provider error %q", err, test.kind)
			}
			if readerCalled {
				t.Fatal("prompt failureでpassword fallbackが呼ばれました")
			}
		})
	}
}

func TestOpenVaultBlockingProviderDeadlineStopsWithoutPasswordFallback(t *testing.T) {
	path, _ := createTestVault(t)
	release := make(chan struct{})
	provider := &FakeProvider{UnlockBlock: release}
	readerCalled := false
	_, err := OpenVault(context.Background(), path, OpenOptions{
		Provider:    provider,
		Interactive: true,
		Timeout:     time.Nanosecond,
		ReadPassword: func(string) (string, error) {
			readerCalled = true
			return testMasterPassword, nil
		},
	})
	assertProviderErrorKind(t, err, ErrorTimeout)
	if readerCalled {
		t.Fatal("provider deadline後にpassword fallbackが呼ばれました")
	}
	close(release)
}

func TestOpenVaultProviderMismatchStopsWithoutPasswordFallback(t *testing.T) {
	path, id := createTestVault(t)
	provider := &FakeProvider{
		UnlockValue: Credential{Password: testMasterPassword, VaultID: strings.Repeat("0", len(id))},
	}
	readerCalled := false
	result, err := OpenVault(context.Background(), path, OpenOptions{
		Provider:    provider,
		Interactive: true,
		ReadPassword: func(string) (string, error) {
			readerCalled = true
			return testMasterPassword, nil
		},
	})
	if result != nil {
		t.Fatalf("mismatchでresultが返りました: %#v", result)
	}
	var mismatch CredentialMismatchError
	if !errors.As(err, &mismatch) {
		t.Fatalf("error = %v, want CredentialMismatchError", err)
	}
	if readerCalled {
		t.Fatal("credential mismatchでpassword fallbackが呼ばれました")
	}
	if strings.Contains(err.Error(), testMasterPassword) || strings.Contains(err.Error(), id) {
		t.Fatalf("errorにsecretまたはvault IDが含まれています: %v", err)
	}
}

func TestOpenVaultProviderPasswordFailureStopsWithoutPasswordFallback(t *testing.T) {
	path, id := createTestVault(t)
	provider := &FakeProvider{
		UnlockValue: Credential{Password: "wrong-synthetic-password", VaultID: id},
	}
	readerCalled := false
	result, err := OpenVault(context.Background(), path, OpenOptions{
		Provider:    provider,
		Interactive: true,
		ReadPassword: func(string) (string, error) {
			readerCalled = true
			return testMasterPassword, nil
		},
	})
	if result != nil {
		t.Fatalf("credential staleでresultが返りました: %#v", result)
	}
	var mismatch CredentialMismatchError
	if !errors.As(err, &mismatch) {
		t.Fatalf("error = %v, want CredentialMismatchError", err)
	}
	if readerCalled {
		t.Fatal("credential staleでpassword fallbackが呼ばれました")
	}
}

func TestOpenVaultUnknownProviderErrorStopsWithoutPasswordFallback(t *testing.T) {
	path, _ := createTestVault(t)
	provider := &FakeProvider{UnlockErr: errors.New("synthetic provider failure")}
	readerCalled := false
	_, err := OpenVault(context.Background(), path, OpenOptions{
		Provider:    provider,
		Interactive: true,
		ReadPassword: func(string) (string, error) {
			readerCalled = true
			return testMasterPassword, nil
		},
	})
	if err == nil || err.Error() != "synthetic provider failure" {
		t.Fatalf("error = %v, want opaque provider error", err)
	}
	if readerCalled {
		t.Fatal("unknown provider errorでpassword fallbackが呼ばれました")
	}
}

func createTestVault(t *testing.T) (string, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "vault.age")
	created, err := vault.Create(path, testMasterPassword)
	if err != nil {
		t.Fatal(err)
	}
	return path, created.VaultID()
}
