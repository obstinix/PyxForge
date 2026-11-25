package toolchain

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// gitTool is Git's entry: a tool the Windows release bundles, with install folders of its own.
func gitTool(t *testing.T) Tool {
	t.Helper()
	for _, tl := range Tools {
		if tl.ID == "git" {
			return tl
		}
	}
	t.Fatal("no Git in Tools")
	return Tool{}
}

// TestLookupOrder: a path set in Doctor wins, then a bundled copy, then a toolchain PyxForge
// installed, then PATH, then the install folders; a path set in Doctor that no longer exists is reported, not replaced.
func TestLookupOrder(t *testing.T) {
	bundle := filepath.Join(programFiles, "PyxForge")
	bundled := filepath.Join(bundle, "git", "cmd", "git")
	chosen := filepath.Join(programFiles, "Git", "cmd", "git")
	d := fake(map[string]string{"git": "git version 2.40.0", bundled: "git version 2.55.0", chosen: "git version 2.30.1"})
	git := gitTool(t)
	check := func(what, wantPath, wantSource, wantVersion string) {
		t.Helper()
		st := d.Detect(context.Background(), []Tool{git})[0]
		if st.Path != wantPath || st.Source != wantSource || st.Version != wantVersion {
			t.Errorf("%s: %q from %q, version %q; want %q from %q, %q", what, st.Path, st.Source, st.Version, wantPath, wantSource, wantVersion)
		}
	}
	check("PATH only", "git", SourcePath, "2.40.0")
	installed := filepath.Join(programFiles, "PyxForge", "toolchains", "git", "cmd")
	d.LookPath = func(name string) (string, error) { // the fake knows the managed copy too
		if name == filepath.Join(installed, "git") {
			return name, nil
		}
		return fake(map[string]string{"git": "", bundled: "", chosen: ""}).LookPath(name)
	}
	d.Run = func(ctx context.Context, path string, args []string) (string, error) {
		return map[string]string{"git": "git version 2.40.0", bundled: "git version 2.55.0", chosen: "git version 2.30.1",
			filepath.Join(installed, "git"): "git version 2.50.1"}[path], nil
	}
	d.Managed = func() []string { return []string{installed} }
	check("installed by PyxForge, before PATH", filepath.Join(installed, "git"), SourceManaged, "2.50.1")
	d.BundleDir = bundle
	check("bundled, before installed by PyxForge", bundled, SourceBundled, "2.55.0")
	d.Overrides = func() Overrides { return Overrides{"git": chosen} }
	check("set in Doctor", chosen, SourceConfigured, "2.30.1")

	d.Overrides = func() Overrides { return Overrides{"git": filepath.Join(programFiles, "gone", "git")} }
	st := d.Detect(context.Background(), []Tool{git})[0]
	if st.Found() || st.Source != SourceConfigured || st.Err == nil || !strings.Contains(st.Err.Error(), "set in Doctor") {
		t.Errorf("a vanished path was not reported: %+v", st)
	}
}

// TestInstalledOutsidePath: a tool in its own install folder (Git for Windows, for all users
// or one) is found without PATH.
func TestInstalledOutsidePath(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Git's own install folders are Windows ones")
	}
	for _, variable := range []string{"ProgramFiles", "LOCALAPPDATA"} {
		t.Run(variable, func(t *testing.T) {
			root := t.TempDir()
			dir := filepath.Join(root, "Git", "cmd")
			if variable == "LOCALAPPDATA" {
				dir = filepath.Join(root, "Programs", "Git", "cmd")
			}
			path := filepath.Join(dir, "git")
			d := fake(map[string]string{path: "git version 2.55.0"})
			d.Getenv = func(key string) string {
				if key == variable {
					return root
				}
				return ""
			}
			st := d.Detect(context.Background(), []Tool{gitTool(t)})[0]
			if !st.Found() || st.Path != path || st.Source != SourceFolder {
				t.Fatalf("installed Git not discovered without PATH: %+v", st)
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

// TestExtendPath: folders are added after the user's PATH, once each (whatever their case on
// Windows), and only when they exist.
func TestExtendPath(t *testing.T) {
	user, a, b := t.TempDir(), t.TempDir(), t.TempDir()
	gone := filepath.Join(t.TempDir(), "gone")
	sep := string(os.PathListSeparator)
	got := appendPath(user, []string{a, gone, b, a})
	if want := user + sep + a + sep + b; got != want {
		t.Errorf("appendPath = %q, want %q", got, want)
	}
	if again := appendPath(got, []string{b, a}); again != got {
		t.Errorf("adding them again changed PATH: %q", again)
	}
	if runtime.GOOS == "windows" {
		if again := appendPath(got, []string{strings.ToUpper(a)}); again != got {
			t.Errorf("a folder in other case was added again: %q", again)
		}
	}
}
