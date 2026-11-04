package unlock

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/ishizakahiroshi/kinko/internal/vault"
)

func TestSetupValidatesVaultBeforeProviderWrite(t *testing.T) {
	path, id := createTestVault(t)
	provider := &FakeProvider{}

	if err := Setup(context.Background(), path, provider, testMasterPassword, false); err != nil {
		t.Fatal(err)
	}
	if provider.SetupCalls != 1 {
		t.Fatalf("setup calls = %d, want 1", provider.SetupCalls)
	}
	if provider.LastVaultID != id {
		t.Fatalf("setup vault ID = %q, want %q", provider.LastVaultID, id)
	}
	if provider.LastPassword != testMasterPassword {
		t.Fatal("providerへ渡したpasswordが一致しません")
	}
	if provider.LastReplace {
		t.Fatal("初回setupがreplace扱いになっています")
	}
}

func TestSetupWrongPasswordDoesNotCallProvider(t *testing.T) {
	path, _ := createTestVault(t)
	provider := &FakeProvider{}
	if err := Setup(context.Background(), path, provider, "wrong-synthetic-password", false); err == nil {
		t.Fatal("wrong passwordでsetupが成功しました")
	}
	if provider.SetupCalls != 0 {
		t.Fatalf("wrong passwordでproviderが呼ばれました: %d", provider.SetupCalls)
	}
}

func TestProviderStatusAndDisableDoNotTouchVault(t *testing.T) {
	path, _ := createTestVault(t)
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	provider := &FakeProvider{
		StatusValue: Status{
			Provider:     "fake",
			Level:        LevelUserVerified,
			Binding:      BindingCredentialBound,
			Availability: AvailabilityAvailable,
			Configured:   true,
			Available:    true,
		},
	}

	status, err := ProviderStatus(context.Background(), path, provider)
	if err != nil {
		t.Fatal(err)
	}
	if status.Provider != "fake" || status.Level != LevelUserVerified || status.Binding != BindingCredentialBound || !status.Configured || !status.Available {
		t.Fatalf("unexpected status: %#v", status)
	}
	if err := Disable(context.Background(), path, provider); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("status/disableでvaultが変更されました")
	}
	if provider.DisableCalls != 1 {
		t.Fatalf("disable calls = %d, want 1", provider.DisableCalls)
	}
}

func TestVaultIDSurvivesSave(t *testing.T) {
	path, id := createTestVault(t)
	opened, err := vault.Open(path, testMasterPassword)
	if err != nil {
		t.Fatal(err)
	}
	if err := opened.Set("example-service/token", "synthetic-value"); err != nil {
		t.Fatal(err)
	}
	if err := opened.Save(testMasterPassword); err != nil {
		t.Fatal(err)
	}
	reopened, err := vault.Open(path, testMasterPassword)
	if err != nil {
		t.Fatal(err)
	}
	if reopened.VaultID() != id {
		t.Fatalf("save後のvault ID = %q, want %q", reopened.VaultID(), id)
	}
}

func TestProviderLifecycleDeadlineStopsBlockingFake(t *testing.T) {
	path, _ := createTestVault(t)

	setupBlock := make(chan struct{})
	setupProvider := &FakeProvider{SetupBlock: setupBlock}
	err := SetupWithOptions(context.Background(), path, setupProvider, testMasterPassword, false, ProviderCallOptions{Timeout: time.Nanosecond})
	assertProviderErrorKind(t, err, ErrorTimeout)
	close(setupBlock)

	statusBlock := make(chan struct{})
	statusProvider := &FakeProvider{StatusBlock: statusBlock}
	_, err = ProviderStatusWithOptions(context.Background(), path, statusProvider, ProviderCallOptions{Timeout: time.Nanosecond})
	assertProviderErrorKind(t, err, ErrorTimeout)
	close(statusBlock)

	disableBlock := make(chan struct{})
	disableProvider := &FakeProvider{DisableBlock: disableBlock}
	err = DisableWithOptions(context.Background(), path, disableProvider, ProviderCallOptions{Timeout: time.Nanosecond})
	assertProviderErrorKind(t, err, ErrorTimeout)
	close(disableBlock)
}

func assertProviderErrorKind(t *testing.T, err error, want ErrorKind) {
	t.Helper()
	kind, ok := ProviderErrorKindOf(err)
	if !ok || kind != want {
		t.Fatalf("error = %v, want provider error %q", err, want)
	}
}
