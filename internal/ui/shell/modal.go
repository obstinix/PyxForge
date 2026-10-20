package shell

import (
	"slices"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
)

// Dialogs answer the keyboard (PXF-DEF-021): Esc closes the dialog in front, as Cancel would,
// and Enter runs its default action. Fyne's dialogs do neither by themselves. Keys reach the
// window only while no control has focus, which is how a dialog opens, so the window's key
// handler serves them then; a text field in a dialog (newDialogEntry) passes Esc on itself,
// and submits with Enter.

// modal is a dialog PyxForge shows, newest last.
type modal struct {
	hide    func()
	confirm func() // the default action, for Enter; nil when Enter should do nothing
}

// shownDialog is what present needs from a dialog: Fyne's dialogs and file dialogs have it.
type shownDialog interface {
	Show()
	Hide()
	SetOnClosed(func())
}

// present shows d and makes it the dialog Esc closes and Enter confirms (confirm may be nil),
// until it closes however it closes.
func (s *Shell) present(d shownDialog, confirm func()) {
	m := &modal{hide: d.Hide, confirm: confirm}
	s.modals = append(s.modals, m)
	d.SetOnClosed(func() {
		s.modals = slices.DeleteFunc(s.modals, func(x *modal) bool { return x == m })
	})
	d.Show()
}

// modalKey is the window's handler for keys no control takes.
func (s *Shell) modalKey(ev *fyne.KeyEvent) {
	if len(s.modals) == 0 {
		return
	}
	top := s.modals[len(s.modals)-1]
	switch ev.Name {
	case fyne.KeyEscape:
		top.hide()
	case fyne.KeyReturn, fyne.KeyEnter:
		if top.confirm != nil {
			top.confirm()
		}
	}
}

// dialogEntry is a text field for dialogs: Esc closes the dialog in front, as it does when no
// field has focus. Enter submits through OnSubmitted, as in any single-line Fyne entry.
type dialogEntry struct {
	widget.Entry
	s *Shell
}

func (s *Shell) newDialogEntry() *dialogEntry {
	e := &dialogEntry{s: s}
	e.ExtendBaseWidget(e)
	return e
}

func (e *dialogEntry) TypedKey(ev *fyne.KeyEvent) {
	if ev.Name == fyne.KeyEscape {
		e.s.modalKey(ev)
		return
	}
	e.Entry.TypedKey(ev)
}

// submitForm gives a form dialog's text fields Enter-to-submit and presents it.
func (s *Shell) submitForm(d interface {
	shownDialog
	Submit()
}, fields ...*dialogEntry) {
	for _, f := range fields {
		f.OnSubmitted = func(string) { d.Submit() }
	}
	s.present(d, d.Submit)
}
