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

// withRetention shrinks the retention limits for one test.
func withRetention(t *testing.T, count int, age time.Duration, capacity int64) {
	t.Helper()
	oldCount, oldAge, oldCapacity := retainCount, retainAge, generationCapacity
	retainCount, retainAge, generationCapacity = count, age, capacity
	t.Cleanup(func() { retainCount, retainAge, generationCapacity = oldCount, oldAge, oldCapacity })
}

// ageGenerations moves every generation file back by d so later saves are
// strictly newer even on coarse filesystem clocks.
func ageGenerations(t *testing.T, dir string, d time.Duration) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), generationExt) {
			continue
		}
		path := filepath.Join(dir, e.Name())
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		mod := info.ModTime().Add(-d)
		if err := os.Chtimes(path, mod, mod); err != nil {
			t.Fatal(err)
		}
	}
}

func countGenerations(t *testing.T, dir string) (committed, candidates int) {
	t.Helper()
	store := &generationStore{dir: dir}
	files, err := store.list()
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		if f.committed {
			committed++
		} else {
			candidates++
		}
	}
	return committed, candidates
}

func saveValue(t *testing.T, v *Vault, value string) {
	t.Helper()
	if err := v.Set("example-service/token", value); err != nil {
		t.Fatal(err)
	}
	mustSave(t, v)
}

func TestRetentionKeepsNewestCountWhenOld(t *testing.T) {
	privateGenerations(t)
	withRetention(t, 3, 0, 512<<20)
	v, err := Create(filepath.Join(t.TempDir(), "vault.age"), testPassword)
	if err != nil {
		t.Fatal(err)
	}
	dir, _ := v.GenerationsDir()
	for i := 0; i < 6; i++ {
		ageGenerations(t, dir, time.Hour)
		saveValue(t, v, "value-"+string(rune('a'+i)))
	}
	committed, _ := countGenerations(t, dir)
	if committed != 3 {
		t.Fatalf("committed generations = %d, want 3", committed)
	}
	gens := readGenerations(t, v, testPassword)
	if _, ok := gens[v.SnapshotHash()]; !ok {
		t.Fatal("the live generation was pruned")
	}
	seen := map[string]bool{}
	for _, g := range gens {
		value, _ := g.vault.Get("example-service/token")
		seen[value] = true
	}
	for _, want := range []string{"value-d", "value-e", "value-f"} {
		if !seen[want] {
			t.Errorf("newest generation %q was pruned", want)
		}
	}
}

func TestRetentionKeepsEverythingWithinAge(t *testing.T) {
	privateGenerations(t)
	withRetention(t, 1, 30*24*time.Hour, 512<<20)
	v, err := Create(filepath.Join(t.TempDir(), "vault.age"), testPassword)
	if err != nil {
		t.Fatal(err)
	}
	dir, _ := v.GenerationsDir()
	for i := 0; i < 4; i++ {
		ageGenerations(t, dir, time.Hour)
		saveValue(t, v, "recent-"+string(rune('a'+i)))
	}
	if committed, _ := countGenerations(t, dir); committed != 5 {
		t.Fatalf("committed generations within 30 days = %d, want 5", committed)
	}
	ageGenerations(t, dir, 31*24*time.Hour)
	saveValue(t, v, "today")
	// Live before this save (aged, but it is the newest by count) and the new live one remain.
	if committed, _ := countGenerations(t, dir); committed != 1 {
		t.Fatalf("committed generations after aging = %d, want 1 (the live one)", committed)
	}
}

func TestRetentionNeverRemovesCandidatesOrPinned(t *testing.T) {
	privateGenerations(t)
	withRetention(t, 1, 0, 512<<20)
	path := filepath.Join(t.TempDir(), "vault.age")
	v, err := Create(path, testPassword)
	if err != nil {
		t.Fatal(err)
	}
	dir, _ := v.GenerationsDir()
	pinned := v.SnapshotHash()
	if err := os.WriteFile(filepath.Join(dir, pinned+pinExt), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := v.Set("example-service/token", "interrupted"); err != nil {
		t.Fatal(err)
	}
	if err := v.save(testPassword, func(string, string) error { return errors.New("injected") }); err == nil {
		t.Fatal("injected replace failure did not fail")
	}
	reopened, err := Open(path, testPassword)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 4; i++ {
		ageGenerations(t, dir, time.Hour)
		saveValue(t, reopened, "later-"+string(rune('a'+i)))
	}
	if _, err := os.Stat(filepath.Join(dir, pinned+generationExt)); err != nil {
		t.Fatal("pinned generation was pruned")
	}
	if _, candidates := countGenerations(t, dir); candidates != 1 {
		t.Fatalf("uncommitted candidates = %d, want 1", candidates)
	}
}

func TestCapacityStopsWritesButKeepsReads(t *testing.T) {
	privateGenerations(t)
	path := filepath.Join(t.TempDir(), "vault.age")
	v, err := Create(path, testPassword)
	if err != nil {
		t.Fatal(err)
	}
	saveValue(t, v, "kept")
	dir, _ := v.GenerationsDir()
	used, err := (&generationStore{dir: dir}).usage()
	if err != nil {
		t.Fatal(err)
	}
	// Everything is protected by count, and there is no room for one more generation.
	withRetention(t, 1000, 0, used+10)
	before, _ := os.ReadFile(path)
	if err := v.Set("example-service/token", "blocked"); err != nil {
		t.Fatal(err)
	}
	if err := v.Save(testPassword); !errors.Is(err, ErrGenerationsFull) {
		t.Fatalf("Save over capacity = %v, want ErrGenerationsFull", err)
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(before, after) {
		t.Fatal("vault changed although generations were full")
	}
	reopened, err := Open(path, testPassword)
	if err != nil {
		t.Fatal(err)
	}
	if value, err := reopened.Get("example-service/token"); err != nil || value != "kept" {
		t.Fatal("reading stopped working when generations were full")
	}
}

func TestNearlyFullGenerationsLeaveANotice(t *testing.T) {
	privateGenerations(t)
	v, err := Create(filepath.Join(t.TempDir(), "vault.age"), testPassword)
	if err != nil {
		t.Fatal(err)
	}
	if len(v.SaveNotices()) != 0 {
		t.Fatal("unexpected notice with plenty of room")
	}
	dir, _ := v.GenerationsDir()
	used, _ := (&generationStore{dir: dir}).usage()
	// After the save two generations of about `used` bytes exist: about 83% of 2.4×used.
	withRetention(t, 1000, 30*24*time.Hour, int64(float64(used)*2.4))
	saveValue(t, v, "x")
	notices := v.SaveNotices()
	if len(notices) == 0 {
		t.Fatal("no notice when the generations area is above 80%")
	}
	for _, n := range notices {
		if strings.Contains(n, "x") && strings.Contains(n, "example-service") {
			t.Fatal("notice contains secret names or values")
		}
	}
}
