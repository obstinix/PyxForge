package workspace

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
)

func TestStateRoundTrip(t *testing.T) {
	st := StateStore{Dir: filepath.Join(t.TempDir(), "workspaces")}
	root := t.TempDir()
	boot := filepath.Join(root, "boot.asm")
	if err := os.WriteFile(boot, []byte("org 0x7c00\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := st.Load(root); got.Root != root || got.Files != nil {
		t.Errorf("a new workspace: %+v", got)
	}
	off := false
	want := State{Root: root, Files: []string{boot, filepath.Join(root, "deleted.c")}, Active: boot,
		Expanded: []string{"src", "src/kernel"}, Inspector: &off, PanelTab: "Build", Build: "boot", Width: 1280, Height: 800}
	if err := st.Save(want); err != nil {
		t.Fatal(err)
	}
	got := st.Load(root)
	want.Files = want.Files[:1] // files that no longer exist are dropped
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Load:\n got %+v\nwant %+v", got, want)
	}

	// Another workspace has its own state; a damaged file reads as empty.
	other := t.TempDir()
	if s := st.Load(other); s.Build != "" {
		t.Errorf("another workspace inherited state: %+v", s)
	}
	if err := os.WriteFile(st.path(other), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if s := st.Load(other); s.Root != other || s.Build != "" {
		t.Errorf("damaged file: %+v", s)
	}
	// A file of NULs, as a crash between write and flush leaves one, reads as empty too, and
	// the next save replaces it.
	if err := os.WriteFile(st.path(other), make([]byte, 256), 0o644); err != nil {
		t.Fatal(err)
	}
	if s := st.Load(other); s.Root != other || s.Build != "" {
		t.Errorf("file of NULs: %+v", s)
	}
	if err := st.Save(State{Root: other, Build: "kernel"}); err != nil || st.Load(other).Build != "kernel" {
		t.Errorf("saving over a file of NULs: %v, %+v", err, st.Load(other))
	}
	if err := st.Forget(root); err != nil || st.Load(root).Build != "" {
		t.Errorf("Forget: %v", err)
	}
	if err := st.Forget(root); err != nil {
		t.Errorf("forgetting twice: %v", err)
	}
}

// TestShadaFile: each workspace has a ShaDa file of its own beside its state, in a folder that
// exists, the same however the folder is spelled where the file system ignores case.
func TestShadaFile(t *testing.T) {
	st := StateStore{Dir: filepath.Join(t.TempDir(), "workspaces")}
	a, b := st.ShadaFile(filepath.Join("C:", "src", "kernel")), st.ShadaFile(filepath.Join("C:", "src", "bootloader"))
	if a == b || filepath.Ext(a) != ".shada" || filepath.Dir(a) != st.Dir {
		t.Errorf("ShadaFile: %q and %q", a, b)
	}
	if fi, err := os.Stat(st.Dir); err != nil || !fi.IsDir() {
		t.Errorf("the folder for ShaDa files was not made: %v", err)
	}
	if again := st.ShadaFile(filepath.Join("C:", "src", "kernel") + string(filepath.Separator)); again != a {
		t.Errorf("a trailing separator gave another file: %q", again)
	}
	if runtime.GOOS == "windows" && st.ShadaFile(filepath.Join("C:", "SRC", "Kernel")) != a {
		t.Error("the same folder in other letter case gave another file")
	}
	if none := st.ShadaFile(""); none == a || filepath.Dir(none) != st.Dir || filepath.Base(none) != "no-workspace.shada" {
		t.Errorf("without a workspace: %q", none)
	}
}

// TestPruneForgetsMissingWorkspaces: the state and editor history of a workspace whose folder
// is gone are removed; a workspace that still exists keeps both.
func TestPruneForgetsMissingWorkspaces(t *testing.T) {
	st := StateStore{Dir: t.TempDir()}
	here, gone := t.TempDir(), filepath.Join(t.TempDir(), "deleted-project")
	if err := os.WriteFile(filepath.Join(here, "boot.asm"), []byte("hlt\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, root := range []string{here, gone} {
		if err := st.Save(State{Root: root, Files: []string{filepath.Join(root, "boot.asm")}}); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(st.ShadaFile(root), []byte("shada"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	removed, err := st.Prune()
	if err != nil || len(removed) != 2 {
		t.Fatalf("removed %v, %v", removed, err)
	}
	if _, err := os.Stat(st.path(gone)); !os.IsNotExist(err) {
		t.Errorf("the missing workspace's state is still there: %v", err)
	}
	if _, err := os.Stat(st.ShadaFile(gone)); !os.IsNotExist(err) {
		t.Errorf("its history is still there: %v", err)
	}
	if got := st.Load(here); len(got.Files) != 1 {
		t.Errorf("the existing workspace lost its state: %+v", got)
	}
	if _, err := os.Stat(st.ShadaFile(here)); err != nil {
		t.Errorf("the existing workspace lost its history: %v", err)
	}
}
