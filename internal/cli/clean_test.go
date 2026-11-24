package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/obstinix/PyxForge/internal/runfile"
	"github.com/obstinix/PyxForge/internal/workspace"
)

// TestClean: pyxforge clean removes compiled programs, the state of a workspace whose folder
// is gone and the Neovim configuration release candidates unpacked, and keeps an existing
// workspace's state (#54).
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
	userCache, err := os.UserCacheDir() // the temporary folder above, except on macOS
	if err != nil {
		t.Fatal(err)
	}
	old := filepath.Join(userCache, "PyxForge", "nvim-runtime") // a release candidate's Neovim configuration
	write(filepath.Join(old, "0123456789abcdef", "init.lua"), "-- an older version")

	r := do(t, ".", nil, "clean")
	if r.res.Exit != ExitOK {
		t.Fatalf("clean: %+v", r)
	}
	for _, want := range []string{"Compiled programs (Run Current File): 1 removed", "no longer exist: 1 removed", "The old Neovim editor configuration: 1 removed", "Freed "} {
		if !strings.Contains(r.stdout, want) {
			t.Errorf("output lacks %q:\n%s", want, r.stdout)
		}
	}
	if _, err := os.Stat(program); !os.IsNotExist(err) {
		t.Errorf("the compiled program is still there: %v", err)
	}
	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Errorf("the old Neovim configuration is still there: %v", err)
	}
	if s := st.Load(here); s.Width != 1234 {
		t.Errorf("the existing workspace's state went: %+v", s)
	}
	// Nothing left to do is not an error.
	if r := do(t, ".", nil, "clean"); r.res.Exit != ExitOK || !strings.Contains(r.stdout, "Freed 0 bytes.") {
		t.Errorf("a second clean: %+v", r)
	}
}
