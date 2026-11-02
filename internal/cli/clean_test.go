package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/obstinix/PyxForge/internal/runfile"
	"github.com/obstinix/PyxForge/internal/workspace"
	"github.com/obstinix/PyxForge/nvim"
)

// TestClean: pyxforge clean removes compiled programs, the state of a workspace whose folder
// is gone and another version's editor configuration, and keeps an existing workspace's state
// and this version's configuration (#54).
func TestClean(t *testing.T) {
	cache, config := t.TempDir(), t.TempDir()
	t.Setenv("LOCALAPPDATA", cache) // os.UserCacheDir on Windows
	t.Setenv("XDG_CACHE_HOME", cache)
	t.Setenv("APPDATA", config) // os.UserConfigDir on Windows
	t.Setenv("XDG_CONFIG_HOME", config)

	write := func(p, body string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	program := filepath.Join(runfile.CacheDir(), "0123abcd", "hello.exe")
	write(program, strings.Repeat("x", 2048))
	st, err := workspace.DefaultStateStore()
	if err != nil {
		t.Fatal(err)
	}
	here, gone := t.TempDir(), filepath.Join(t.TempDir(), "deleted")
	for _, root := range []string{here, gone} {
		if err := st.Save(workspace.State{Root: root, Width: 1234}); err != nil {
			t.Fatal(err)
		}
	}
	base, _ := nvim.DefaultBase()
	if _, err := nvim.Install(base); err != nil {
		t.Fatal(err)
	}
	old := filepath.Join(base, "0123456789abcdef")
	write(filepath.Join(old, "init.lua"), "-- an older version")

	r := do(t, ".", nil, "clean")
	if r.res.Exit != ExitOK {
		t.Fatalf("clean: %+v", r)
	}
	for _, want := range []string{"Compiled programs (Run Current File): 1 removed", "no longer exist: 1 removed", "editor configurations: 1 removed", "Freed "} {
		if !strings.Contains(r.stdout, want) {
			t.Errorf("output lacks %q:\n%s", want, r.stdout)
		}
	}
	if _, err := os.Stat(program); !os.IsNotExist(err) {
		t.Errorf("the compiled program is still there: %v", err)
	}
	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Errorf("the older configuration is still there: %v", err)
	}
	if _, err := os.Stat(filepath.Join(base, nvim.Hash(), "init.lua")); err != nil {
		t.Errorf("this version's configuration went: %v", err)
	}
	if s := st.Load(here); s.Width != 1234 {
		t.Errorf("the existing workspace's state went: %+v", s)
	}
	// Nothing left to do is not an error.
	if r := do(t, ".", nil, "clean"); r.res.Exit != ExitOK || !strings.Contains(r.stdout, "Freed 0 bytes.") {
		t.Errorf("a second clean: %+v", r)
	}
}
