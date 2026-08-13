package unlock

import (
	"context"
	"time"

	"github.com/ishizakahiroshi/kinko/internal/vault"
)

// ProviderCallOptions はprovider呼び出しのdeadlineを注入するための値である。
// OS APIがcontextを受け取らない場合も、caller側はこの期限で戻る。
type ProviderCallOptions struct {
	Timeout time.Duration
}

func (o ProviderCallOptions) timeout() time.Duration {
	if o.Timeout > 0 {
		return o.Timeout
	}
	return defaultProviderTimeout
}

// Setup はmaster passwordでvaultを検証し、legacy vaultならvault IDを追加してから
// providerへcredentialを登録する。providerの登録に失敗しても、master password
// のfallbackは残る。replace=falseではprovider側が既存credentialを保持する。
func Setup(ctx context.Context, path string, provider Provider, password string, replace bool) error {
	return SetupWithOptions(ctx, path, provider, password, replace, ProviderCallOptions{})
}

// SetupWithOptions はprovider.Setupに注入可能なdeadlineを適用する。
func SetupWithOptions(ctx context.Context, path string, provider Provider, password string, replace bool, options ProviderCallOptions) error {
	if provider == nil {
		return NewProviderError(ErrorUnsupported)
	}
	route, err := NewRoute(path)
	if err != nil {
		return err
	}
	opened, err := vault.Open(path, password)
	if err != nil {
		return err
	}
	wasLegacy := opened.VaultID() == ""
	vaultID, err := opened.EnsureID()
	if err != nil {
		return err
	}
	if wasLegacy {
		if err := opened.Save(password); err != nil {
			return err
		}
	}
	return callProviderError(ctx, options, func(providerCtx context.Context) error {
		return provider.Setup(providerCtx, route, vaultID, password, replace)
	})
}

// ProviderStatus はproviderの公開可能な状態だけを返す。
func ProviderStatus(ctx context.Context, path string, provider Provider) (Status, error) {
	return ProviderStatusWithOptions(ctx, path, provider, ProviderCallOptions{})
}

// ProviderStatusWithOptions はprovider.Statusに注入可能なdeadlineを適用する。
func ProviderStatusWithOptions(ctx context.Context, path string, provider Provider, options ProviderCallOptions) (Status, error) {
	if provider == nil {
		return Status{
			Provider:     "none",
			Level:        LevelPasswordOnly,
			Binding:      BindingNone,
			Availability: AvailabilityUnsupported,
			Configured:   false,
			Available:    false,
		}, nil
	}
	route, err := NewRoute(path)
	if err != nil {
		return Status{}, err
	}
	providerCtx, cancel := context.WithTimeout(ctx, options.timeout())
	defer cancel()
	resultCh := make(chan Status, 1)
	go func() {
		resultCh <- provider.Status(providerCtx, route)
	}()
	select {
	case status := <-resultCh:
		return status, nil
	case <-providerCtx.Done():
		return Status{
			Provider:     provider.Name(),
			Level:        provider.Level(),
			Availability: availabilityForProviderError(providerErrorForContext(providerCtx.Err())),
			Available:    false,
		}, providerErrorForContext(providerCtx.Err())
	}
}

// Disable はprovider内のcredentialだけを削除する。vault fileへ触れない。
func Disable(ctx context.Context, path string, provider Provider) error {
	return DisableWithOptions(ctx, path, provider, ProviderCallOptions{})
}

// DisableWithOptions はprovider.Disableに注入可能なdeadlineを適用する。
func DisableWithOptions(ctx context.Context, path string, provider Provider, options ProviderCallOptions) error {
	if provider == nil {
		return NewProviderError(ErrorUnsupported)
	}
	route, err := NewRoute(path)
	if err != nil {
		return err
	}
	return callProviderError(ctx, options, func(providerCtx context.Context) error {
		return provider.Disable(providerCtx, route)
	})
}

func callProviderError(ctx context.Context, options ProviderCallOptions, call func(context.Context) error) error {
	providerCtx, cancel := context.WithTimeout(ctx, options.timeout())
	defer cancel()
	resultCh := make(chan error, 1)
	go func() {
		resultCh <- call(providerCtx)
	}()
	select {
	case err := <-resultCh:
		return err
	case <-providerCtx.Done():
		return providerErrorForContext(providerCtx.Err())
	}
}

func availabilityForProviderError(err error) Availability {
	kind, ok := ProviderErrorKindOf(err)
	if !ok {
		return AvailabilityUnavailable
	}
	switch kind {
	case ErrorTimeout:
		return AvailabilityTimeout
	case ErrorUnsupported:
		return AvailabilityUnsupported
	default:
		return AvailabilityUnavailable
	}
}
