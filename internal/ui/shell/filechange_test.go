package shell

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"fyne.io/fyne/v2/test"
)

// TestChangedOnDiskAsksBeforeLosingChanges: a file with unsaved changes changed by another
// program brings up PyxForge's question, not Neovim's prompt inside the grid; nothing is
// written until the user answers; Overwrite Disk saves the buffer over the file.
func TestChangedOnDiskAsksBeforeLosingChanges(t *testing.T) {
	s, root, q := newEditorShell(t)
	file := filepath.Join(root, "notes.txt")
	if err := os.WriteFile(file, []byte("first\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	s.OpenFile(file)
	q.pumpUntil(t, "the tab", func() bool { return len(s.editors.Items) == 1 })
	s.ed.sess.Input("Omine<Esc>")
	q.pumpUntil(t, "the unsaved marker", func() bool { return s.editors.Selected().Text == "notes.txt ●" })

	if err := os.WriteFile(file, []byte("theirs\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	later := time.Now().Add(time.Hour)
	if err := os.Chtimes(file, later, later); err != nil {
		t.Fatal(err)
	}
	s.checkFiles() // what the window coming to the front, or the explorer's watcher, does
	q.pumpUntil(t, "the question", func() bool { return len(s.conflicts) == 1 && s.lastDialog != nil })
	s.checkFiles() // asking again while the question shows adds no second dialog
	s.saveCurrent()
	time.Sleep(200 * time.Millisecond)
	q.pumpUntil(t, "events", func() bool { return true })
	if len(s.conflicts) != 1 {
		t.Errorf("%d questions showing", len(s.conflicts))
	}
	if data, _ := os.ReadFile(file); string(data) != "theirs\n" {
		t.Fatalf("written before the user answered: %q", data)
	}

	overwrite := findButton(s.win.Canvas().Overlays().Top(), "Overwrite Disk")
	if overwrite == nil {
		t.Fatal("no Overwrite Disk button")
	}
	test.Tap(overwrite)
	q.pumpUntil(t, "the file overwritten", func() bool {
		data, _ := os.ReadFile(file)
		return string(data) == "mine\nfirst\n"
	})
	q.pumpUntil(t, "the saved title", func() bool { return s.editors.Selected().Text == "notes.txt" })
	if len(s.conflicts) != 0 {
		t.Error("the question is still recorded as showing")
	}
}
