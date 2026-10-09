package workspace

import (
	"os"
	"path/filepath"
	"reflect"
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
	if err := st.Forget(root); err != nil || st.Load(root).Build != "" {
		t.Errorf("Forget: %v", err)
	}
	if err := st.Forget(root); err != nil {
		t.Errorf("forgetting twice: %v", err)
	}
}
