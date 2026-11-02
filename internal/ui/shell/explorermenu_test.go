package shell

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

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
	q.pumpUntil(t, "two tabs", func() bool { return len(s.ed.bufs) == 2 })
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
	q.pumpUntil(t, "a.txt selected", func() bool { return s.activeFile() == a })
	s.ed.sess.Input("Ochanged<Esc>")
	q.pumpUntil(t, "a.txt modified", func() bool { return s.activeBuffer() != nil && s.activeBuffer().modified })
	s.renameTo(a, "b.txt")
	if _, err := os.Stat(a); err != nil || !logged(s, "Save a.txt first") {
		t.Fatalf("a rename of a modified file went ahead (%v)", err)
	}
	s.trashEntry(a, false)
	test.Tap(findButton(s.win.Canvas().Overlays().Top(), "Yes"))
	if len(trashed) != 0 {
		t.Fatalf("a modified file was trashed")
	}
	s.ed.sess.Input("u")
	q.pumpUntil(t, "a.txt unmodified", func() bool { return !s.activeBuffer().modified })

	// A folder's rename takes the tabs of the files in it along.
	s.renameTo(filepath.Join(root, "src"), "lib")
	q.pumpUntil(t, "main.c open from lib", func() bool {
		return s.ed.tabFor(filepath.Join(root, "lib", "main.c")) != nil && len(s.ed.bufs) == 2
	})
	if _, err := os.Stat(filepath.Join(root, "lib", "main.c")); err != nil {
		t.Error(err)
	}
	s.renameTo(a, "b.txt")
	q.pumpUntil(t, "b.txt open", func() bool { return s.ed.tabFor(filepath.Join(root, "b.txt")) != nil && len(s.ed.bufs) == 2 })
	if got := strings.Join(tabNames(), " "); got != "b.txt main.c" {
		t.Errorf("tabs after the renames: %s", got)
	}

	// Move to Trash asks, closes the file's tab, and moves it.
	b := filepath.Join(root, "b.txt")
	s.trashEntry(b, false)
	test.Tap(findButton(s.win.Canvas().Overlays().Top(), "Yes"))
	q.pumpUntil(t, "b.txt trashed", func() bool { return len(trashed) == 1 })
	q.pumpUntil(t, "its tab closed", func() bool { return len(s.ed.bufs) == 1 })
	if _, err := os.Stat(b); !os.IsNotExist(err) || trashed[0] != b {
		t.Errorf("trashed %v; b.txt: %v", trashed, err)
	}
}
