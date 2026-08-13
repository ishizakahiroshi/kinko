package cli

import (
	"io"
	"os"
	"path/filepath"
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

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
