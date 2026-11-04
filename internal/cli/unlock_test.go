package cli

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	unlockservice "github.com/ishizakahiroshi/kinko/internal/unlock"
	"github.com/ishizakahiroshi/kinko/internal/vault"
)

func TestParseUnlockFlags(t *testing.T) {
	flags, err := parseUnlockFlags([]string{"--dry-run", "--yes", "--replace"}, true)
	if err != nil {
		t.Fatal(err)
	}
	if !flags.dryRun || !flags.yes || !flags.replace {
		t.Fatalf("flags = %#v", flags)
	}
	if _, err := parseUnlockFlags([]string{"--replace"}, false); err == nil {
		t.Fatal("disableでreplaceを受理しました")
	}
	if _, err := parseUnlockFlags([]string{"--unknown"}, true); err == nil {
		t.Fatal("unknown flagを受理しました")
	}
}

func TestUnlockFallbackMessageDoesNotIncludeProviderDetails(t *testing.T) {
	message := unlockFallbackMessage(unlockservice.ErrorUnavailable)
	if strings.Contains(message, "synthetic") || strings.Contains(message, "vault_id") {
		t.Fatalf("fallback message contains secret-like detail: %s", message)
	}
}

func TestAddWithOptionsJoinsMultipleArgsWithSpace(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.age")
	if _, err := vault.Create(path, "synthetic-master-password"); err != nil {
		t.Fatal(err)
	}
	t.Setenv(EnvVaultPath, path)
	t.Setenv(EnvPassword, "synthetic-master-password")

	if err := AddWithOptions([]string{"example-service/token", "part-one", "part-two"}, CommandOptions{}); err != nil {
		t.Fatal(err)
	}

	reopened, err := vault.Open(path, "synthetic-master-password")
	if err != nil {
		t.Fatal(err)
	}
	got, err := reopened.Get("example-service/token")
	if err != nil {
		t.Fatal(err)
	}
	if want := "part-one part-two"; got != want {
		t.Fatalf("value = %q, want %q（複数引数はスペースで結合されるはず）", got, want)
	}
}

func TestListWithOptionsFiltersByPrefix(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.age")
	created, err := vault.Create(path, "synthetic-master-password")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"app/one", "app/two", "other/three"} {
		if err := created.Set(name, "synthetic-value"); err != nil {
			t.Fatal(err)
		}
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
	listErr := ListWithOptions([]string{"app/"}, CommandOptions{})
	write.Close()
	os.Stdout = oldStdout
	if listErr != nil {
		t.Fatal(listErr)
	}
	raw, err := io.ReadAll(read)
	if err != nil {
		t.Fatal(err)
	}
	got := string(raw)
	if !strings.Contains(got, "app/one") || !strings.Contains(got, "app/two") {
		t.Fatalf("一覧にprefix一致が欠けている: %q", got)
	}
	if strings.Contains(got, "other/three") {
		t.Fatalf("一覧にprefix不一致が混ざっている: %q", got)
	}
}
