package cli

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/ishizakahiroshi/kinko/internal/vault"
)

// placeholder はテンプレ内の参照。`${kinko:名前}` の形で書く。
var placeholder = regexp.MustCompile(`\$\{kinko:([^}]+)\}`)

// Run はテンプレの参照を実値へ展開して、子プロセスを起動する。
//
//	kinko run --env-file .env.tmpl -- npm run dev
//
// # これが本体
//
// ディスクに置くのは「参照名だけのテンプレ」と「暗号化された保管庫」の2つで、
// 実値が平文でディスクへ載る瞬間がない。実値は子プロセスの環境変数として
// 渡され、そのプロセスが終われば消える。
//
// テンプレの例（これはリポジトリへコミットしてよい）:
//
//	DB_PASSWORD=${kinko:example-service/db_password}
//	SMTP_USER=noreply@example.com
//
// # 限界
//
// 環境変数は子プロセスから孫プロセスへも引き継がれる。渡した先が
// `env` を出力すれば当然漏れる。「ディスクに置かない」ための仕組みであって、
// 渡した先の振る舞いまでは面倒を見られない。
func Run(args []string) error {
	return RunWithOptions(args, CommandOptions{})
}

// RunWithOptions は解除optionsを受けてtemplate commandを実行する。
func RunWithOptions(args []string, options CommandOptions) error {
	envFile := ""
	rest := args
	for len(rest) > 0 {
		switch {
		case rest[0] == "--env-file":
			if len(rest) < 2 {
				return errors.New("--env-file にファイルを指定してください")
			}
			envFile = rest[1]
			rest = rest[2:]
		case strings.HasPrefix(rest[0], "--env-file="):
			envFile = strings.TrimPrefix(rest[0], "--env-file=")
			rest = rest[1:]
		case rest[0] == "--":
			rest = rest[1:]
			goto done
		default:
			goto done
		}
	}
done:

	if envFile == "" {
		return errors.New("使い方: kinko run --env-file <テンプレ> -- <コマンド> [引数...]")
	}
	if len(rest) == 0 {
		return errors.New("実行するコマンドを指定してください（-- のあとに書きます）")
	}

	template, err := os.ReadFile(envFile)
	if err != nil {
		return fmt.Errorf("kinko: テンプレを読めません: %w", err)
	}

	// 参照が1つも無いなら保管庫を開く必要がない。パスワードを聞かずに済ませる。
	needsVault := placeholder.Match(template)

	var v *vault.Vault
	if needsVault {
		path, err := VaultPath()
		if err != nil {
			return err
		}
		result, err := options.openVault(path)
		if err != nil {
			return err
		}
		v = result.Session.Vault()
	}

	env, err := expand(string(template), v)
	if err != nil {
		return err
	}

	cmd := exec.Command(rest[0], rest[1:]...)
	cmd.Env = childEnvironment(os.Environ(), env)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			// 子プロセスの終了コードをそのまま返す。ラップすると
			// スクリプトから見たときに成否が変わる。
			os.Exit(exitErr.ExitCode())
		}
		return fmt.Errorf("kinko: コマンドを実行できません: %w", err)
	}
	return nil
}

// childEnvironment は子プロセスへ渡す環境を組み立てる。
//
// KINKO_PASSWORD は保管庫全体を開ける資格情報であり、テンプレで指定した
// 個別の秘密より権限が強い。自動処理のため親プロセスへ設定されていても、
// 子プロセスへは引き継がない。Windows では環境変数名の大文字小文字が
// 区別されないため、比較はすべての OS で大文字小文字を無視する。
func childEnvironment(parent, expanded []string) []string {
	out := make([]string, 0, len(parent)+len(expanded))
	for _, entry := range parent {
		name, _, ok := strings.Cut(entry, "=")
		if ok && strings.EqualFold(name, EnvPassword) {
			continue
		}
		out = append(out, entry)
	}
	return append(out, expanded...)
}

// expand はテンプレの各行を KEY=VALUE として読み、参照を実値へ置き換える。
func expand(template string, v *vault.Vault) ([]string, error) {
	var out []string
	var missing []string

	scanner := bufio.NewScanner(strings.NewReader(template))
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			return nil, fmt.Errorf("kinko: テンプレの %d 行目が KEY=VALUE の形ではありません", lineNo)
		}
		key = strings.TrimSpace(key)
		if key == "" {
			return nil, fmt.Errorf("kinko: テンプレの %d 行目の環境変数名が空です", lineNo)
		}
		if strings.EqualFold(key, EnvPassword) {
			return nil, fmt.Errorf(
				"kinko: テンプレの %d 行目では予約変数 %s を設定できません",
				lineNo, EnvPassword)
		}

		expanded := placeholder.ReplaceAllStringFunc(value, func(m string) string {
			name := placeholder.FindStringSubmatch(m)[1]
			if v == nil {
				missing = append(missing, name)
				return ""
			}
			secret, err := v.Get(name)
			if err != nil {
				missing = append(missing, name)
				return ""
			}
			return secret
		})

		out = append(out, key+"="+expanded)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("kinko: テンプレを読めません: %w", err)
	}

	// 見つからない参照は空文字で渡さず、必ず止める。
	// 空のまま起動すると「パスワード無しで接続を試みる」ような動きになり、
	// 原因が設定ミスだと分かりにくい形で失敗する。
	if len(missing) > 0 {
		return nil, fmt.Errorf("kinko: 保管庫に無い参照があります: %s", strings.Join(missing, ", "))
	}
	return out, nil
}

// Export は保管庫を別のファイルへ書き出す（バックアップ用）。
//
// 出力も age で暗号化する。別のパスワードを設定できるようにしているのは、
// バックアップを別の場所（外付けディスク・別マシン）へ置くときに、
// 日常使いのパスワードと分けられるようにするため。
func Export(args []string) error {
	return ExportWithOptions(args, CommandOptions{})
}

// ExportWithOptions は解除optionsを受けて保管庫をexportする。
func ExportWithOptions(args []string, options CommandOptions) error {
	if len(args) != 1 {
		return errors.New("使い方: kinko export <出力先>")
	}
	dest := args[0]

	if _, err := os.Stat(dest); err == nil {
		return fmt.Errorf("kinko: 出力先が既にあります: %s", dest)
	}

	path, err := VaultPath()
	if err != nil {
		return err
	}
	result, err := options.openVault(path)
	if err != nil {
		return err
	}
	session := result.Session
	src := session.Vault()

	destPassword, err := ReadPassword("書き出し先のパスワード（空なら同じものを使う）: ")
	if err != nil {
		return err
	}
	if destPassword == "" {
		// 空入力は既存のexport契約に従った明示選択であり、providerから
		// passwordを暗黙に別APIへ流用することではない。
		destPassword = session.PasswordForExplicitExport()
	} else if len(destPassword) < 8 {
		return errors.New("kinko: パスワードは8文字以上にしてください")
	}

	out, err := vault.Create(dest, destPassword)
	if err != nil {
		return err
	}
	if src.VaultID() != "" {
		if err := out.SetVaultID(src.VaultID()); err != nil {
			return err
		}
	}
	for _, name := range src.Names() {
		value, err := src.Get(name)
		if err != nil {
			return err
		}
		if err := out.Set(name, value); err != nil {
			return err
		}
	}
	if err := out.Save(destPassword); err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "%d 件を書き出しました: %s\n", out.Count(), dest)
	return nil
}
