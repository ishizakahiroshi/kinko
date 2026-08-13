// Package cli は kinko のコマンドを実装する。
package cli

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/term"
)

// EnvVaultPath は保管庫の場所を指定する環境変数。
const EnvVaultPath = "KINKO_VAULT"

// EnvPassword はパスワードを渡す環境変数。
//
// # 使いどころと危うさ
//
// CI や自動処理で対話入力ができないときの口。ただし環境変数は
// 子プロセスすべてへ引き継がれ、`env` を出力する何かが1つでもあれば漏れる。
// 手で使うときは設定せず、端末から入力すること。
const EnvPassword = "KINKO_PASSWORD"

// VaultPath は保管庫の場所を決める。
//
// 環境変数があればそれを使い、無ければ OS ごとの既定の場所にする。
// 既定を「ユーザー固有のアプリデータ」に置くのは、同期フォルダ
// （OneDrive・Dropbox・Google Drive）へ入りにくいため。暗号化されているとはいえ、
// 保管庫がクラウドへ自動で上がる状態は既定にしない。
func VaultPath() (string, error) {
	if p := os.Getenv(EnvVaultPath); p != "" {
		return p, nil
	}

	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("kinko: 設定の置き場を解決できません: %w", err)
	}
	return filepath.Join(dir, "kinko", "vault.age"), nil
}

// ReadPassword はパスワードを取得する。
//
// 環境変数があればそれを使う。無ければ端末から入力させる（画面には出さない）。
// 端末でない（パイプで渡された）場合は、標準入力から1行読む。
func ReadPassword(prompt string) (string, error) {
	if p := os.Getenv(EnvPassword); p != "" {
		return p, nil
	}

	fd := int(os.Stdin.Fd())
	if term.IsTerminal(fd) {
		// プロンプトは標準エラーへ出す。標準出力へ出すと、
		// `kinko get x > file` のようなリダイレクトでファイルへ混ざる。
		fmt.Fprint(os.Stderr, prompt)
		raw, err := term.ReadPassword(fd)
		fmt.Fprintln(os.Stderr)
		if err != nil {
			return "", fmt.Errorf("kinko: パスワードを読めません: %w", err)
		}
		return string(raw), nil
	}

	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil && line == "" {
		return "", errors.New("kinko: 標準入力からパスワードを読めません")
	}
	return strings.TrimRight(line, "\r\n"), nil
}

// ReadValue は秘密の値を取得する。
//
// 引数で渡さない形も用意するのは、コマンドライン引数がプロセス一覧（ps）から
// 他の利用者に見え、シェルの履歴にも残るため。
func ReadValue(prompt string) (string, error) {
	fd := int(os.Stdin.Fd())
	if term.IsTerminal(fd) {
		fmt.Fprint(os.Stderr, prompt)
		raw, err := term.ReadPassword(fd)
		fmt.Fprintln(os.Stderr)
		if err != nil {
			return "", fmt.Errorf("kinko: 値を読めません: %w", err)
		}
		return string(raw), nil
	}

	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil && line == "" {
		return "", errors.New("kinko: 標準入力から値を読めません")
	}
	return strings.TrimRight(line, "\r\n"), nil
}
