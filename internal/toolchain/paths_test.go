package toolchain

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestLookupOrder: a path set in Doctor wins, then a bundled copy, then PATH, then the
// install folders; a path set in Doctor that no longer exists is reported, not replaced.
func TestLookupOrder(t *testing.T) {
	bundle := filepath.Join(programFiles, "PyxForge")
	bundled := filepath.Join(bundle, "nvim", "bin", "nvim")
	chosen := filepath.Join(programFiles, "Neovim", "bin", "nvim")
	d := fake(map[string]string{"nvim": "NVIM v0.11.0", bundled: "NVIM v0.12.6", chosen: "NVIM v0.10.4"})
	nvim := Tools[0]
	if nvim.ID != "nvim" {
		t.Fatal("Tools[0] is not Neovim")
	}
	check := func(what, wantPath, wantSource, wantVersion string) {
		t.Helper()
		st := d.Detect(context.Background(), []Tool{nvim})[0]
		if st.Path != wantPath || st.Source != wantSource || st.Version != wantVersion {
			t.Errorf("%s: %q from %q, version %q; want %q from %q, %q", what, st.Path, st.Source, st.Version, wantPath, wantSource, wantVersion)
		}
	}
	check("PATH only", "nvim", SourcePath, "0.11.0")
	d.BundleDir = bundle
	check("bundled", bundled, SourceBundled, "0.12.6")
	d.Overrides = func() Overrides { return Overrides{"nvim": chosen} }
	check("set in Doctor", chosen, SourceConfigured, "0.10.4")

	d.Overrides = func() Overrides { return Overrides{"nvim": filepath.Join(programFiles, "gone", "nvim")} }
	st := d.Detect(context.Background(), []Tool{nvim})[0]
	if st.Found() || st.Source != SourceConfigured || st.Err == nil || !strings.Contains(st.Err.Error(), "set in Doctor") {
		t.Errorf("a vanished path was not reported: %+v", st)
	}
}

func TestNeovimInstalledOutsidePath(t *testing.T) {
	for _, variable := range []string{"ProgramFiles", "LOCALAPPDATA"} {
		t.Run(variable, func(t *testing.T) {
			root := t.TempDir()
			dir := filepath.Join(root, "Neovim", "bin")
			if variable == "LOCALAPPDATA" {
				dir = filepath.Join(root, "Programs", "Neovim", "bin")
			}
			path := filepath.Join(dir, "nvim")
			d := fake(map[string]string{path: "NVIM v0.12.6"})
			d.Getenv = func(key string) string {
				if key == variable {
					return root
				}
				return ""
			}
			st := d.Detect(context.Background(), []Tool{Tools[0]})[0]
			if !st.Found() || st.Path != path || st.Source != SourceFolder {
				t.Fatalf("installed Neovim not discovered without PATH: %+v", st)
			}
		})
	}
}

// TestOverridesPersist: SaveOverride writes the user's settings file, LoadOverrides reads it
// back, and clearing a path removes it.
func TestOverridesPersist(t *testing.T) {
	cfg := t.TempDir()
	t.Setenv("APPDATA", cfg)         // Windows
	t.Setenv("XDG_CONFIG_HOME", cfg) // Linux
	t.Setenv("HOME", cfg)            // macOS
	t.Cleanup(func() { SetOverrides(nil) })
	SetOverrides(nil)

	gdb := filepath.Join(cfg, "tools dir ü", "gdb.exe")
	if err := SaveOverride("gdb", gdb); err != nil {
		t.Fatal(err)
	}
	if err := SaveOverride("git", "relative/git"); err == nil {
		t.Error("a relative path was accepted")
	}
	SetOverrides(nil)
	got, err := LoadOverrides()
	if err != nil || got["gdb"] != gdb || CurrentOverrides()["gdb"] != gdb {
		t.Fatalf("loaded %v, %v", got, err)
	}
	file, _ := OverridesFile()
	if _, err := os.Stat(file); err != nil {
		t.Errorf("settings file: %v", err)
	}
	if err := SaveOverride("gdb", ""); err != nil {
		t.Fatal(err)
	}
	if got, _ := LoadOverrides(); len(got) != 0 {
		t.Errorf("after clearing: %v", got)
	}
}
