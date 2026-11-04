package vault

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func newVaultWithDestination(t *testing.T) (*Vault, string, Destination) {
	t.Helper()
	privateGenerations(t)
	path := filepath.Join(t.TempDir(), "vault.age")
	v, err := Create(path, testPassword)
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(t.TempDir(), "usb-drive")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	d, err := v.AddDestination(root)
	if err != nil {
		t.Fatal(err)
	}
	mustSave(t, v)
	return v, path, d
}

func destinationFiles(t *testing.T, v *Vault, d Destination) map[string]bool {
	t.Helper()
	entries, err := os.ReadDir(v.destinationCollection(d))
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]bool{}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), generationExt) {
			out[strings.TrimSuffix(e.Name(), generationExt)] = true
		}
	}
	return out
}

func TestPushCopiesCommittedGenerationsAppendOnly(t *testing.T) {
	v, _, d := newVaultWithDestination(t)
	saveValue(t, v, "one")
	saveValue(t, v, "two")

	results := v.Push(time.Time{})
	if len(results) != 1 || results[0].Err != nil || results[0].Pending != 0 || results[0].Copied == 0 {
		t.Fatalf("push result = %+v", results)
	}
	local, err := v.LocalGenerations()
	if err != nil {
		t.Fatal(err)
	}
	remote := destinationFiles(t, v, d)
	for _, g := range local {
		if g.Committed && !remote[g.Hash] {
			t.Fatalf("committed generation %s was not copied", g.Hash[:12])
		}
	}
	if again := v.Push(time.Time{}); again[0].Copied != 0 || again[0].Err != nil {
		t.Fatalf("second push copied again: %+v", again)
	}

	// Local pruning must not delete anything at the destination.
	withRetention(t, 1, 0, 512<<20)
	dir, _ := v.GenerationsDir()
	ageGenerations(t, dir, time.Hour)
	saveValue(t, v, "three")
	v.Push(time.Time{})
	after := destinationFiles(t, v, d)
	for hash := range remote {
		if !after[hash] {
			t.Fatalf("destination lost generation %s after local pruning", hash[:12])
		}
	}
}

func TestOfflineDestinationIsNotRecreatedAndProtectsGenerations(t *testing.T) {
	v, _, d := newVaultWithDestination(t)
	v.Push(time.Time{})

	// Unplug: the root disappears. Then an empty folder appears at the same path.
	unplugged := d.Path + "-unplugged"
	if err := os.Rename(d.Path, unplugged); err != nil {
		t.Fatal(err)
	}
	if r := v.Push(time.Time{}); !errors.Is(r[0].Err, ErrDestinationOffline) {
		t.Fatalf("push to missing root = %v, want offline", r[0].Err)
	}
	if err := os.Mkdir(d.Path, 0o700); err != nil {
		t.Fatal(err)
	}
	if r := v.Push(time.Time{}); !errors.Is(r[0].Err, ErrDestinationMismatch) {
		t.Fatalf("push to look-alike folder = %v, want mismatch", r[0].Err)
	}
	if _, err := os.Stat(filepath.Join(d.Path, destinationSubdir)); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("push created a backup folder in the look-alike location")
	}

	// While the destination is away, local generations are not pruned.
	withRetention(t, 1, 0, 512<<20)
	dir, _ := v.GenerationsDir()
	before, _ := (&generationStore{dir: dir}).list()
	for i := 0; i < 3; i++ {
		ageGenerations(t, dir, time.Hour)
		saveValue(t, v, "while-away-"+string(rune('a'+i)))
	}
	committed, _ := countGenerations(t, dir)
	if committed < len(before)+3 {
		t.Fatalf("committed local generations = %d, want at least %d while the destination is away", committed, len(before)+3)
	}
}

func TestVerifyDestinationOpensTheLiveCopyAndDetectsDamage(t *testing.T) {
	v, _, d := newVaultWithDestination(t)
	saveValue(t, v, "verified")
	v.Push(time.Time{})

	result, err := v.VerifyDestination(d, testPassword)
	if err != nil {
		t.Fatal(err)
	}
	if !result.MatchesLive || result.Names != v.Count() || result.Damaged != 0 {
		t.Fatalf("verify = %+v", result)
	}
	states := v.DestinationStates()
	if len(states) != 1 || states[0].VerifiedPrefix != v.SnapshotHash()[:12] || states[0].Pending != 0 {
		t.Fatalf("state after verify = %+v", states)
	}

	if _, err := v.VerifyDestination(d, "wrong-password-value"); err == nil {
		t.Fatal("verify succeeded with a wrong password")
	}
	col := v.destinationCollection(d)
	victim := filepath.Join(col, v.SnapshotHash()+generationExt)
	if err := os.WriteFile(victim, []byte("damaged"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := v.VerifyDestination(d, testPassword); err == nil {
		t.Fatal("verify did not report a damaged copy")
	}
}

func TestRestoreWritesANewFileAndLeavesTheVault(t *testing.T) {
	v, path, _ := newVaultWithDestination(t)
	saveValue(t, v, "old-value")
	oldHash := v.SnapshotHash()
	saveValue(t, v, "new-value")
	vaultBefore, _ := os.ReadFile(path)

	src, err := v.FindGeneration(oldHash[:10])
	if err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(t.TempDir(), "restored.age")
	if err := RestoreGeneration(src, dest, testPassword); err != nil {
		t.Fatal(err)
	}
	restored, err := Open(dest, testPassword)
	if err != nil {
		t.Fatal(err)
	}
	if value, _ := restored.Get("example-service/token"); value != "old-value" {
		t.Fatalf("restored value = %q, want old-value", value)
	}
	if err := RestoreGeneration(src, dest, testPassword); err == nil {
		t.Fatal("restore overwrote an existing file")
	}
	if err := RestoreGeneration(src, filepath.Join(t.TempDir(), "x.age"), "wrong-password-value"); err == nil {
		t.Fatal("restore succeeded with a wrong password")
	}
	vaultAfter, _ := os.ReadFile(path)
	if !bytes.Equal(vaultBefore, vaultAfter) {
		t.Fatal("restore changed the current vault")
	}
	if _, err := v.FindGeneration("abc"); err == nil {
		t.Fatal("a too-short prefix was accepted")
	}
}

func TestDestinationRulesAndExport(t *testing.T) {
	v, _, d := newVaultWithDestination(t)
	if _, err := v.AddDestination(d.Path); err == nil {
		t.Fatal("the same destination was registered twice")
	}
	if _, err := v.AddDestination(filepath.Join(t.TempDir(), "missing")); !errors.Is(err, ErrDestinationOffline) {
		t.Fatalf("missing root = %v, want offline", err)
	}
	if d.Volume != "" && !d.SameDisk {
		t.Fatal("a destination on the vault's own disk was not flagged as the same disk")
	}

	exportPath := filepath.Join(t.TempDir(), "copy.age")
	if err := v.Export(exportPath, testPassword); err != nil {
		t.Fatal(err)
	}
	exported, err := Open(exportPath, testPassword)
	if err != nil {
		t.Fatal(err)
	}
	if len(exported.BackupDestinations()) != 0 {
		t.Fatal("export carried backup destinations")
	}

	if r := v.Push(time.Now().Add(-time.Second)); r[0].Copied != 0 || r[0].Pending == 0 {
		t.Fatalf("push past its deadline = %+v, want nothing copied and pending kept", r)
	}

	if err := v.RemoveDestination(d.Path); err != nil {
		t.Fatal(err)
	}
	mustSave(t, v)
	if _, err := os.Stat(v.destinationCollection(d)); err != nil {
		t.Fatal("removing a destination deleted the copies there")
	}
}
