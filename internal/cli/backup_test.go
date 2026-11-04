package cli

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/ishizakahiroshi/kinko/internal/vault"
)

func countAgeFiles(t *testing.T, root string) int {
	t.Helper()
	n := 0
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(path, ".age") {
			n++
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return n
}

// The backup commands run end to end in a subprocess with synthetic data:
// status without destinations, add (copies), add of a secret (auto push),
// list, restore to a new file, and remove (copies stay).
func TestBackupCommandsEndToEnd(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "vault.age")
	gens := []string{vault.EnvGenerationsDir + "=" + filepath.Join(dir, "generations")}
	pw := practicalPassword + "\n"

	runCLI(t, path, pw+pw, gens, 0, "init")
	runCLI(t, path, practicalValue+"\n"+pw, gens, 0, "add", "example-service/token")

	out, diagnostic := runCLI(t, path, pw, gens, 0, "backup", "status")
	if !strings.Contains(out, "generations: 2") || !strings.Contains(out, "destinations: 0") {
		t.Fatalf("status without destinations:\n%s", out)
	}
	if !strings.Contains(diagnostic, "PC 外の保存先がありません") {
		t.Fatal("status did not warn that nothing is outside this PC")
	}

	usb := filepath.Join(dir, "usb-drive")
	if err := os.Mkdir(usb, 0o700); err != nil {
		t.Fatal(err)
	}
	_, diagnostic = runCLI(t, path, pw, gens, 0, "backup", "add", usb)
	if !strings.Contains(diagnostic, "保存先を登録しました") || !strings.Contains(diagnostic, "未転送 0 件") {
		t.Fatalf("backup add output:\n%s", diagnostic)
	}
	copied := countAgeFiles(t, usb)
	if copied == 0 {
		t.Fatal("backup add copied nothing")
	}

	// A later change is copied automatically (decision D4).
	runCLI(t, path, "second-synthetic-value\n"+pw, gens, 0, "add", "example-service/other")
	if after := countAgeFiles(t, usb); after <= copied {
		t.Fatalf("add did not push automatically: %d -> %d", copied, after)
	}

	out, _ = runCLI(t, path, pw, gens, 0, "backup", "status")
	if !strings.Contains(out, "destinations: 1") || !strings.Contains(out, "  pending: 0") || !strings.Contains(out, "  state: ok") {
		t.Fatalf("status with a destination:\n%s", out)
	}

	out, _ = runCLI(t, path, pw, gens, 0, "backup", "list")
	if !strings.Contains(out, "current") {
		t.Fatalf("list has no current generation:\n%s", out)
	}
	hashes := regexp.MustCompile(`(?m)^([0-9a-f]{12})  .*  saved$`).FindAllStringSubmatch(out, -1)
	if len(hashes) == 0 {
		t.Fatalf("list has no earlier generation:\n%s", out)
	}

	restored := filepath.Join(dir, "restored.age")
	runCLI(t, path, pw, gens, 0, "backup", "restore", hashes[0][1], restored)
	if _, err := vault.Open(restored, practicalPassword); err != nil {
		t.Fatalf("restored file does not open: %v", err)
	}
	runCLI(t, path, pw, gens, 1, "backup", "restore", hashes[0][1], restored)

	out, _ = runCLI(t, path, pw, gens, 0, "backup", "verify", usb)
	if !strings.Contains(out, "current: true") {
		t.Fatalf("verify did not open the current copy:\n%s", out)
	}

	runCLI(t, path, pw, gens, 0, "backup", "remove", usb, "--yes")
	if countAgeFiles(t, usb) == 0 {
		t.Fatal("remove deleted the copies at the destination")
	}
	out, _ = runCLI(t, path, pw, gens, 0, "backup", "status")
	if !strings.Contains(out, "destinations: 0") {
		t.Fatalf("destination still registered after remove:\n%s", out)
	}
}

// Backup output never contains secret values.
func TestBackupOutputHasNoSecretValues(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "vault.age")
	gens := []string{vault.EnvGenerationsDir + "=" + filepath.Join(dir, "generations")}
	pw := practicalPassword + "\n"
	runCLI(t, path, pw+pw, gens, 0, "init")
	runCLI(t, path, practicalValue+"\n"+pw, gens, 0, "add", "example-service/token")
	for _, args := range [][]string{{"backup", "status"}, {"backup", "list"}} {
		out, diagnostic := runCLI(t, path, pw, gens, 0, args...)
		if strings.Contains(out+diagnostic, practicalValue) || strings.Contains(out+diagnostic, practicalPassword) {
			t.Fatalf("%v printed a secret", args)
		}
	}
}
