package vault

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// DESIGN.md chapter 12: concurrent writers and pushes. Writes go through
// separate Open handles (the supported way); a second handle pushes while they
// run. The destination must only ever hold complete copies named by their hash,
// and a final push must leave nothing committed behind.
func TestPushWhileWritingKeepsDestinationConsistent(t *testing.T) {
	v, path, d := newVaultWithDestination(t)
	pusher, err := Open(path, testPassword)
	if err != nil {
		t.Fatal(err)
	}

	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
				pusher.Push(time.Time{})
			}
		}
	}()

	for i := 0; i < 6; i++ {
		writer, err := Open(path, testPassword)
		if err != nil {
			t.Fatal(err)
		}
		if err := writer.Set("example-service/token", "concurrent-"+string(rune('a'+i))); err != nil {
			t.Fatal(err)
		}
		if err := writer.Save(testPassword); err != nil && !errors.Is(err, ErrLocked) && !errors.Is(err, ErrConflict) {
			t.Fatalf("save while pushing: %v", err)
		}
	}
	close(stop)
	wg.Wait()

	final, err := Open(path, testPassword)
	if err != nil {
		t.Fatal(err)
	}
	if r := final.Push(time.Time{}); r[0].Err != nil || r[0].Pending != 0 {
		t.Fatalf("final push = %+v", r)
	}
	col := v.destinationCollection(d)
	entries, err := os.ReadDir(col)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, generationExt) {
			continue
		}
		data, err := os.ReadFile(filepath.Join(col, name))
		if err != nil || hashHex(data)+generationExt != name {
			t.Fatalf("destination holds an incomplete or misnamed copy: %s", name)
		}
	}
	local, err := final.LocalGenerations()
	if err != nil {
		t.Fatal(err)
	}
	remote := destinationFiles(t, final, d)
	for _, g := range local {
		if g.Committed && !remote[g.Hash] {
			t.Fatalf("committed generation %s missing at destination after final push", g.Hash[:12])
		}
	}
}

// DESIGN.md chapter 12: clock rollback. Generation identity never uses time;
// with the default 30-day window, files dated in the "future" (the clock moved
// back) and new ones are both kept, and the live vault is never pruned.
func TestClockRollbackDoesNotLoseGenerations(t *testing.T) {
	privateGenerations(t)
	v, err := Create(filepath.Join(t.TempDir(), "vault.age"), testPassword)
	if err != nil {
		t.Fatal(err)
	}
	saveValue(t, v, "before-rollback")
	dir, _ := v.GenerationsDir()
	ageGenerations(t, dir, -48*time.Hour) // existing files now look 2 days in the future
	before, _ := countGenerations(t, dir)

	for i := 0; i < 3; i++ {
		saveValue(t, v, "after-rollback-"+string(rune('a'+i)))
	}
	after, _ := countGenerations(t, dir)
	if after != before+3 {
		t.Fatalf("committed generations = %d, want %d (nothing pruned)", after, before+3)
	}
	gens := readGenerations(t, v, testPassword)
	if _, ok := gens[v.SnapshotHash()]; !ok {
		t.Fatal("the live generation is missing after a clock rollback")
	}
	meta, ok := v.Generation()
	if !ok || meta.Parent == "" {
		t.Fatal("lineage is missing after a clock rollback")
	}
	if _, ok := gens[meta.Parent]; !ok {
		t.Fatal("the parent of the live generation was lost after a clock rollback")
	}
}
