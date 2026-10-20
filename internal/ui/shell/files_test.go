package shell

import (
	"os"
	"path/filepath"
	"testing"

	"fyne.io/fyne/v2/test"
)

// TestSaveAsAndCloseAll: Save As writes the buffer to a new path (folders made as needed), the
// tab follows it, the old file stays as it was; replacing an existing file waits for the
// user's yes; Close All with unsaved changes asks, and Don't Save closes every tab without
// writing.
func TestSaveAsAndCloseAll(t *testing.T) {
	s, root, q := newEditorShell(t)
	a := filepath.Join(root, "a.txt")
	other := filepath.Join(root, "other.txt")
	for p, body := range map[string]string{a: "alpha\n", other: "keep me\n"} {
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	read := func(p string) string { b, _ := os.ReadFile(p); return string(b) }
	s.OpenFile(a)
	q.pumpUntil(t, "a.txt open", func() bool { return len(s.editors.Items) == 1 })
	s.ed.sess.Input("Onew line<Esc>")
	q.pumpUntil(t, "modified", func() bool { return s.editors.Selected().Text == "a.txt ●" })

	target := filepath.Join(root, "sub folder ü", "b.txt")
	s.saveAsTo(s.activeBuffer(), target)
	q.pumpUntil(t, "the tab renamed b.txt", func() bool { return s.editors.Selected().Text == "b.txt" })
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
	s.saveAsTo(s.activeBuffer(), other)
	if s.lastDialog == nil || read(other) != "keep me\n" {
		t.Fatalf("replacing a file did not ask first (file now %q)", read(other))
	}
	replace := findButton(s.win.Canvas().Overlays().Top(), "Replace")
	if replace == nil {
		t.Fatal("no Replace button")
	}
	test.Tap(replace)
	q.pumpUntil(t, "other.txt replaced", func() bool { return read(other) == "new line\nalpha\n" })

	// Close All asks about unsaved changes; Don't Save closes everything and writes nothing.
	s.OpenFile(a)
	q.pumpUntil(t, "a.txt open again", func() bool { return len(s.editors.Items) == 2 })
	s.ed.sess.Input("Ounsaved<Esc>")
	q.pumpUntil(t, "a.txt modified", func() bool { return s.editors.Selected().Text == "a.txt ●" })
	s.lastDialog = nil
	s.closeAllTabs()
	if s.lastDialog == nil {
		t.Fatal("Close All with unsaved changes did not ask")
	}
	dontSave := findButton(s.win.Canvas().Overlays().Top(), "Don't Save")
	if dontSave == nil {
		t.Fatal("no Don't Save button")
	}
	test.Tap(dontSave)
	q.pumpUntil(t, "every tab closed", func() bool { return len(s.editors.Items) == 0 })
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
	s.Commands().Run("file.saveAs") // no buffer in the editor: a notice, no dialog, no crash
}
