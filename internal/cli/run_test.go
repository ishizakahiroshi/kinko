package cli

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/ishizakahiroshi/kinko/internal/vault"
)

func TestChildEnvironmentDoesNotInheritVaultPassword(t *testing.T) {
	parent := []string{
		"PATH=C:\\tools",
		EnvPassword + "=parent-master-value",
		"kinko_password=case-variant-value",
		"APP_MODE=test",
		"=C:=C:\\work",
	}
	expanded := []string{"APP_TOKEN=individual-value"}

	got := childEnvironment(parent, expanded)

	for _, entry := range got {
		name, _, ok := strings.Cut(entry, "=")
		if ok && strings.EqualFold(name, EnvPassword) {
			t.Fatalf("子環境に %s が残っている: %q", EnvPassword, entry)
		}
	}

	for _, want := range []string{
		"PATH=C:\\tools",
		"APP_MODE=test",
		"=C:=C:\\work",
		"APP_TOKEN=individual-value",
	} {
		if !containsString(got, want) {
			t.Errorf("子環境に %q が無い: %#v", want, got)
		}
	}
}

func TestExpandRejectsReservedPasswordVariable(t *testing.T) {
	for _, name := range []string{EnvPassword, "kinko_password"} {
		t.Run(name, func(t *testing.T) {
			_, err := expand(name+"=value\n", nil)
			if err == nil {
				t.Fatalf("予約変数 %s を設定できてしまった", name)
			}
			if !strings.Contains(err.Error(), EnvPassword) {
				t.Errorf("エラーに予約変数名が無い: %v", err)
			}
		})
	}
}

func TestExpandRejectsEmptyEnvironmentName(t *testing.T) {
	_, err := expand(" =value\n", nil)
	if err == nil {
		t.Fatal("空の環境変数名を受理してしまった")
	}
}

func TestExpandRejectsDuplicateKeys(t *testing.T) {
	// 同じキーが2回あると、どちらが子プロセスへ渡るかはOS依存になる
	// （glibcのgetenvは先頭一致・Windowsは未定義）。曖昧なまま進めない。
	_, err := expand("A=1\nA=2\n", nil)
	if err == nil {
		t.Fatal("重複キーを受理してしまった")
	}
	if !strings.Contains(err.Error(), "重複") {
		t.Fatalf("err = %v, want 重複を示すメッセージ", err)
	}
}

func TestExpandDuplicateDetectionIsCaseInsensitive(t *testing.T) {
	_, err := expand("Key=1\nKEY=2\n", nil)
	if err == nil {
		t.Fatal("大文字小文字違いの重複キーを受理してしまった")
	}
}

func TestLoadEnvStripsUTF8BOMFromFirstKey(t *testing.T) {
	envFile := filepath.Join(t.TempDir(), "template.env")
	content := "\xef\xbb\xbfPLAIN_KEY=plain-value\n"
	if err := os.WriteFile(envFile, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	env, err := loadEnv(envFile, CommandOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !containsString(env, "PLAIN_KEY=plain-value") {
		t.Fatalf("env = %#v, want PLAIN_KEY=plain-value（BOMが1行目のキー名に残っている可能性）", env)
	}
}

func TestLoadEnvIgnoresVaultReferenceInsideComment(t *testing.T) {
	envFile := filepath.Join(t.TempDir(), "template.env")
	content := "# API_TOKEN=${kinko:example-service/token}\nPLAIN_KEY=plain-value\n"
	if err := os.WriteFile(envFile, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	// 実在しない場所を指しても、コメント内参照だけでは保管庫を開こうと
	// しないはずなので、これがエラーにならないことがそのまま検証になる。
	t.Setenv(EnvVaultPath, filepath.Join(t.TempDir(), "does-not-exist.age"))

	env, err := loadEnv(envFile, CommandOptions{MasterPasswordOnly: true})
	if err != nil {
		t.Fatalf("コメント内の参照だけで保管庫を開こうとした: %v", err)
	}
	if !containsString(env, "PLAIN_KEY=plain-value") {
		t.Fatalf("env = %#v", env)
	}
}

func TestRunWithOptionsAcceptsEnvFileEqualsForm(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "does-not-exist.tmpl")
	err := RunWithOptions([]string{"--env-file=" + missing, "--", "does-not-matter"}, CommandOptions{})
	if err == nil {
		t.Fatal("存在しないテンプレでもエラーにならなかった")
	}
	// "使い方"ではなく"テンプレを読めません"であれば、--env-file=の
	// 値がenvFileとして正しく解釈されテンプレ読み込みまで進んだ証拠になる。
	if !strings.Contains(err.Error(), "テンプレを読めません") {
		t.Fatalf("err = %v, want テンプレ読み込みエラー（--env-file=形式が未解釈の可能性）", err)
	}
}

func TestRunWithOptionsMapsSignalDeathToShellConvention(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windowsにはシグナル終了の概念がない")
	}
	envFile := filepath.Join(t.TempDir(), "empty.env")
	if err := os.WriteFile(envFile, []byte(""), 0o600); err != nil {
		t.Fatal(err)
	}

	err := RunWithOptions([]string{"--env-file", envFile, "--", "sh", "-c", "kill -TERM $$"}, CommandOptions{})
	var childErr *ChildExitError
	if !errors.As(err, &childErr) {
		t.Fatalf("err = %v (%T), want *ChildExitError", err, err)
	}
	const wantCode = 128 + 15 // 128 + SIGTERM
	if childErr.Code != wantCode {
		t.Fatalf("Code = %d, want %d", childErr.Code, wantCode)
	}
}

func TestGetWithOptionsUsesAutomationPasswordWithoutProvider(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.age")
	created, err := vault.Create(path, "synthetic-master-password")
	if err != nil {
		t.Fatal(err)
	}
	if err := created.Set("example-service/token", "synthetic-value"); err != nil {
		t.Fatal(err)
	}
	if err := created.Save("synthetic-master-password"); err != nil {
		t.Fatal(err)
	}
	t.Setenv(EnvVaultPath, path)
	t.Setenv(EnvPassword, "synthetic-master-password")

	oldStdout := os.Stdout
	read, write, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = write
	defer func() { os.Stdout = oldStdout }()

	if err := GetWithOptions([]string{"example-service/token"}, CommandOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := write.Close(); err != nil {
		t.Fatal(err)
	}
	raw, err := io.ReadAll(read)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(raw); got != "synthetic-value" {
		t.Fatalf("stdout = %q, want synthetic value", got)
	}
}

func TestExportPreservesVaultID(t *testing.T) {
	source := filepath.Join(t.TempDir(), "source.age")
	dest := filepath.Join(t.TempDir(), "backup.age")
	created, err := vault.Create(source, "synthetic-master-password")
	if err != nil {
		t.Fatal(err)
	}
	id := created.VaultID()
	if err := created.Set("example-service/token", "synthetic-value"); err != nil {
		t.Fatal(err)
	}
	if err := created.Save("synthetic-master-password"); err != nil {
		t.Fatal(err)
	}
	t.Setenv(EnvVaultPath, source)
	t.Setenv(EnvPassword, "synthetic-master-password")

	if err := ExportWithOptions([]string{dest}, CommandOptions{MasterPasswordOnly: true}); err != nil {
		t.Fatal(err)
	}
	backup, err := vault.Open(dest, "synthetic-master-password")
	if err != nil {
		t.Fatal(err)
	}
	if backup.VaultID() != id {
		t.Fatalf("backup vault ID = %q, want %q", backup.VaultID(), id)
	}
}

func TestExportRefusesExistingDestination(t *testing.T) {
	source := filepath.Join(t.TempDir(), "source.age")
	dest := filepath.Join(t.TempDir(), "backup.age")
	if _, err := vault.Create(source, "synthetic-master-password"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dest, []byte("existing-content"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(EnvVaultPath, source)
	t.Setenv(EnvPassword, "synthetic-master-password")

	if err := ExportWithOptions([]string{dest}, CommandOptions{MasterPasswordOnly: true}); err == nil {
		t.Fatal("既存の出力先を上書きできてしまった")
	}
	got, err := os.ReadFile(dest)
	if err != nil || string(got) != "existing-content" {
		t.Fatalf("既存の出力先が変更された: %q, err=%v", got, err)
	}
}

func TestExportRejectsShortDestPasswordByRuneCountNotByteLength(t *testing.T) {
	source := filepath.Join(t.TempDir(), "source.age")
	dest := filepath.Join(t.TempDir(), "backup.age")
	if _, err := vault.Create(source, "synthetic-master-password"); err != nil {
		t.Fatal(err)
	}
	t.Setenv(EnvVaultPath, source)
	t.Setenv(EnvPassword, "")
	// "あいう" = 3文字・9バイト。旧実装のlen(password)<8はバイト長で
	// 判定していたため9>=8で受理してしまっていた。1行目は保管庫を開く
	// パスワード、2行目が出力先パスワード。
	withPipedStdin(t, "synthetic-master-password", "あいう")

	err := ExportWithOptions([]string{dest}, CommandOptions{MasterPasswordOnly: true})
	if err == nil {
		t.Fatal("3文字（9バイト）の出力先パスワードを受理してしまった（バイト長判定の再発）")
	}
	if !strings.Contains(err.Error(), "8文字以上") {
		t.Fatalf("err = %v, want 8文字以上メッセージ", err)
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
