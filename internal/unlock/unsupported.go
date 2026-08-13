package unlock

import "context"

// UnsupportedProvider は、対象OS・build tagでadapterが無い場合の安全なfallback。
type UnsupportedProvider struct {
	ProviderName string
}

func (p UnsupportedProvider) Name() string {
	if p.ProviderName == "" {
		return "unsupported"
	}
	return p.ProviderName
}

func (p UnsupportedProvider) Level() SecurityLevel { return LevelPasswordOnly }

func (p UnsupportedProvider) Setup(context.Context, Route, string, string, bool) error {
	return NewProviderError(ErrorUnsupported)
}

func (p UnsupportedProvider) Unlock(context.Context, Route) (Credential, error) {
	return Credential{}, NewProviderError(ErrorUnsupported)
}

func (p UnsupportedProvider) Status(context.Context, Route) Status {
	return Status{
		Provider:     p.Name(),
		Level:        LevelPasswordOnly,
		Binding:      BindingNone,
		Availability: AvailabilityUnsupported,
		Configured:   false,
		Available:    false,
	}
}

func (p UnsupportedProvider) Disable(context.Context, Route) error {
	return NewProviderError(ErrorUnsupported)
}
