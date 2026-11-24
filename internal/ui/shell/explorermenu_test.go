package shell

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
)

// TestExplorerContextMenu: Rename and Move to Trash refuse a file with unsaved changes, and
// otherwise close its tabs first; a renamed file's tab opens again at the new name, folders
// included; Copy Relative Path copies the workspace-relative path with forward slashes.
func TestExplorerContextMenu(t *testing.T) {
	s, root, q := newEditorShell(t)
	var trashed []string
	was := moveToTrash
	moveToTrash = func(p string) error { trashed = append(trashed, p); return os.RemoveAll(p) }
	t.Cleanup(func() { moveToTrash = was })
	for _, f := range []string{"a.txt", "src/main.c", "gone.txt"} {
		p := filepath.Join(root, filepath.FromSlash(f))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(f+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	a, mainC := filepath.Join(root, "a.txt"), filepath.Join(root, "src", "main.c")
	s.OpenFile(a)
	s.OpenFile(mainC)
	if len(s.nat.docs) != 2 {
		t.Fatalf("%d documents open, want 2", len(s.nat.docs))
	}
	tabNames := func() []string {
		var n []string
		for _, it := range s.editors.Items {
			n = append(n, it.Text)
		}
		slices.Sort(n)
		return n
	}

	var labels []string
	for _, it := range s.explorerMenu(mainC, false).Items {
		labels = append(labels, it.Label)
	}
	if got := strings.Join(labels, "|"); got != "Open||New File…|New Folder…||Rename…|Move to "+trashName()+"||Copy Path|Copy Relative Path|"+revealLabel() {
		t.Errorf("menu items %q", got)
	}
	for _, it := range s.explorerMenu(mainC, false).Items {
		if it.Label == "Copy Relative Path" {
			it.Action()
		}
	}
	if got := s.app.Clipboard().Content(); got != "src/main.c" {
		t.Errorf("Copy Relative Path copied %q", got)
	}

	// Unsaved changes stop a rename.
	s.openFile(a, true)
	doc := s.activeDoc()
	if s.activeFile() != a || doc == nil {
		t.Fatal("a.txt is not the active document")
	}
	test.Type(doc.ed, "changed")
	s.renameTo(a, "b.txt")
	if _, err := os.Stat(a); err != nil || !logged(s, "Save a.txt first") {
		t.Fatalf("a rename of a modified file went ahead (%v)", err)
	}
	s.trashEntry(a, false)
	test.Tap(findButton(s.win.Canvas().Overlays().Top(), "Yes"))
	if len(trashed) != 0 {
		t.Fatalf("a modified file was trashed")
	}
	doc.ed.TypedShortcut(&fyne.ShortcutUndo{})
	if doc.ed.Buf.Modified() {
		t.Fatal("undo left a.txt modified")
	}

	// A folder's rename takes the tabs of the files in it along.
	s.renameTo(filepath.Join(root, "src"), "lib")
	if s.nat.docFor(filepath.Join(root, "lib", "main.c")) == nil || len(s.nat.docs) != 2 {
		t.Fatalf("main.c is not open from lib: %v", tabNames())
	}
	if _, err := os.Stat(filepath.Join(root, "lib", "main.c")); err != nil {
		t.Error(err)
	}
	s.renameTo(a, "b.txt")
	if s.nat.docFor(filepath.Join(root, "b.txt")) == nil || len(s.nat.docs) != 2 {
		t.Fatalf("b.txt is not open: %v", tabNames())
	}
	if got := strings.Join(tabNames(), " "); got != "b.txt main.c" {
		t.Errorf("tabs after the renames: %s", got)
	}

	// Move to Trash asks, closes the file's tab, and moves it.
	b := filepath.Join(root, "b.txt")
	s.trashEntry(b, false)
	test.Tap(findButton(s.win.Canvas().Overlays().Top(), "Yes"))
	q.pumpUntil(t, "b.txt trashed", func() bool { return len(trashed) == 1 })
	if len(s.nat.docs) != 1 {
		t.Errorf("%d documents open after b.txt went to the trash", len(s.nat.docs))
	}
	if _, err := os.Stat(b); !os.IsNotExist(err) || trashed[0] != b {
		t.Errorf("trashed %v; b.txt: %v", trashed, err)
	}
}

// TestCopyActivePath: the active file's path, whole or within the workspace with forward
// slashes, goes to the clipboard.
func TestCopyActivePath(t *testing.T) {
	s, root, _ := newEditorShell(t)
	f := filepath.Join(root, "src", "boot.asm")
	if err := os.MkdirAll(filepath.Dir(f), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(f, []byte("hlt\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	s.OpenFile(f)
	if s.activeDoc() == nil {
		t.Fatal("boot.asm did not open")
	}
	s.copyActivePath(true)
	if got := s.app.Clipboard().Content(); got != "src/boot.asm" {
		t.Errorf("relative path %q", got)
	}
	s.copyActivePath(false)
	if got := s.app.Clipboard().Content(); !sameFile(got, f) {
		t.Errorf("path %q, want %q", got, f)
	}
}
