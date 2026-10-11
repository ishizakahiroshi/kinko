package cli

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"syscall"
	"unicode/utf8"

	"github.com/ishizakahiroshi/kinko/internal/vault"
)

// placeholder はテンプレ内の参照。`${kinko:名前}` の形で書く。
var placeholder = regexp.MustCompile(`\$\{kinko:([^}]+)\}`)

// RunWithOptions はテンプレの参照を実値へ展開して、子プロセスを起動する。
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
func RunWithOptions(args []string, options CommandOptions) error {
	envFile := ""
	seenEnvFile := false
	rest := args
	for len(rest) > 0 {
		switch {
		case rest[0] == "--env-file":
			if seenEnvFile {
				return errors.New("--env-file を重複して指定できません")
			}
			seenEnvFile = true
			if len(rest) < 2 || strings.HasPrefix(rest[1], "--") {
				return errors.New("--env-file にファイルを指定してください")
			}
			envFile = rest[1]
			rest = rest[2:]
		case strings.HasPrefix(rest[0], "--env-file="):
			if seenEnvFile {
				return errors.New("--env-file を重複して指定できません")
			}
			seenEnvFile = true
			envFile = strings.TrimPrefix(rest[0], "--env-file=")
			rest = rest[1:]
		case rest[0] == "--":
			rest = rest[1:]
			goto done
		default:
			if strings.HasPrefix(rest[0], "-") {
				return errors.New("kinko: run のオプションが不正です")
			}
			goto done
		}
	}
done:

	if envFile == "" {
		return errors.New("使い方: kinko run --env-file <テンプレ> -- <コマンド> [引数...]")
	}
	if len(rest) == 0 || strings.TrimSpace(rest[0]) == "" {
		return errors.New("実行するコマンドを指定してください（-- のあとに書きます）")
	}

	env, err := loadEnv(envFile, options)
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
			// スクリプトから見たときに成否が変わる。os.Exitをここで直接呼ぶと
			// テストプロセスごと終了してしまい、この契約への回帰テストが書けない
			// ため、呼び出し元（main）まで運ぶerrorとして返す。
			return &ChildExitError{Code: childExitCode(exitErr)}
		}
		return fmt.Errorf("kinko: コマンドを実行できません: %w", err)
	}
	return nil
}

// loadEnv はenvFileを読み、${kinko:...}参照を実値へ展開したKEY=VALUE一覧を返す。
// 保管庫を開くのは実際に参照があるテンプレのときだけ。read+BOM除去+expandを
// RunWithOptionsの子プロセス起動部分から切り離すことで、動くコマンドを
// 用意しなくてもこのパイプラインだけをin-processで検証できる。
func loadEnv(envFile string, options CommandOptions) ([]string, error) {
	template, err := os.ReadFile(envFile)
	if err != nil {
		return nil, fmt.Errorf("kinko: テンプレを読めません: %w", err)
	}
	// エディタが付けるUTF-8 BOMを落とす。残すと1行目のキー名の先頭に
	// 不可視のBOM文字が混ざり、子プロセスから見えない環境変数として
	// 黙って渡ってしまう。
	template = bytes.TrimPrefix(template, []byte("\xef\xbb\xbf"))

	// 参照が1つも無いなら保管庫を開く必要がない。パスワードを聞かずに済ませる。
	// expand()と同じ行フィルタ（空行・#コメント除外）を通すことで、コメント内の
	// 参照だけで保管庫を開こうとする誤動作を防ぐ。
	needsVault := templateNeedsVault(template)

	var v *vault.Vault
	if needsVault {
		path, err := VaultPath()
		if err != nil {
			return nil, err
		}
		result, err := options.openVault(path)
		if err != nil {
			return nil, err
		}
		v = result.Session.Vault()
	}

	return expand(string(template), v)
}

// ChildExitError はkinko runが起動した子プロセスが非0で終わったことを運ぶ。
// mainはこれをos.Exit(Code)へ写像し、追加のメッセージは出さない
// （子が既にstdout/stderrを持っているため）。
type ChildExitError struct {
	Code int
}

func (e *ChildExitError) Error() string {
	return fmt.Sprintf("kinko: 子プロセスが終了コード %d で終わりました", e.Code)
}

// childExitCode はexec.ExitErrorから終了コードを取り出す。シグナルで死んだ
// 場合はExitCode()が-1（さらにos.Exitでは255）になり、「子と同じ終了コードで
// 終わる」というreference_cli.mdの契約から外れるため、shが使う128+シグナル
// 番号の慣例へ合わせる。Windowsではsyscall.WaitStatus.Signaled()が常にfalseを
// 返すため、この分岐に入らずExitCode()がそのまま使われる。
func childExitCode(exitErr *exec.ExitError) int {
	if status, ok := exitErr.Sys().(syscall.WaitStatus); ok && status.Signaled() {
		return 128 + int(status.Signal())
	}
	return exitErr.ExitCode()
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

// templateNeedsVault は、expandが実際に評価する行（空行・#コメントを除く）に
// ${kinko:...} 参照が1つでもあるかを判定する。placeholder.Matchをテンプレ全体へ
// 直接掛けると、コメント行にある参照だけでも保管庫を開こうとしてしまう。
func templateNeedsVault(template []byte) bool {
	scanner := bufio.NewScanner(bytes.NewReader(template))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if placeholder.MatchString(line) {
			return true
		}
	}
	return false
}

// expand はテンプレの各行を KEY=VALUE として読み、参照を実値へ置き換える。
func expand(template string, v *vault.Vault) ([]string, error) {
	var out []string
	var missing []string
	seenKeys := make(map[string]int)

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
		// 同じキーが2回あると、どちらが子プロセスへ渡るかはOS依存になる
		// （glibcのgetenvは先頭一致・Windowsは未定義）。曖昧なまま黙って
		// 進めず、テンプレの書き手に気づかせる。
		foldKey := strings.ToLower(key)
		if firstLine, dup := seenKeys[foldKey]; dup {
			return nil, fmt.Errorf(
				"kinko: テンプレの %d 行目のキー %s が %d 行目と重複しています",
				lineNo, key, firstLine)
		}
		seenKeys[foldKey] = lineNo

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

// ExportWithOptions は保管庫を別のファイルへ書き出す（バックアップ用）。
//
// 出力も age で暗号化する。別のパスワードを設定できるようにしているのは、
// バックアップを別の場所（外付けディスク・別マシン）へ置くときに、
// 日常使いのパスワードと分けられるようにするため。
func ExportWithOptions(args []string, options CommandOptions) error {
	args, err := positionalArgs(args)
	if err != nil {
		return err
	}
	if len(args) != 1 || strings.TrimSpace(args[0]) == "" {
		return errors.New("使い方: kinko export <出力先>")
	}
	dest := args[0]

	if _, err := os.Lstat(dest); err == nil {
		return fmt.Errorf("kinko: 出力先が既にあります: %s", dest)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("kinko: 出力先を確認できません: %w", err)
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
	} else if utf8.RuneCountInString(destPassword) < 8 {
		return errors.New("kinko: パスワードは8文字以上にしてください")
	}

	// A legacy source first gets a persisted identity, so source and backup
	// remain bound to the same logical vault even across a later reopen.
	if src.VaultID() == "" {
		if err := session.Save(); err != nil {
			return err
		}
	}
	if err := src.Export(dest, destPassword); err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "%d 件を書き出しました: %s\n", src.Count(), dest)
	return nil
}
