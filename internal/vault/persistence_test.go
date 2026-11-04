package vault

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"filippo.io/age"
)

func TestStaleSnapshotCannotLoseAnotherUpdate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.age")
	a, err := Create(path, testPassword)
	if err != nil {
		t.Fatal(err)
	}
	b, err := Open(path, testPassword)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Set("example-service/first", "synthetic-first"); err != nil {
		t.Fatal(err)
	}
	if err := a.Save(testPassword); err != nil {
		t.Fatal(err)
	}
	before := readCiphertext(t, path)
	if err := b.Set("example-service/second", "synthetic-second"); err != nil {
		t.Fatal(err)
	}
	if err := b.Save(testPassword); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale Save error = %v, want ErrConflict", err)
	}
	if !bytes.Equal(before, readCiphertext(t, path)) {
		t.Fatal("conflict changed ciphertext")
	}
	current, err := Open(path, testPassword)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := current.Get("example-service/first"); err != nil {
		t.Fatal("successful writer's update disappeared")
	}
	if _, err := current.Get("example-service/second"); !errors.Is(err, ErrNotFound) {
		t.Fatal("rejected writer's update was published")
	}
	// A stale deletion must not erase a concurrently added entry either.
	stale, err := Open(path, testPassword)
	if err != nil {
		t.Fatal(err)
	}
	if err := stale.Delete("example-service/first"); err != nil {
		t.Fatal(err)
	}
	if err := current.Set("example-service/third", "synthetic-third"); err != nil {
		t.Fatal(err)
	}
	if err := current.Save(testPassword); err != nil {
		t.Fatal(err)
	}
	if err := stale.Save(testPassword); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale deletion error = %v", err)
	}
	assertNoPersistenceDebris(t, filepath.Dir(path))
}

func TestReplacementFailurePreservesOriginalAndReleasesLock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.age")
	v, err := Create(path, testPassword)
	if err != nil {
		t.Fatal(err)
	}
	before := readCiphertext(t, path)
	if err := v.Set("example-service/token", "synthetic-private-value"); err != nil {
		t.Fatal(err)
	}
	injected := errors.New("synthetic replacement failure")
	err = v.save(testPassword, func(temp, dest string) error {
		raw := readCiphertext(t, temp)
		if bytes.Contains(raw, []byte("synthetic-private-value")) || bytes.Contains(raw, []byte("example-service/token")) {
			t.Fatal("plaintext present in temporary file")
		}
		return injected
	})
	if !errors.Is(err, injected) {
		t.Fatalf("Save error = %v", err)
	}
	if !bytes.Equal(before, readCiphertext(t, path)) {
		t.Fatal("replacement failure changed original")
	}
	assertNoPersistenceDebris(t, filepath.Dir(path))
	if err := v.Save(testPassword); err != nil {
		t.Fatalf("failed save did not release lock: %v", err)
	}
}

func TestSymlinkAliasesShareLockAndSnapshot(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "vault.age")
	v, err := Create(path, testPassword)
	if err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(dir, "alias.age")
	if err := os.Symlink(path, alias); err != nil {
		t.Skipf("symlink privilege unavailable: %v", err)
	}
	other, err := Open(alias, testPassword)
	if err != nil {
		t.Fatal(err)
	}
	if other.Path() != v.Path() {
		t.Fatal("symlink alias did not resolve to same target")
	}
	release, err := lockWriter(v.Path())
	if err != nil {
		t.Fatal(err)
	}
	if err := other.Save(testPassword); !errors.Is(err, ErrLocked) {
		t.Fatalf("alias Save error = %v, want ErrLocked", err)
	}
	if err := release(); err != nil {
		t.Fatal(err)
	}
	if err := v.Save(testPassword); err != nil {
		t.Fatal(err)
	}
	if err := other.Save(testPassword); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale alias Save error = %v", err)
	}
	info, err := os.Lstat(alias)
	if err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatal("saving through alias replaced the symlink")
	}
	parentAlias := filepath.Join(dir, "directory-alias")
	if err := os.Symlink(dir, parentAlias); err != nil {
		t.Fatal(err)
	}
	if _, err := Create(filepath.Join(parentAlias, "vault.age"), testPassword); err == nil {
		t.Fatal("directory alias overwrote an existing vault")
	}
	if err := os.Symlink(filepath.Join(dir, "missing.age"), filepath.Join(dir, "dangling.age")); err != nil {
		t.Fatal(err)
	}
	if _, err := Create(filepath.Join(dir, "dangling.age"), testPassword); err == nil {
		t.Fatal("dangling symlink accepted for creation")
	}
	assertNoPersistenceDebris(t, dir)
}

func TestOpenRejectsInvalidJSONVersionAndUnauthenticatedEOF(t *testing.T) {
	valid := `{"version":1,"secrets":{"example-service/token":"synthetic-value"}}`
	for _, test := range []struct {
		name  string
		plain string
	}{
		{"zero version", `{"version":0,"secrets":{}}`},
		{"negative version", `{"version":-1,"secrets":{}}`},
		{"missing version", `{"secrets":{}}`},
		{"null version", `{"version":null,"secrets":{}}`},
		{"missing secrets", `{"version":1}`},
		{"null secrets", `{"version":1,"secrets":null}`},
		{"duplicate version", `{"version":0,"version":1,"secrets":{}}`},
		{"null value", `{"version":1,"secrets":{"example-service/token":null}}`},
		{"number value", `{"version":1,"secrets":{"example-service/token":42}}`},
		{"duplicate secret", `{"version":1,"secrets":{"example-service/token":"one","example-service/token":"two"}}`},
		{"second JSON", valid + ` {"version":1,"secrets":{}}`},
		{"trailing garbage", valid + " synthetic-private-value"},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "vault.age")
			writeEncryptedPlaintext(t, path, test.plain)
			before := readCiphertext(t, path)
			if _, err := Open(path, testPassword); err == nil {
				t.Fatal("invalid document accepted")
			} else if strings.Contains(err.Error(), "synthetic-private-value") {
				t.Fatal("diagnostic contains plaintext")
			}
			if !bytes.Equal(before, readCiphertext(t, path)) {
				t.Fatal("rejected Open modified ciphertext")
			}
		})
	}
	path := filepath.Join(t.TempDir(), "vault.age")
	writeEncryptedPlaintext(t, path, valid+strings.Repeat(" ", 3*64*1024))
	original := readCiphertext(t, path)
	if _, err := Open(path, testPassword); err != nil {
		t.Fatalf("authenticated whitespace rejected: %v", err)
	}
	for _, mutation := range []string{"truncate", "tamper", "append"} {
		t.Run(mutation, func(t *testing.T) {
			raw := append([]byte(nil), original...)
			switch mutation {
			case "truncate":
				raw = raw[:len(raw)-16]
			case "tamper":
				raw[len(raw)-1] ^= 1
			case "append":
				raw = append(raw, 1)
			}
			if err := os.WriteFile(path, raw, 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := Open(path, testPassword); err == nil {
				t.Fatal("damaged authenticated stream accepted")
			}
		})
	}
}

func TestUnknownMetadataSurvivesSaveAndExport(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "vault.age")
	writeEncryptedPlaintext(t, path, `{"version":1,"secrets":{},"future_metadata":{"example":true}}`)
	v, err := Open(path, testPassword)
	if err != nil {
		t.Fatal(err)
	}
	if err := v.Save(testPassword); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(dir, "backup.age")
	if err := v.Export(dest, testPassword); err != nil {
		t.Fatal(err)
	}
	for _, file := range []string{path, dest} {
		reopened, err := Open(file, testPassword)
		if err != nil {
			t.Fatal(err)
		}
		var metadata map[string]bool
		if err := json.Unmarshal(reopened.doc.Extra["future_metadata"], &metadata); err != nil || !metadata["example"] {
			t.Fatal("optional metadata lost")
		}
		if reopened.VaultID() != v.VaultID() {
			t.Fatal("logical vault ID changed")
		}
	}
	before := readCiphertext(t, dest)
	if err := v.Export(dest, testPassword); err == nil {
		t.Fatal("export overwrote existing destination")
	}
	if !bytes.Equal(before, readCiphertext(t, dest)) {
		t.Fatal("failed export changed destination")
	}
	assertNoPersistenceDebris(t, dir)
}

func TestConcurrentInitProcessesPublishExactlyOnce(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "vault.age")
	start := filepath.Join(dir, "start")
	commands := make([]*exec.Cmd, 2)
	outputs := make([]bytes.Buffer, 2)
	for i := range commands {
		commands[i] = vaultHelper(t, "create", path, start)
		commands[i].Stdout = &outputs[i]
		commands[i].Stderr = &outputs[i]
		if err := commands[i].Start(); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = commands[i].Process.Kill() })
	}
	if err := os.WriteFile(start, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	successes := 0
	for i, cmd := range commands {
		if err := cmd.Wait(); err == nil {
			successes++
		} else if cmd.ProcessState.ExitCode() != 20 {
			t.Fatalf("unexpected helper failure: %v, %s", err, &outputs[i])
		}
	}
	if successes != 1 {
		t.Fatalf("successful creators = %d, want 1", successes)
	}
	if _, err := Open(path, testPassword); err != nil {
		t.Fatal(err)
	}
	assertNoPersistenceDebris(t, dir)
}

func TestCrashedWriterLockIsNeverStolen(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "vault.age")
	v, err := Create(path, testPassword)
	if err != nil {
		t.Fatal(err)
	}
	before := readCiphertext(t, path)
	ready := filepath.Join(dir, "ready")
	cmd := vaultHelper(t, "lock", v.Path(), ready)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill() })
	waitForFile(t, ready)
	if err := cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = cmd.Wait()
	if err := v.Save(testPassword); !errors.Is(err, ErrLocked) {
		t.Fatalf("crashed-owner lock error = %v", err)
	}
	if !bytes.Equal(before, readCiphertext(t, path)) {
		t.Fatal("lock conflict changed ciphertext")
	}
	// Recovery is explicit and only after the helper has terminated.
	if err := os.Remove(v.Path() + ".lock"); err != nil {
		t.Fatal(err)
	}
	if err := v.Save(testPassword); err != nil {
		t.Fatal(err)
	}
}

func vaultHelper(t *testing.T, mode, path, signal string) *exec.Cmd {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return exec.Command(executable, "-test.run=^TestVaultPersistenceHelper$", "--", mode, path, signal)
}

func TestVaultPersistenceHelper(t *testing.T) {
	var args []string
	for i, arg := range os.Args {
		if arg == "--" {
			args = os.Args[i+1:]
			break
		}
	}
	if len(args) != 3 {
		return
	}
	switch args[0] {
	case "create":
		waitForFile(t, args[2])
		if _, err := Create(args[1], testPassword); err != nil {
			os.Exit(20)
		}
	case "stale":
		v, err := Open(args[1], testPassword)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(args[2]+".ready", nil, 0o600); err != nil {
			t.Fatal(err)
		}
		waitForFile(t, args[2])
		if err := v.Set("example-service/second", "synthetic-second"); err != nil {
			t.Fatal(err)
		}
		if err := v.Save(testPassword); !errors.Is(err, ErrConflict) {
			t.Fatal("stale process was not rejected with ErrConflict")
		}
	case "lock":
		if _, err := lockWriter(args[1]); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(args[2], nil, 0o600); err != nil {
			t.Fatal(err)
		}
		for {
			time.Sleep(time.Second)
		}
	default:
		t.Fatal("invalid helper operation")
	}
}

func waitForFile(t *testing.T, path string) {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("helper synchronization timed out")
}

func writeEncryptedPlaintext(t *testing.T, path, plain string) {
	t.Helper()
	recipient, err := age.NewScryptRecipient(testPassword)
	if err != nil {
		t.Fatal(err)
	}
	var raw bytes.Buffer
	w, err := age.Encrypt(&raw, recipient)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fmt.Fprint(w, plain); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
}

func readCiphertext(t *testing.T, path string) []byte {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func assertNoPersistenceDebris(t *testing.T, dir string) {
	t.Helper()
	files, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		if strings.HasPrefix(file.Name(), ".kinko-") || strings.HasSuffix(file.Name(), ".lock") {
			t.Fatalf("persistence debris left behind: %s", file.Name())
		}
	}
}

func TestStaleProcessCannotOverwriteAnotherProcess(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "vault.age")
	v, err := Create(path, testPassword)
	if err != nil {
		t.Fatal(err)
	}
	signal := filepath.Join(dir, "save")
	cmd := vaultHelper(t, "stale", path, signal)
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill() })
	waitForFile(t, signal+".ready")
	if err := v.Set("example-service/first", "synthetic-first"); err != nil {
		t.Fatal(err)
	}
	if err := v.Save(testPassword); err != nil {
		t.Fatal(err)
	}
	before := readCiphertext(t, path)
	if err := os.WriteFile(signal, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Wait(); err != nil {
		t.Fatalf("stale process check failed: %v, %s", err, &output)
	}
	if !bytes.Equal(before, readCiphertext(t, path)) {
		t.Fatal("stale process changed committed ciphertext")
	}
	assertNoPersistenceDebris(t, dir)
}
