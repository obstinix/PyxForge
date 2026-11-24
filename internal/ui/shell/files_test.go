package shell

import (
	"os"
	"path/filepath"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
)

// saveAsThrough answers the Save As dialog with path.
func saveAsThrough(t *testing.T, s *Shell, path string) {
	t.Helper()
	s.saveAsDialog()
	entry, ok := s.win.Canvas().Focused().(*dialogEntry)
	if !ok {
		t.Fatalf("Save As: focused %T", s.win.Canvas().Focused())
	}
	entry.SetText(path)
	entry.TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})
}

// TestSaveAsAndCloseAll: Save As writes the document to a new path (folders made as needed),
// the tab follows it, the old file stays as it was; replacing an existing file waits for the
// user's yes; Close All with unsaved changes asks, and Don't Save closes every tab without
// writing.
func TestSaveAsAndCloseAll(t *testing.T) {
	s, root, _ := newEditorShell(t)
	a := filepath.Join(root, "a.txt")
	other := filepath.Join(root, "other.txt")
	for p, body := range map[string]string{a: "alpha\n", other: "keep me\n"} {
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	read := func(p string) string { b, _ := os.ReadFile(p); return string(b) }
	s.OpenFile(a)
	d := s.activeDoc()
	test.Type(d.ed, "new line")
	d.ed.TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})
	if s.editors.Selected().Text != "a.txt ●" {
		t.Fatalf("tab %q, want the unsaved marker", s.editors.Selected().Text)
	}

	target := filepath.Join(root, "sub folder ü", "b.txt")
	saveAsThrough(t, s, target)
	if got := s.editors.Selected().Text; got != "b.txt" {
		t.Errorf("tab %q after Save As", got)
	}
	if got := read(target); got != "new line\nalpha\n" {
		t.Errorf("Save As wrote %q", got)
	}
	if got := read(a); got != "alpha\n" {
		t.Errorf("Save As changed the original: %q", got)
	}
	if s.activeFile() != target {
		t.Errorf("active file %q", s.activeFile())
	}

	// An existing file is replaced only after the user says so.
	saveAsThrough(t, s, other)
	if read(other) != "keep me\n" {
		t.Fatalf("replacing a file did not ask first (file now %q)", read(other))
	}
	replace := findButton(s.win.Canvas().Overlays().Top(), "Replace")
	if replace == nil {
		t.Fatal("no Replace button")
	}
	test.Tap(replace)
	if got := read(other); got != "new line\nalpha\n" {
		t.Errorf("after Replace other.txt holds %q", got)
	}

	// Close All asks about unsaved changes; Don't Save closes everything and writes nothing.
	s.OpenFile(a)
	test.Type(s.activeDoc().ed, "unsaved")
	if s.editors.Selected().Text != "a.txt ●" || len(s.editors.Items) != 2 {
		t.Fatalf("tabs %v", tabTitles(s))
	}
	s.lastDialog = nil
	s.closeAllTabs()
	if s.lastDialog == nil {
		t.Fatal("Close All with unsaved changes did not ask")
	}
	tapInDialog(t, s, "Don't Save")
	if len(s.editors.Items) != 0 {
		t.Errorf("tabs left after Close All: %v", tabTitles(s))
	}
	if got := read(a); got != "alpha\n" {
		t.Errorf("Don't Save wrote a.txt: %q", got)
	}
}

// TestFileDialogsOpen: Open File… (Ctrl+O) and Save As… show their dialogs without crashing (a
// FileDialog sized before Show dereferences a nil window: found in the Windows build).
func TestFileDialogsOpen(t *testing.T) {
	s, _ := newTestShell(t)
	s.Commands().Run("file.open")
	if s.win.Canvas().Overlays().Top() == nil {
		t.Error("Open File showed no dialog")
	}
	s.Commands().Run("file.saveAs") // no document in the editor: a notice, no dialog, no crash
}

// TestUntitledFile: New File (Ctrl+N) opens an untitled file; saving it asks where, and once
// saved its tab takes the file's name; closing a changed untitled file asks first. A new file
// is written with LF line endings on every platform; files that exist keep their own.
func TestUntitledFile(t *testing.T) {
	s, root, _ := newEditorShell(t)
	read := func(p string) string { b, _ := os.ReadFile(p); return string(b) }
	s.Commands().Run("file.new")
	d := s.activeDoc()
	if d == nil || s.editors.Selected().Text != "Untitled-1" {
		t.Fatalf("no untitled tab: %v", tabTitles(s))
	}
	test.Type(d.ed, "hello from a new file")
	if s.editors.Selected().Text != "Untitled-1 ●" {
		t.Fatalf("tab %q, want the unsaved marker", s.editors.Selected().Text)
	}

	s.lastDialog = nil
	s.saveCurrent()
	entry, ok := s.win.Canvas().Focused().(*dialogEntry)
	if !ok || s.lastDialog == nil || filepath.Base(entry.Text) != "untitled.txt" {
		t.Fatalf("Save on an untitled file: focused %T, dialog %v", s.win.Canvas().Focused(), s.lastDialog != nil)
	}
	target := filepath.Join(root, "notes", "first.md")
	entry.SetText(target)
	entry.TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})
	if got := s.editors.Selected().Text; got != "first.md" {
		t.Errorf("tab %q after saving", got)
	}
	if got := read(target); got != "hello from a new file" {
		t.Errorf("saved %q", got)
	}

	// Closing a changed untitled file asks first.
	s.newUntitled()
	if s.editors.Selected().Text != "Untitled-2" {
		t.Fatalf("tabs %v", tabTitles(s))
	}
	test.Type(s.activeDoc().ed, "changed")
	s.lastDialog = nil
	s.closeEditor()
	if s.lastDialog == nil {
		t.Error("closing a changed untitled file did not ask")
	}
}
