package shell

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
)

// TestChangedOnDiskAsksBeforeLosingChanges: a file with unsaved changes changed by another
// program brings up PyxForge's question; nothing is written until the user answers (Save asks
// the same question rather than writing); Overwrite Disk saves the document over the file.
func TestChangedOnDiskAsksBeforeLosingChanges(t *testing.T) {
	s, root, _ := newEditorShell(t)
	file := filepath.Join(root, "notes.txt")
	if err := os.WriteFile(file, []byte("first\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	s.OpenFile(file)
	d := s.activeDoc()
	if d == nil {
		t.Fatal("notes.txt did not open")
	}
	test.Type(d.ed, "mine")
	d.ed.TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})
	if s.editors.Selected().Text != "notes.txt ●" {
		t.Fatalf("tab %q, want the unsaved marker", s.editors.Selected().Text)
	}

	if err := os.WriteFile(file, []byte("theirs\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	later := time.Now().Add(time.Hour)
	if err := os.Chtimes(file, later, later); err != nil {
		t.Fatal(err)
	}
	s.checkFiles() // what the window coming to the front, or the explorer's watcher, does
	if !d.asking || len(s.modals) != 1 {
		t.Fatalf("question showing %v, %d dialogs", d.asking, len(s.modals))
	}
	s.checkFiles() // asking again while the question shows adds no second dialog
	s.saveCurrent()
	if len(s.modals) != 1 {
		t.Errorf("%d dialogs showing", len(s.modals))
	}
	if data, _ := os.ReadFile(file); string(data) != "theirs\n" {
		t.Fatalf("written before the user answered: %q", data)
	}

	overwrite := findButton(s.win.Canvas().Overlays().Top(), "Overwrite Disk")
	if overwrite == nil {
		t.Fatal("no Overwrite Disk button")
	}
	test.Tap(overwrite)
	if data, _ := os.ReadFile(file); string(data) != "mine\nfirst\n" {
		t.Errorf("after Overwrite Disk the file holds %q", data)
	}
	if s.editors.Selected().Text != "notes.txt" || d.asking {
		t.Errorf("after Overwrite Disk: tab %q, question showing %v", s.editors.Selected().Text, d.asking)
	}

	// Changed on disk again, with unsaved changes: Reload from Disk drops them.
	test.Type(d.ed, "more")
	even := later.Add(time.Hour)
	if err := os.WriteFile(file, []byte("theirs again\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(file, even, even); err != nil {
		t.Fatal(err)
	}
	s.checkFiles()
	reload := findButton(s.win.Canvas().Overlays().Top(), "Reload from Disk")
	if reload == nil {
		t.Fatal("no Reload from Disk button")
	}
	test.Tap(reload)
	if got := d.ed.Buf.Text(); got != "theirs again\n" || d.ed.Buf.Modified() {
		t.Errorf("after Reload from Disk: %q, modified %v", got, d.ed.Buf.Modified())
	}
}
