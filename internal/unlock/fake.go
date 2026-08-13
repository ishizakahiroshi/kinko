package unlock

import (
	"context"
	"sync"
)

// FakeProvider はOS非依存unit test用のproviderである。
// 実credential storeへ接続せず、呼び出し回数と入力だけを検証できる。
type FakeProvider struct {
	ProviderName  string
	ProviderLevel SecurityLevel
	UnlockValue   Credential
	UnlockErr     error
	StatusValue   Status
	SetupErr      error
	DisableErr    error
	SetupBlock    <-chan struct{}
	StatusBlock   <-chan struct{}
	DisableBlock  <-chan struct{}
	UnlockBlock   <-chan struct{}

	mu           sync.Mutex
	UnlockCalls  int
	SetupCalls   int
	DisableCalls int
	LastRoute    Route
	LastVaultID  string
	LastPassword string
	LastReplace  bool
}

func (p *FakeProvider) Name() string {
	if p.ProviderName == "" {
		return "fake"
	}
	return p.ProviderName
}

func (p *FakeProvider) Level() SecurityLevel {
	if p.ProviderLevel == "" {
		return LevelUserVerified
	}
	return p.ProviderLevel
}

func (p *FakeProvider) Setup(_ context.Context, route Route, vaultID, password string, replace bool) error {
	if p.SetupBlock != nil {
		<-p.SetupBlock
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.SetupCalls++
	p.LastRoute = route
	p.LastVaultID = vaultID
	p.LastPassword = password
	p.LastReplace = replace
	return p.SetupErr
}

func (p *FakeProvider) Unlock(_ context.Context, route Route) (Credential, error) {
	if p.UnlockBlock != nil {
		<-p.UnlockBlock
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.UnlockCalls++
	p.LastRoute = route
	if p.UnlockErr != nil {
		return Credential{}, p.UnlockErr
	}
	return p.UnlockValue, nil
}

func (p *FakeProvider) Status(_ context.Context, route Route) Status {
	if p.StatusBlock != nil {
		<-p.StatusBlock
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.LastRoute = route
	if p.StatusValue.Provider == "" {
		return Status{
			Provider:     p.Name(),
			Level:        p.Level(),
			Binding:      BindingCredentialBound,
			Availability: AvailabilityAvailable,
			Configured:   false,
			Available:    true,
		}
	}
	return p.StatusValue
}

func (p *FakeProvider) Disable(_ context.Context, route Route) error {
	if p.DisableBlock != nil {
		<-p.DisableBlock
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.DisableCalls++
	p.LastRoute = route
	return p.DisableErr
}
