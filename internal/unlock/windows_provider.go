//go:build windows

package unlock

import (
	"context"
	"encoding/json"

	"github.com/ishizakahiroshi/kinko/internal/vault"
)

const windowsCredentialPrefix = "kinko/unlock/v1/"

type windowsConsentVerifierAPI interface {
	supported(context.Context) error
	availability(context.Context) (Availability, error)
	verify(context.Context) error
}

type windowsCredentialStore interface {
	configured(Route) (bool, error)
	read(Route) ([]byte, error)
	write(Route, []byte) error
	delete(Route) error
}

// WindowsProvider はWindows Hello等のOS user presence確認と、
// current userのCredential Managerを組み合わせるadapterである。
//
// Credential Manager単体はuser verificationを強制しないため、Unlockは
// 常にconsent verifierを先に呼び、成功した場合だけcredentialを読む。
// この分離をStatusのapplication-gatedとして明示する。
type WindowsProvider struct {
	verifier windowsConsentVerifierAPI
	store    windowsCredentialStore
}

// NewWindowsProvider はWindows native providerを作る。
func NewWindowsProvider() Provider {
	return &WindowsProvider{
		verifier: windowsConsentVerifier{buildNumber: windowsBuildNumber},
		store:    windowsCredentialManager{},
	}
}

func (p *WindowsProvider) Name() string { return "windows-hello-credential-manager" }

func (p *WindowsProvider) Level() SecurityLevel { return LevelUserVerified }

func (p *WindowsProvider) Setup(ctx context.Context, route Route, vaultID, password string, replace bool) error {
	if err := ctx.Err(); err != nil {
		return providerErrorForContext(err)
	}
	if vaultID == "" || password == "" || !validWindowsVaultID(vaultID) {
		return NewProviderError(ErrorCorruptEntry)
	}
	if err := p.verifier.supported(ctx); err != nil {
		return err
	}
	availability, err := p.verifier.availability(ctx)
	if err != nil {
		return err
	}
	if availability != AvailabilityAvailable {
		return AvailabilityToError(availability)
	}
	if err := ctx.Err(); err != nil {
		return providerErrorForContext(err)
	}
	if !replace {
		exists, err := p.store.configured(route)
		if err != nil {
			return err
		}
		if exists {
			return NewProviderError(ErrorAlreadyConfigured)
		}
	}

	payload, err := json.Marshal(credentialPayload{
		VaultID:  vaultID,
		Password: password,
	})
	if err != nil {
		return NewProviderError(ErrorCorruptEntry)
	}
	if err := ctx.Err(); err != nil {
		return providerErrorForContext(err)
	}
	defer zeroBytes(payload)
	return p.store.write(route, payload)
}

func (p *WindowsProvider) Unlock(ctx context.Context, route Route) (Credential, error) {
	if err := ctx.Err(); err != nil {
		return Credential{}, providerErrorForContext(err)
	}
	if err := p.verifier.supported(ctx); err != nil {
		return Credential{}, err
	}
	if err := p.verifier.verify(ctx); err != nil {
		return Credential{}, err
	}

	if err := ctx.Err(); err != nil {
		return Credential{}, providerErrorForContext(err)
	}
	payload, err := p.store.read(route)
	if err != nil {
		// Verification has already happened. Missing/transient/unknown store
		// failures must not trigger a second password prompt in this invocation.
		return Credential{}, NewProviderError(ErrorCredentialReadFailed)
	}
	defer zeroBytes(payload)

	var stored credentialPayload
	if err := json.Unmarshal(payload, &stored); err != nil {
		return Credential{}, NewProviderError(ErrorCorruptEntry)
	}
	if stored.Password == "" || !validWindowsVaultID(stored.VaultID) {
		return Credential{}, NewProviderError(ErrorCorruptEntry)
	}
	return Credential{Password: stored.Password, VaultID: stored.VaultID}, nil
}

func (p *WindowsProvider) Status(ctx context.Context, route Route) Status {
	status := Status{
		Provider:     p.Name(),
		Level:        p.Level(),
		Binding:      BindingApplicationGated,
		Availability: AvailabilityUnavailable,
		Available:    false,
		Configured:   false,
	}
	if err := ctx.Err(); err != nil {
		status.Availability = availabilityForProviderError(providerErrorForContext(err))
		return status
	}
	if err := p.verifier.supported(ctx); err != nil {
		status.Availability = availabilityForProviderError(err)
		return status
	}
	availability, err := p.verifier.availability(ctx)
	if err != nil {
		status.Availability = availabilityForProviderError(err)
	} else {
		status.Availability = availability
		status.Available = availability == AvailabilityAvailable
	}
	configured, err := p.store.configured(route)
	if err != nil {
		// A transient Credential Manager read failure must not be reported as
		// "available but not configured" — that reads as an invitation to run
		// unlock setup again over an item that may already exist.
		status.Available = false
		status.Availability = AvailabilityUnavailable
		return status
	}
	status.Configured = configured
	return status
}

func (p *WindowsProvider) Disable(ctx context.Context, route Route) error {
	if err := ctx.Err(); err != nil {
		return providerErrorForContext(err)
	}
	if err := p.verifier.supported(ctx); err != nil {
		return err
	}
	return p.store.delete(route)
}

func validWindowsVaultID(id string) bool { return vault.ValidID(id) }

func windowsTarget(route Route) string { return windowsCredentialPrefix + route.Key }
