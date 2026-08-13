package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	unlockservice "github.com/ishizakahiroshi/kinko/internal/unlock"
	"github.com/ishizakahiroshi/kinko/internal/vault"
	"golang.org/x/term"
)

// CommandOptions は1 invocationのglobal command optionをまとめる。
type CommandOptions struct {
	// MasterPasswordOnly はOS providerを使わず、既存のpassword readerだけを使う。
	MasterPasswordOnly bool
	// Providerはtestまたは将来のcallerが差し替えるprovider。通常はdefault providerを使う。
	Provider unlockservice.Provider
	// ProviderTimeoutはprovider呼び出しのdeadline。0なら共通既定値を使う。
	ProviderTimeout time.Duration
}

func (options CommandOptions) provider() unlockservice.Provider {
	if options.Provider != nil {
		return options.Provider
	}
	return unlockservice.DefaultProvider()
}

func (options CommandOptions) providerCallOptions() unlockservice.ProviderCallOptions {
	return unlockservice.ProviderCallOptions{Timeout: options.ProviderTimeout}
}

// openVault は既存callerと将来UIが共有する解除境界である。
func (options CommandOptions) openVault(path string) (*unlockservice.OpenResult, error) {
	automationPassword := ""
	if !options.MasterPasswordOnly {
		automationPassword = os.Getenv(EnvPassword)
	}
	interactive := !options.MasterPasswordOnly && term.IsTerminal(int(os.Stdin.Fd()))
	result, err := unlockservice.OpenVault(context.Background(), path, unlockservice.OpenOptions{
		Provider:           options.provider(),
		Interactive:        interactive,
		AutomationPassword: automationPassword,
		ReadPassword:       ReadPassword,
		Timeout:            options.ProviderTimeout,
	})
	if result != nil && result.FallbackReason != "" {
		fmt.Fprintln(os.Stderr, unlockFallbackMessage(result.FallbackReason))
	}
	return result, err
}

func unlockFallbackMessage(kind unlockservice.ErrorKind) string {
	switch kind {
	case unlockservice.ErrorNotConfigured:
		return "端末解除は未設定のため、master passwordへ切り替えます"
	case unlockservice.ErrorUnsupported:
		return "この環境では端末解除を利用できないため、master passwordへ切り替えます"
	case unlockservice.ErrorUnavailable:
		return "端末解除を利用できないため、master passwordへ切り替えます"
	default:
		return "端末解除からmaster passwordへ切り替えます"
	}
}

// AddWithOptions は秘密を追加・更新する。
func AddWithOptions(args []string, options CommandOptions) error {
	if len(args) < 1 {
		return errors.New("使い方: kinko add <名前> [値]")
	}
	name := args[0]

	var value string
	if len(args) >= 2 {
		value = strings.Join(args[1:], " ")
		fmt.Fprintln(os.Stderr,
			"注意: 引数で渡した値はプロセス一覧とシェルの履歴に残ります。値を省略すると入力を求めます。")
	} else {
		v, err := ReadValue(fmt.Sprintf("%s の値: ", name))
		if err != nil {
			return err
		}
		value = v
	}

	path, err := VaultPath()
	if err != nil {
		return err
	}
	result, err := options.openVault(path)
	if err != nil {
		return err
	}
	if err := result.Session.Vault().Set(name, value); err != nil {
		return err
	}
	if err := result.Session.Save(); err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "保存しました: %s\n", name)
	return nil
}

// GetWithOptions は秘密を標準出力へ出す。
func GetWithOptions(args []string, options CommandOptions) error {
	if len(args) != 1 {
		return errors.New("使い方: kinko get <名前>")
	}

	path, err := VaultPath()
	if err != nil {
		return err
	}
	result, err := options.openVault(path)
	if err != nil {
		return err
	}
	value, err := result.Session.Vault().Get(args[0])
	if err != nil {
		return err
	}
	fmt.Print(value)
	return nil
}

// ListWithOptions は秘密の名前一覧を出す。
func ListWithOptions(args []string, options CommandOptions) error {
	path, err := VaultPath()
	if err != nil {
		return err
	}
	result, err := options.openVault(path)
	if err != nil {
		return err
	}

	prefix := ""
	if len(args) >= 1 {
		prefix = args[0]
	}

	count := 0
	for _, name := range result.Session.Vault().Names() {
		if prefix != "" && !strings.HasPrefix(name, prefix) {
			continue
		}
		fmt.Println(name)
		count++
	}
	fmt.Fprintf(os.Stderr, "%d 件\n", count)
	return nil
}

// RemoveWithOptions は秘密を削除する。
func RemoveWithOptions(args []string, options CommandOptions) error {
	if len(args) != 1 {
		return errors.New("使い方: kinko rm <名前>")
	}

	path, err := VaultPath()
	if err != nil {
		return err
	}
	result, err := options.openVault(path)
	if err != nil {
		return err
	}
	if err := result.Session.Vault().Delete(args[0]); err != nil {
		return err
	}
	if err := result.Session.Save(); err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "削除しました: %s\n", args[0])
	return nil
}

// Unlock は端末解除providerのlifecycle commandを処理する。
func Unlock(args []string) error {
	return UnlockWithOptions(args, CommandOptions{})
}

// UnlockWithOptions はproviderとdeadlineを差し替え可能なunlock lifecycle command。
func UnlockWithOptions(args []string, options CommandOptions) error {
	if len(args) == 0 {
		return errors.New("使い方: kinko unlock <setup|status|disable> [オプション]")
	}
	provider := options.provider()
	switch args[0] {
	case "setup":
		return unlockSetupWithOptions(args[1:], provider, options)
	case "status":
		return unlockStatusWithOptions(args[1:], provider, options)
	case "disable":
		return unlockDisableWithOptions(args[1:], provider, options)
	default:
		return fmt.Errorf("不明なunlockサブコマンド: %s", args[0])
	}
}

type unlockFlags struct {
	dryRun  bool
	yes     bool
	replace bool
}

func parseUnlockFlags(args []string, allowReplace bool) (unlockFlags, error) {
	var flags unlockFlags
	for _, arg := range args {
		switch arg {
		case "--dry-run":
			if flags.dryRun {
				return unlockFlags{}, errors.New("--dry-run を重複して指定できません")
			}
			flags.dryRun = true
		case "--yes":
			if flags.yes {
				return unlockFlags{}, errors.New("--yes を重複して指定できません")
			}
			flags.yes = true
		case "--replace":
			if !allowReplace {
				return unlockFlags{}, errors.New("このunlock commandでは--replaceを使えません")
			}
			if flags.replace {
				return unlockFlags{}, errors.New("--replace を重複して指定できません")
			}
			flags.replace = true
		default:
			return unlockFlags{}, fmt.Errorf("不明なunlockオプション: %s", arg)
		}
	}
	return flags, nil
}

func unlockSetup(args []string, provider unlockservice.Provider) error {
	return unlockSetupWithOptions(args, provider, CommandOptions{})
}

func unlockSetupWithOptions(args []string, provider unlockservice.Provider, options CommandOptions) error {
	flags, err := parseUnlockFlags(args, true)
	if err != nil {
		return err
	}
	path, err := VaultPath()
	if err != nil {
		return err
	}
	password, err := ReadPassword("保管庫のパスワード: ")
	if err != nil {
		return err
	}
	if flags.dryRun {
		if _, err := vault.Open(path, password); err != nil {
			return err
		}
		status, err := unlockservice.ProviderStatusWithOptions(context.Background(), path, provider, options.providerCallOptions())
		if err != nil {
			return err
		}
		printUnlockStatus(status)
		fmt.Fprintln(os.Stderr, "dry-run: credentialは保存していません")
		return nil
	}
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return errors.New("unlock setupの実保存はTTYで実行してください。確認を省略する--yesは認証UIの代わりになりません")
	}

	status, err := unlockservice.ProviderStatusWithOptions(context.Background(), path, provider, options.providerCallOptions())
	if err != nil {
		return err
	}
	if status.Configured && !flags.replace {
		return unlockservice.NewProviderError(unlockservice.ErrorAlreadyConfigured)
	}
	if !status.Available {
		return providerStatusError(status)
	}
	if !flags.yes {
		ok, err := confirm("このvaultの端末解除credentialを保存しますか")
		if err != nil {
			return err
		}
		if !ok {
			return errors.New("unlock setupを中止しました")
		}
	}
	if err := unlockservice.SetupWithOptions(context.Background(), path, provider, password, flags.replace, options.providerCallOptions()); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "端末解除を設定しました: %s\n", provider.Name())
	return nil
}

func unlockStatus(args []string, provider unlockservice.Provider) error {
	return unlockStatusWithOptions(args, provider, CommandOptions{})
}

func unlockStatusWithOptions(args []string, provider unlockservice.Provider, options CommandOptions) error {
	if len(args) != 0 {
		return fmt.Errorf("使い方: kinko unlock status")
	}
	path, err := VaultPath()
	if err != nil {
		return err
	}
	status, err := unlockservice.ProviderStatusWithOptions(context.Background(), path, provider, options.providerCallOptions())
	if err != nil {
		return err
	}
	printUnlockStatus(status)
	return nil
}

func unlockDisable(args []string, provider unlockservice.Provider) error {
	return unlockDisableWithOptions(args, provider, CommandOptions{})
}

func unlockDisableWithOptions(args []string, provider unlockservice.Provider, options CommandOptions) error {
	flags, err := parseUnlockFlags(args, false)
	if err != nil {
		return err
	}
	path, err := VaultPath()
	if err != nil {
		return err
	}
	status, err := unlockservice.ProviderStatusWithOptions(context.Background(), path, provider, options.providerCallOptions())
	if err != nil {
		return err
	}
	if flags.dryRun {
		printUnlockStatus(status)
		fmt.Fprintln(os.Stderr, "dry-run: credentialは削除していません")
		return nil
	}
	if !flags.yes && !term.IsTerminal(int(os.Stdin.Fd())) {
		return errors.New("unlock disableはTTYで確認するか--yesを指定してください")
	}
	if !flags.yes {
		ok, err := confirm("このvaultの端末解除credentialを削除しますか")
		if err != nil {
			return err
		}
		if !ok {
			return errors.New("unlock disableを中止しました")
		}
	}
	if err := unlockservice.DisableWithOptions(context.Background(), path, provider, options.providerCallOptions()); err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr, "端末解除を無効にしました")
	return nil
}

func printUnlockStatus(status unlockservice.Status) {
	fmt.Printf("provider: %s\n", status.Provider)
	fmt.Printf("security-level: %s\n", status.Level)
	fmt.Printf("verification-binding: %s\n", status.Binding)
	fmt.Printf("availability: %s\n", status.Availability)
	fmt.Printf("configured: %t\n", status.Configured)
	fmt.Printf("available: %t\n", status.Available)
}

func providerStatusError(status unlockservice.Status) error {
	switch status.Availability {
	case unlockservice.AvailabilityNotConfigured:
		return unlockservice.NewProviderError(unlockservice.ErrorNotConfigured)
	case unlockservice.AvailabilityDeviceNotPresent, unlockservice.AvailabilityDisabledByPolicy,
		unlockservice.AvailabilityUnsupported, unlockservice.AvailabilityNoActiveWindow:
		return unlockservice.NewProviderError(unlockservice.ErrorUnsupported)
	case unlockservice.AvailabilityDeviceBusy:
		return unlockservice.NewProviderError(unlockservice.ErrorDeviceBusy)
	case unlockservice.AvailabilityTimeout:
		return unlockservice.NewProviderError(unlockservice.ErrorTimeout)
	default:
		return unlockservice.NewProviderError(unlockservice.ErrorUnavailable)
	}
}

func confirm(prompt string) (bool, error) {
	fmt.Fprintf(os.Stderr, "%s [y/N]: ", prompt)
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil && line == "" {
		return false, errors.New("確認入力を読めません")
	}
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return true, nil
	default:
		return false, nil
	}
}
