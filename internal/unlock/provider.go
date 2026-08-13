// Package unlock は、保管庫を開くためのpasswordと端末解除providerの境界を持つ。
//
// OS固有のcredential storeはこのpackageの共通契約を実装するだけであり、
// 共通unit testから直接importしない。providerが返すcredentialは、callerが
// vaultを開いて識別子を照合するまで、package外へ出さない。
package unlock

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"path/filepath"
	"runtime"
	"strings"
)

// SecurityLevel は、providerが返すcredentialの保護強度を表す。
type SecurityLevel string

const (
	// LevelUserVerified は、credential返却前にOSのuser presenceを要求する。
	LevelUserVerified SecurityLevel = "user-verified"
	// LevelSessionProtected は、login sessionの保護領域から取得するが、
	// 毎回の本人確認は保証しない。
	LevelSessionProtected SecurityLevel = "session-protected"
	// LevelPasswordOnly は、端末解除providerを使わない。
	LevelPasswordOnly SecurityLevel = "password-only"
)

// ErrorKind はprovider errorの安全な分類である。値やOS内部IDを含めない。
type ErrorKind string

const (
	ErrorNotConfigured      ErrorKind = "not-configured"
	ErrorUnsupported        ErrorKind = "unsupported"
	ErrorUnavailable        ErrorKind = "unavailable"
	ErrorCanceled           ErrorKind = "cancel"
	ErrorLocked             ErrorKind = "locked"
	ErrorCorruptEntry       ErrorKind = "corrupt-entry"
	ErrorTimeout            ErrorKind = "timeout"
	ErrorAlreadyConfigured  ErrorKind = "already-configured"
	ErrorDeviceBusy         ErrorKind = "device-busy"
	ErrorVerificationFailed ErrorKind = "verification-failed"
)

// ProviderError はproviderから返す、secretを含まないerrorである。
type ProviderError struct {
	kind ErrorKind
}

func (e *ProviderError) Error() string {
	return "kinko: terminal unlock provider error: " + string(e.kind)
}

// Kind はerrorの安全な分類を返す。
func (e *ProviderError) Kind() ErrorKind { return e.kind }

// NewProviderError は、OS adapterが安全な分類だけを返すためのconstructor。
func NewProviderError(kind ErrorKind) error { return &ProviderError{kind: kind} }

// ProviderErrorKind は、errがProviderErrorなら分類を返す。
func ProviderErrorKindOf(err error) (ErrorKind, bool) {
	var providerErr *ProviderError
	if !errors.As(err, &providerErr) {
		return "", false
	}
	return providerErr.kind, true
}

// IsFallbackError はmaster passwordへ戻してよいprovider errorかを判定する。
func IsFallbackError(err error) bool {
	kind, ok := ProviderErrorKindOf(err)
	if !ok {
		return false
	}
	switch kind {
	case ErrorNotConfigured, ErrorUnsupported, ErrorUnavailable:
		return true
	default:
		return false
	}
}

// CredentialMismatchError はproviderが返したcredentialとvaultが一致しない状態。
// このerrorではpassword fallbackせず、vaultを変更しない。
type CredentialMismatchError struct{}

func (CredentialMismatchError) Error() string {
	return "kinko: 端末解除のcredentialが対象vaultと一致しません"
}

// Route はOS provider内でcredentialを検索するための値である。
// Keyはpathそのものではなく、固定namespace付きのhashである。
type Route struct {
	Path string
	Key  string
}

// NewRoute はvault pathからprovider lookup用routeを作る。
func NewRoute(path string) (Route, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return Route{}, err
	}
	clean := filepath.Clean(abs)
	if runtime.GOOS == "windows" {
		// Windows path lookup is case-insensitive; keep route lookup stable
		// when the same vault is spelled with a different drive/path case.
		clean = strings.ToLower(clean)
	}
	sum := sha256.Sum256([]byte("github.com/ishizakahiroshi/kinko/unlock/v1\x00" + clean))
	return Route{Path: clean, Key: hex.EncodeToString(sum[:16])}, nil
}

// Credential はproviderが返す、memory-onlyの解除情報である。
// PasswordとVaultIDをlog、stdout、stderr、child environmentへ出してはならない。
type Credential struct {
	Password string
	VaultID  string
}

// Availability はproviderの利用可否を、statusへ安全に表示できる値で表す。
// provider固有のOS errorや内部IDはここへ持ち込まない。
type Availability string

const (
	AvailabilityAvailable        Availability = "available"
	AvailabilityDeviceNotPresent Availability = "device-not-present"
	AvailabilityNotConfigured    Availability = "not-configured"
	AvailabilityDisabledByPolicy Availability = "disabled-by-policy"
	AvailabilityDeviceBusy       Availability = "device-busy"
	AvailabilityNoActiveWindow   Availability = "no-active-window"
	AvailabilityUnsupported      Availability = "unsupported"
	AvailabilityUnavailable      Availability = "unavailable"
	AvailabilityTimeout          Availability = "timeout"
)

// Status はstatus commandで表示可能なprovider状態である。
type Status struct {
	Provider     string
	Level        SecurityLevel
	Binding      VerificationBinding
	Availability Availability
	Configured   bool
	Available    bool
}

// VerificationBinding はuser verificationとcredential readの結び付きを表す。
type VerificationBinding string

const (
	BindingCredentialBound  VerificationBinding = "credential-bound"
	BindingApplicationGated VerificationBinding = "application-gated"
	BindingSessionProtected VerificationBinding = "session-protected"
	BindingNone             VerificationBinding = "none"
)

// Provider はOS固有の端末解除adapterが実装する共通interfaceである。
type Provider interface {
	Name() string
	Level() SecurityLevel
	Setup(ctx context.Context, route Route, vaultID, password string, replace bool) error
	Unlock(ctx context.Context, route Route) (Credential, error)
	Status(ctx context.Context, route Route) Status
	Disable(ctx context.Context, route Route) error
}
