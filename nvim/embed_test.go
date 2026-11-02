package nvim

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestInstallWritesOnceAndNeverOverwrites(t *testing.T) {
	base := filepath.Join(t.TempDir(), "runtime")
	init, err := Install(base)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(filepath.Dir(init)) != Hash() {
		t.Errorf("init.lua is not in the content-addressed folder: %s", init)
	}
	for _, f := range []string{"init.lua", "lua/pyxforge/lsp.lua", "lua/pyxforge/clipboard.lua", "pyxforge-lock.json"} {
		if _, err := os.Stat(filepath.Join(filepath.Dir(init), filepath.FromSlash(f))); err != nil {
			t.Errorf("%s not installed: %v", f, err)
		}
	}
	// A file already there is left alone, even if it differs.
	if err := os.WriteFile(init, []byte("-- edited\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	again, err := Install(base)
	if err != nil || again != init {
		t.Fatalf("second install: %q, %v", again, err)
	}
	if b, _ := os.ReadFile(init); string(b) != "-- edited\n" {
		t.Error("Install overwrote an existing file")
	}
	entries, _ := os.ReadDir(base)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".tmp-") {
			t.Errorf("staging folder left behind: %s", e.Name())
		}
	}
}

func TestHashIsStable(t *testing.T) {
	first, second := Hash(), Hash()
	if first != second || len(first) != 16 {
		t.Errorf("Hash() = %q then %q", first, second)
	}
}

func TestLock(t *testing.T) {
	l, err := Lock()
	if err != nil {
		t.Fatal(err)
	}
	if len(l.Plugins) == 0 || len(l.Parsers) == 0 {
		t.Fatalf("lockfile = %+v", l)
	}
	for _, want := range []string{"c", "nasm", "go", "rust", "linkerscript"} {
		found := false
		for _, p := range l.Parsers {
			found = found || p == want
		}
		if !found {
			t.Errorf("parser %s is not in the lockfile", want)
		}
	}
}

// TestPrune: another version's configuration and an interrupted install's folder go once they
// have been unused long enough; this version's, a recently used one and other folders stay
// (#54).
func TestPrune(t *testing.T) {
	base := t.TempDir()
	if _, err := Install(base); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-48 * time.Hour)
	mk := func(name string, at time.Time) {
		p := filepath.Join(base, name)
		if err := os.MkdirAll(filepath.Join(p, "lua"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(p, at, at); err != nil {
			t.Fatal(err)
		}
	}
	mk("0123456789abcdef", old)        // an older version's
	mk("fedcba9876543210", time.Now()) // a version used today
	mk(".tmp-123", old)                // an interrupted install
	mk("notes", old)                   // not PyxForge's to remove
	if err := os.Chtimes(filepath.Join(base, Hash()), old, old); err != nil {
		t.Fatal(err)
	}
	removed, err := Prune(base, 24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, p := range removed {
		names = append(names, filepath.Base(p))
	}
	slices.Sort(names)
	if strings.Join(names, " ") != ".tmp-123 0123456789abcdef" {
		t.Errorf("removed %v", names)
	}
	left, _ := os.ReadDir(base)
	var kept []string
	for _, e := range left {
		kept = append(kept, e.Name())
	}
	if !slices.Contains(kept, Hash()) || !slices.Contains(kept, "fedcba9876543210") || !slices.Contains(kept, "notes") {
		t.Errorf("kept %v", kept)
	}
	// Install marks this version's folder used again.
	if _, err := Install(base); err != nil {
		t.Fatal(err)
	}
	if fi, _ := os.Stat(filepath.Join(base, Hash())); time.Since(fi.ModTime()) > time.Hour {
		t.Errorf("Install did not mark its folder used: %v", fi.ModTime())
	}
	if removed, err := Prune(filepath.Join(base, "missing"), 0); err != nil || len(removed) != 0 {
		t.Errorf("a missing base: %v, %v", removed, err)
	}
}
