package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// withPipedStdin はos.Stdinを、2つの改行区切り入力を順に読めるpipeへ差し替える。
// EnvPasswordを設定すると全てのReadPassword呼び出しがそちらを優先してしまうため、
// Initの「パスワード入力→確認入力」のような2段の対話を検証するにはstdinを
// 直接差し替える必要がある。
func withPipedStdin(t *testing.T, lines ...string) {
	t.Helper()
	oldStdin := os.Stdin
	read, write, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range lines {
		if _, err := write.WriteString(line + "\n"); err != nil {
			t.Fatal(err)
		}
	}
	if err := write.Close(); err != nil {
		t.Fatal(err)
	}
	os.Stdin = read
	t.Cleanup(func() { os.Stdin = oldStdin })
}

func TestInitRejectsMismatchedConfirmation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.age")
	t.Setenv(EnvVaultPath, path)
	t.Setenv(EnvPassword, "")
	withPipedStdin(t, "first-password", "second-password")

	err := Init(nil)
	if err == nil {
		t.Fatal("パスワード不一致でもInitが成功してしまった")
	}
	if !strings.Contains(err.Error(), "一致しません") {
		t.Fatalf("err = %v, want 不一致メッセージ", err)
	}
	if _, statErr := os.Stat(path); statErr == nil {
		t.Fatal("パスワード不一致なのにvaultファイルが作られた")
	}
}

func TestInitRejectsShortPasswordByRuneCountNotByteLength(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.age")
	t.Setenv(EnvVaultPath, path)
	t.Setenv(EnvPassword, "")
	// "あいう" は3文字だが UTF-8 では9バイト。旧実装の len(password) < 8 は
	// バイト長判定だったため 9 >= 8 で誤って受理していた。
	withPipedStdin(t, "あいう", "あいう")

	err := Init(nil)
	if err == nil {
		t.Fatal("3文字（9バイト）のパスワードを受理してしまった（バイト長判定の再発）")
	}
	if !strings.Contains(err.Error(), "8文字以上") {
		t.Fatalf("err = %v, want 8文字以上メッセージ", err)
	}
	if _, statErr := os.Stat(path); statErr == nil {
		t.Fatal("拒否されたはずのパスワードでvaultファイルが作られた")
	}
}

func TestInitAcceptsEightRuneMultibytePassword(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.age")
	t.Setenv(EnvVaultPath, path)
	t.Setenv(EnvPassword, "")
	// 8文字・24バイト。ルーン数判定でもバイト長判定でも通るが、
	// ルーン数判定への変更で「マルチバイトでも正しく8文字扱いされる」ことの
	// 対称テストとして置く。
	withPipedStdin(t, "あいうえおかきく", "あいうえおかきく")

	if err := Init(nil); err != nil {
		t.Fatalf("8文字のマルチバイトパスワードを拒否した: %v", err)
	}
}
