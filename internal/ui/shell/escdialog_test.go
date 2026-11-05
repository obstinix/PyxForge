package shell

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

// TestEscClosesADialogFromAFocusedButton: once Tab has moved keyboard focus onto a dialog's
// button, a key reaches only that button; Esc there still closes the dialog in front, answering
// as Cancel does, and Return still presses the button (#29).
func TestEscClosesADialogFromAFocusedButton(t *testing.T) {
	s, _ := newTestShell(t)
	c := s.win.Canvas()
	press := func(key fyne.KeyName) {
		t.Helper()
		f := c.Focused()
		if f == nil {
			t.Fatal("nothing has focus")
		}
		f.TypedKey(&fyne.KeyEvent{Name: key}) // what the driver does with a focused widget
	}

	// A confirmation: Esc on its focused Yes answers no and closes it.
	var answers []bool
	d := escConfirm("Replace boot.bin?", "It exists.", func(ok bool) { answers = append(answers, ok) }, s.win)
	s.present(d, d.Confirm)
	c.Focus(d.confirm)
	press(fyne.KeyEscape)
	if len(answers) != 1 || answers[0] || len(s.modals) != 0 {
		t.Fatalf("Esc on a focused Yes: answers %v, %d dialogs open", answers, len(s.modals))
	}
	// Shown again, Return on the focused Yes confirms.
	s.present(d, d.Confirm)
	c.Focus(d.confirm)
	press(fyne.KeyReturn)
	if len(answers) != 2 || !answers[1] || len(s.modals) != 0 {
		t.Fatalf("Return on a focused Yes: answers %v, %d dialogs open", answers, len(s.modals))
	}

	// A form: Esc on its focused Cancel, and on its focused confirm, cancels it.
	entry := s.newDialogEntry()
	submitted := 0
	f := escForm("Rename Terminal", "Rename", "Cancel", []*widget.FormItem{widget.NewFormItem("Name", entry)},
		func(ok bool) {
			if ok {
				submitted++
			}
		}, s.win)
	s.submitForm(f, entry)
	for _, b := range []*dialogButton{f.dismiss, f.confirm} {
		if len(s.modals) == 0 {
			s.submitForm(f, entry)
		}
		c.Focus(b)
		press(fyne.KeyEscape)
		if len(s.modals) != 0 || submitted != 0 {
			t.Fatalf("Esc on the focused %q left %d dialogs open (submitted %d)", b.Text, len(s.modals), submitted)
		}
	}

	// PyxForge's own choice dialogs: Esc on a focused choice cancels without choosing.
	chose := false
	s.choose("Branch has changes", "Switch anyway?", choice{label: "Switch", primary: true, run: func() { chose = true }})
	sw := findDialogButton(s.win.Canvas().Overlays().Top(), "Switch")
	if sw == nil {
		t.Fatal("the choice dialog has no Switch button")
	}
	c.Focus(sw)
	press(fyne.KeyEscape)
	if chose || len(s.modals) != 0 {
		t.Fatalf("Esc on a focused choice: chose %v, %d dialogs open", chose, len(s.modals))
	}
}

// findDialogButton is findButton for PyxForge's dialog buttons, returning the widget itself (it
// takes the focus; its embedded Fyne button does not).
func findDialogButton(o fyne.CanvasObject, text string) *dialogButton {
	if o == nil || !o.Visible() {
		return nil
	}
	switch w := o.(type) {
	case *dialogButton:
		if w.Text == text {
			return w
		}
	case *fyne.Container:
		for _, c := range w.Objects {
			if b := findDialogButton(c, text); b != nil {
				return b
			}
		}
	case fyne.Widget:
		for _, c := range test.WidgetRenderer(w).Objects() {
			if b := findDialogButton(c, text); b != nil {
				return b
			}
		}
	}
	return nil
}
