package shell

import (
	"os"
	"path/filepath"
	"testing"

	"fyne.io/fyne/v2"
	"github.com/obstinix/PyxForge/internal/ui/theme"
)

// TestDialogsAnswerTheKeyboard (PXF-DEF-021): in a form, Enter submits and Esc cancels from
// the text field; with no field focused, Esc cancels and Enter runs the default action; Esc
// closes only the dialog in front.
func TestDialogsAnswerTheKeyboard(t *testing.T) {
	s, root := newTestShell(t)
	c := s.win.Canvas()
	esc, enter := &fyne.KeyEvent{Name: fyne.KeyEscape}, &fyne.KeyEvent{Name: fyne.KeyReturn}

	// New File: Enter in the name field creates the file and closes the dialog.
	s.newEntry(false)
	entry, ok := c.Focused().(*dialogEntry)
	if !ok || len(s.modals) != 1 {
		t.Fatalf("New File: focused %T, %d dialogs", c.Focused(), len(s.modals))
	}
	entry.SetText("notes.md")
	entry.TypedKey(enter)
	if _, err := os.Stat(filepath.Join(root, "notes.md")); err != nil || len(s.modals) != 0 {
		t.Fatalf("Enter did not create the file and close the dialog: %v, %d dialogs", err, len(s.modals))
	}

	// Esc in the field cancels: nothing is created.
	s.newEntry(false)
	entry = c.Focused().(*dialogEntry)
	entry.SetText("never.md")
	entry.TypedKey(esc)
	if _, err := os.Stat(filepath.Join(root, "never.md")); err == nil || len(s.modals) != 0 {
		t.Fatalf("Esc did not cancel New File (%d dialogs)", len(s.modals))
	}

	// A confirmation with nothing focused: Esc cancels, Enter confirms.
	s.setAccent(theme.Accents[len(theme.Accents)-1].ID)
	changed := s.sel
	s.confirmReset()
	c.Unfocus()
	c.OnTypedKey()(esc)
	if len(s.modals) != 0 || s.sel != changed {
		t.Fatalf("Esc on Reset appearance: %d dialogs, selection %+v", len(s.modals), s.sel)
	}
	s.confirmReset()
	c.Unfocus()
	c.OnTypedKey()(enter)
	if len(s.modals) != 0 || s.sel != theme.Default {
		t.Fatalf("Enter on Reset appearance: %d dialogs, selection %+v", len(s.modals), s.sel)
	}

	// Two dialogs: Esc closes the one in front only.
	s.about()
	s.newEntry(true)
	if len(s.modals) != 2 {
		t.Fatalf("%d dialogs showing, want 2", len(s.modals))
	}
	c.Focused().(*dialogEntry).TypedKey(esc)
	if len(s.modals) != 1 {
		t.Fatalf("Esc closed %d dialogs", 2-len(s.modals))
	}
	c.Unfocus()
	c.OnTypedKey()(esc)
	if len(s.modals) != 0 {
		t.Error("Esc did not close About")
	}
}

// TestDialogKeysDoNotReachTheEditor: while a dialog shows, keys go to it, not to Neovim
// underneath.
func TestDialogKeysDoNotReachTheEditor(t *testing.T) {
	s, root, q := newEditorShell(t)
	f := filepath.Join(root, "a.txt")
	if err := os.WriteFile(f, []byte("a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	s.OpenFile(f)
	q.pumpUntil(t, "the editor focused", func() bool { return s.win.Canvas().Focused() == s.ed.view })
	s.about()
	if got := s.win.Canvas().Focused(); got == s.ed.view {
		t.Error("the editor keeps keyboard focus under a dialog")
	}
	s.win.Canvas().OnTypedKey()(&fyne.KeyEvent{Name: fyne.KeyEscape})
	q.pumpUntil(t, "focus back in the editor", func() bool { return s.win.Canvas().Focused() == s.ed.view })
}
