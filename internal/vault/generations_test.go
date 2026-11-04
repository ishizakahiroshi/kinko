package vault

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"filippo.io/age"
)

// privateGenerations gives one test its own generations root.
func privateGenerations(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv(EnvGenerationsDir, dir)
	return dir
}

type storedGeneration struct {
	hash      string
	committed bool
	vault     *Vault
}

func readGenerations(t *testing.T, v *Vault, password string) map[string]storedGeneration {
	t.Helper()
	dir, err := v.GenerationsDir()
	if err != nil || dir == "" {
		t.Fatalf("GenerationsDir: %q %v", dir, err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	out := map[string]storedGeneration{}
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, generationExt) {
			continue
		}
		hash := strings.TrimSuffix(name, generationExt)
		raw, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		if hashHex(raw) != hash {
			t.Fatalf("generation %s is not named by its hash", name)
		}
		opened, err := Open(filepath.Join(dir, name), password)
		if err != nil {
			t.Fatalf("open generation %s: %v", name, err)
		}
		_, markerErr := os.Stat(filepath.Join(dir, hash+committedExt))
		out[hash] = storedGeneration{hash: hash, committed: markerErr == nil, vault: opened}
	}
	return out
}

func mustSave(t *testing.T, v *Vault) {
	t.Helper()
	if err := v.Save(testPassword); err != nil {
		t.Fatalf("Save: %v", err)
	}
}

// Every step of create → first add → update → delete stays recoverable, and the
// lineage inside each generation links back to the empty G0.
func TestGenerationsKeepEveryStepAndLinkLineage(t *testing.T) {
	privateGenerations(t)
	path := filepath.Join(t.TempDir(), "vault.age")
	v, err := Create(path, testPassword)
	if err != nil {
		t.Fatal(err)
	}
	if err := v.Set("example-service/token", "first-value"); err != nil {
		t.Fatal(err)
	}
	mustSave(t, v)
	if err := v.Set("example-service/token", "second-value"); err != nil {
		t.Fatal(err)
	}
	mustSave(t, v)
	if err := v.Delete("example-service/token"); err != nil {
		t.Fatal(err)
	}
	mustSave(t, v)

	gens := readGenerations(t, v, testPassword)
	if len(gens) != 4 {
		t.Fatalf("generations = %d, want 4 (G0, first, second, deleted)", len(gens))
	}
	seen := map[string]bool{}
	for _, g := range gens {
		if !g.committed {
			t.Fatalf("generation %s is not marked committed after successful saves", g.hash[:12])
		}
		value, err := g.vault.Get("example-service/token")
		switch {
		case errors.Is(err, ErrNotFound):
			seen["absent"] = true
		case err == nil:
			seen[value] = true
		default:
			t.Fatal(err)
		}
	}
	for _, want := range []string{"absent", "first-value", "second-value"} {
		if !seen[want] {
			t.Errorf("no generation holds %q", want)
		}
	}

	// Walk the lineage from the live vault back to G0.
	current, ok := v.Generation()
	if !ok {
		t.Fatal("live vault has no generation metadata")
	}
	steps := 0
	for current.Parent != "" {
		parent, ok := gens[current.Parent]
		if !ok {
			t.Fatalf("parent %s is not a stored generation", current.Parent[:12])
		}
		current, ok = parent.vault.Generation()
		if !ok {
			t.Fatal("stored generation has no metadata")
		}
		steps++
	}
	if steps != 3 {
		t.Fatalf("lineage steps = %d, want 3", steps)
	}
	if gens[v.SnapshotHash()].vault == nil {
		t.Fatal("the live vault is not stored as a generation")
	}
}

// If a generation cannot be stored, the vault must not change.
func TestGenerationStoreFailureLeavesVaultUnchanged(t *testing.T) {
	root := privateGenerations(t)
	path := filepath.Join(t.TempDir(), "vault.age")
	v, err := Create(path, testPassword)
	if err != nil {
		t.Fatal(err)
	}
	if err := v.Set("example-service/token", "kept-value"); err != nil {
		t.Fatal(err)
	}
	mustSave(t, v)
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	blocked := filepath.Join(root, "not-a-directory")
	if err := os.WriteFile(blocked, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(EnvGenerationsDir, blocked)
	if err := v.Set("example-service/token", "lost-value"); err != nil {
		t.Fatal(err)
	}
	if err := v.Save(testPassword); err == nil {
		t.Fatal("Save succeeded without a place for generations")
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("vault changed although generations could not be stored")
	}
}

// A save that stops before replacing the vault leaves an uncommitted candidate;
// a save that replaced the vault but lost its marker is repaired by the next save.
func TestInterruptedSavesAreReconciled(t *testing.T) {
	privateGenerations(t)
	path := filepath.Join(t.TempDir(), "vault.age")
	v, err := Create(path, testPassword)
	if err != nil {
		t.Fatal(err)
	}
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	if err := v.Set("example-service/token", "never-published"); err != nil {
		t.Fatal(err)
	}
	failReplace := func(string, string) error { return errors.New("injected replace failure") }
	if err := v.save(testPassword, failReplace); err == nil {
		t.Fatal("save with a failing replace succeeded")
	}
	now, _ := os.ReadFile(path)
	if !bytes.Equal(original, now) {
		t.Fatal("vault changed although replace failed")
	}
	candidates := 0
	for _, g := range readGenerations(t, v, testPassword) {
		if !g.committed {
			candidates++
			if value, err := g.vault.Get("example-service/token"); err != nil || value != "never-published" {
				t.Fatal("uncommitted candidate does not hold the interrupted change")
			}
		}
	}
	if candidates != 1 {
		t.Fatalf("uncommitted candidates = %d, want 1", candidates)
	}

	// Replace succeeds but the committed marker is lost (crash right after replace).
	reopened, err := Open(path, testPassword)
	if err != nil {
		t.Fatal(err)
	}
	if err := reopened.Set("example-service/token", "published"); err != nil {
		t.Fatal(err)
	}
	mustSave(t, reopened)
	dir, _ := reopened.GenerationsDir()
	marker := filepath.Join(dir, reopened.SnapshotHash()+committedExt)
	if err := os.Remove(marker); err != nil {
		t.Fatal(err)
	}
	next, err := Open(path, testPassword)
	if err != nil {
		t.Fatal(err)
	}
	if err := next.Set("example-service/other", "x"); err != nil {
		t.Fatal(err)
	}
	mustSave(t, next)
	if _, err := os.Stat(marker); err != nil {
		t.Fatal("next save did not record the previously published generation as committed")
	}
}

// export is a separate copy: no generations, no inherited lineage.
func TestExportCreatesNoGenerations(t *testing.T) {
	privateGenerations(t)
	path := filepath.Join(t.TempDir(), "vault.age")
	v, err := Create(path, testPassword)
	if err != nil {
		t.Fatal(err)
	}
	if err := v.Set("example-service/token", "value"); err != nil {
		t.Fatal(err)
	}
	mustSave(t, v)
	count := len(readGenerations(t, v, testPassword))

	exportPath := filepath.Join(t.TempDir(), "backup.age")
	if err := v.Export(exportPath, testPassword); err != nil {
		t.Fatal(err)
	}
	if got := len(readGenerations(t, v, testPassword)); got != count {
		t.Fatalf("export added generations: %d -> %d", count, got)
	}
	exported, err := Open(exportPath, testPassword)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := exported.Generation(); ok {
		t.Fatal("export carried the source lineage")
	}
	if _, ok := v.Generation(); !ok {
		t.Fatal("export removed lineage from the source vault in memory")
	}
}

// A vault from before vault_id and generations keeps its original bytes as the
// first generation when it is changed.
func TestLegacyVaultBaselineIsStored(t *testing.T) {
	privateGenerations(t)
	path := filepath.Join(t.TempDir(), "vault.age")
	recipient, err := age.NewScryptRecipient(testPassword)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	w, err := age.Encrypt(&buf, recipient)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte(`{"version":1,"secrets":{"example-service/token":"legacy-value"}}`)); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	legacyHash := hashHex(buf.Bytes())

	v, err := Open(path, testPassword)
	if err != nil {
		t.Fatal(err)
	}
	if err := v.Set("example-service/token", "new-value"); err != nil {
		t.Fatal(err)
	}
	mustSave(t, v)
	gens := readGenerations(t, v, testPassword)
	baseline, ok := gens[legacyHash]
	if !ok || !baseline.committed {
		t.Fatal("legacy ciphertext was not kept as a committed generation")
	}
	if value, _ := baseline.vault.Get("example-service/token"); value != "legacy-value" {
		t.Fatal("legacy generation does not hold the old value")
	}
	meta, ok := v.Generation()
	if !ok || meta.Parent != legacyHash {
		t.Fatal("new generation does not point at the legacy baseline")
	}
}

// Tests lower the scrypt work factor for speed; production (0) must keep age's
// default, which age v1.3.1 writes as "18" in the scrypt stanza.
func TestProductionSaveKeepsDefaultScryptWorkFactor(t *testing.T) {
	privateGenerations(t)
	old := testScryptWorkFactor
	testScryptWorkFactor = 0
	t.Cleanup(func() { testScryptWorkFactor = old })
	path := filepath.Join(t.TempDir(), "vault.age")
	if _, err := Create(path, testPassword); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	header := string(raw[:bytes.Index(raw, []byte("\n---"))])
	var stanza string
	for _, line := range strings.Split(header, "\n") {
		if strings.HasPrefix(line, "-> scrypt ") {
			stanza = line
		}
	}
	fields := strings.Fields(stanza)
	if len(fields) != 4 || fields[3] != "18" {
		t.Fatalf("production scrypt stanza = %q, want work factor 18", stanza)
	}
}

// The collection directory must not reveal vault_id, and the default root must
// sit outside the default vault directory.
func TestGenerationLocationIsOpaqueAndOutsideVaultDirectory(t *testing.T) {
	privateGenerations(t)
	v, err := Create(filepath.Join(t.TempDir(), "vault.age"), testPassword)
	if err != nil {
		t.Fatal(err)
	}
	dir, err := v.GenerationsDir()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.ToLower(dir), strings.ToLower(v.VaultID())) {
		t.Fatal("generation directory name contains vault_id")
	}

	t.Setenv(EnvGenerationsDir, "")
	root, err := GenerationsRoot()
	if err != nil {
		t.Skipf("no user config dir on this runner: %v", err)
	}
	config, _ := os.UserConfigDir()
	defaultVaultDir := filepath.Join(config, "kinko")
	if rel, err := filepath.Rel(defaultVaultDir, root); err == nil && !strings.HasPrefix(rel, "..") {
		t.Fatalf("default generations root %q is inside the default vault directory", root)
	}
}
