package unlock

import (
	"context"
	"errors"
	"time"

	"github.com/ishizakahiroshi/kinko/internal/vault"
)

const defaultProviderTimeout = 10 * time.Second

// PasswordReader はmaster passwordの入力をcallerから注入する関数型。
type PasswordReader func(prompt string) (string, error)

// Source はvaultを開いた経路である。
type Source string

const (
	SourceAutomation Source = "automation"
	SourceProvider   Source = "provider"
	SourcePassword   Source = "password"
)

// OpenOptions はOpenVaultの入力をまとめる。
type OpenOptions struct {
	Provider           Provider
	Interactive        bool
	AutomationPassword string
	ReadPassword       PasswordReader
	Timeout            time.Duration
}

// OpenResult は開いたsessionと、fallbackが起きた場合の安全な分類を返す。
type OpenResult struct {
	Session        *Session
	FallbackReason ErrorKind
}

// Session は1回のunlockで開いたvaultを保持する。
// passwordはSaveまたはexportの明示的な出力先選択にだけ使い、外へ表示しない。
type Session struct {
	vault    *vault.Vault
	password string
	source   Source
}

// Vault は開いた保管庫を返す。
func (s *Session) Vault() *vault.Vault { return s.vault }

// Source はunlock経路を返す。
func (s *Session) Source() Source { return s.source }

// Save は同じunlock sessionのpasswordで安全に保存する。
func (s *Session) Save() error { return s.vault.Save(s.password) }

// PasswordForExplicitExport はexportの「空ならsourceと同じ」という明示契約のためだけに
// passwordをcallerへ返す。callerはchild processや診断へ渡してはならない。
func (s *Session) PasswordForExplicitExport() string { return s.password }

// OpenVault はautomation、provider、master passwordの順序でvaultを開く。
// providerがcredentialを返した後のvault open失敗・vault ID不一致は、fallbackせず停止する。
func OpenVault(ctx context.Context, path string, options OpenOptions) (*OpenResult, error) {
	route, err := NewRoute(path)
	if err != nil {
		return nil, err
	}

	if options.AutomationPassword != "" {
		return openWithPassword(path, options.AutomationPassword, SourceAutomation, "")
	}

	if options.Interactive && options.Provider != nil {
		timeout := options.Timeout
		if timeout <= 0 {
			timeout = defaultProviderTimeout
		}
		providerCtx, cancel := context.WithTimeout(ctx, timeout)
		resultCh := make(chan providerUnlockResult, 1)
		go func() {
			credential, providerErr := options.Provider.Unlock(providerCtx, route)
			resultCh <- providerUnlockResult{credential: credential, err: providerErr}
		}()
		var result providerUnlockResult
		select {
		case result = <-resultCh:
		case <-providerCtx.Done():
			result.err = providerErrorForContext(providerCtx.Err())
		}
		cancel()
		credential, providerErr := result.credential, result.err

		if providerErr == nil {
			if credential.Password == "" || credential.VaultID == "" {
				providerErr = NewProviderError(ErrorCorruptEntry)
			} else {
				opened, openErr := vault.Open(path, credential.Password)
				if openErr != nil || opened.VaultID() == "" || opened.VaultID() != credential.VaultID {
					return nil, CredentialMismatchError{}
				}
				return &OpenResult{
					Session: &Session{vault: opened, password: credential.Password, source: SourceProvider},
				}, nil
			}
		}

		if providerErr == nil {
			providerErr = NewProviderError(ErrorUnavailable)
		}
		if !IsFallbackError(providerErr) {
			return nil, providerErr
		}
		kind, _ := ProviderErrorKindOf(providerErr)
		return openWithReader(path, options, SourcePassword, kind)
	}

	return openWithReader(path, options, SourcePassword, "")
}

type providerUnlockResult struct {
	credential Credential
	err        error
}

func openWithPassword(path, password string, source Source, reason ErrorKind) (*OpenResult, error) {
	opened, err := vault.Open(path, password)
	if err != nil {
		return nil, err
	}
	return &OpenResult{
		Session:        &Session{vault: opened, password: password, source: source},
		FallbackReason: reason,
	}, nil
}

func openWithReader(path string, options OpenOptions, source Source, reason ErrorKind) (*OpenResult, error) {
	if options.ReadPassword == nil {
		return nil, errors.New("kinko: password readerが設定されていません")
	}
	password, err := options.ReadPassword("保管庫のパスワード: ")
	if err != nil {
		return nil, err
	}
	return openWithPassword(path, password, source, reason)
}
