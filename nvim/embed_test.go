package nvim

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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
