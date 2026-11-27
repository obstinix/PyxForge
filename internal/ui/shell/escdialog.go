package shell

import (
	"github.com/obstinix/PyxForge/internal/ui/kit"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	fynetheme "fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// Dialogs whose buttons answer Esc (#29). Esc closes the dialog in front, as Cancel would
// (modal.go), but a focused control gets every key first, and Fyne's dialog buttons ignore Esc:
// after Tab moved focus onto one, Esc did nothing. PyxForge's dialogs therefore use
// dialogButtons, through these replacements for Fyne's NewConfirm, NewCustomConfirm, NewForm and
// NewCustom, which keep their layout and their callbacks' meaning: Hide (and Esc) answers false,
// as Fyne's dialogs do.

// shells finds the Shell of a window (fyne.Window -> *Shell), so the constructors below take a
// parent window as Fyne's do. A workspace switch reuses the window with a new Shell.
var shells sync.Map

func shellOf(w fyne.Window) *Shell {
	if v, ok := shells.Load(w); ok {
		return v.(*Shell)
	}
	return nil
}

// dialogButton is a dialog's button (a kit.Button): Esc on it closes the dialog in front.
type dialogButton struct {
	kit.Button
	s *Shell // nil for a window that isn't a Shell's: Esc then does nothing on it
}

func newDialogButton(s *Shell, label string, tapped func()) *dialogButton {
	b := &dialogButton{s: s}
	b.Text, b.OnTapped = label, tapped
	b.ExtendBaseWidget(b)
	return b
}

// TypedKey closes the dialog in front on Esc; Space, Return and Enter press the button.
func (b *dialogButton) TypedKey(ev *fyne.KeyEvent) {
	if ev.Name == fyne.KeyEscape && b.s != nil {
		b.s.modalKey(ev)
		return
	}
	b.Button.TypedKey(ev)
}

// escDialog is a Fyne custom dialog with dialogButtons, and the methods PyxForge uses of Fyne's
// confirm and form dialogs.
type escDialog struct {
	*dialog.CustomDialog
	confirm, dismiss *dialogButton // nil when the dialog has none
	form             *widget.Form  // a form dialog's form
	answer           func(bool)
	answered         bool
}

func newEscDialog(parent fyne.Window, title string, content fyne.CanvasObject, confirm, dismiss string, answer func(bool)) *escDialog {
	s := shellOf(parent)
	d := &escDialog{CustomDialog: dialog.NewCustomWithoutButtons(title, content, parent), answer: answer}
	var buttons []fyne.CanvasObject
	if dismiss != "" {
		d.dismiss = newDialogButton(s, dismiss, d.Hide)
		buttons = append(buttons, d.dismiss)
	}
	if confirm != "" {
		d.confirm = newDialogButton(s, confirm, d.Confirm)
		d.confirm.Variant = kit.ButtonCTA
		buttons = append(buttons, d.confirm)
	}
	if len(buttons) > 0 {
		d.SetButtons([]fyne.CanvasObject{container.NewGridWithColumns(len(buttons), buttons...)})
	}
	return d
}

// respond hides the dialog and answers its callback, once per showing.
func (d *escDialog) respond(ok bool) {
	if d.answered {
		return
	}
	d.answered = true
	d.CustomDialog.Hide()
	if d.answer != nil {
		d.answer(ok)
	}
}

// Show shows the dialog, ready to be answered again if it is shown again.
func (d *escDialog) Show() {
	d.answered = false
	d.CustomDialog.Show()
}

// Hide closes the dialog answering false, as Cancel would: what Esc does.
func (d *escDialog) Hide() { d.respond(false) }

// Dismiss is Hide.
func (d *escDialog) Dismiss() { d.respond(false) }

// Confirm closes the dialog answering true; a form's invalid fields keep it open.
func (d *escDialog) Confirm() {
	if d.confirm != nil && d.confirm.Disabled() {
		return
	}
	d.respond(true)
}

// Submit is Confirm, for form dialogs.
func (d *escDialog) Submit() { d.Confirm() }

// SetConfirmText relabels the confirm button.
func (d *escDialog) SetConfirmText(label string) {
	if d.confirm != nil {
		d.confirm.SetText(label)
	}
}

// SetDismissText relabels the dismiss button.
func (d *escDialog) SetDismissText(label string) {
	if d.dismiss != nil {
		d.dismiss.SetText(label)
	}
}

// escConfirm is Fyne's dialog.NewConfirm: message, No and Yes.
func escConfirm(title, message string, callback func(bool), parent fyne.Window) *escDialog {
	text := widget.NewLabel(message)
	text.Wrapping = fyne.TextWrapWord
	text.Alignment = fyne.TextAlignCenter
	d := newEscDialog(parent, title, text, "Yes", "No", callback)
	d.SetIcon(fynetheme.QuestionIcon())
	return d
}

// escCustomConfirm is Fyne's dialog.NewCustomConfirm.
func escCustomConfirm(title, confirm, dismiss string, content fyne.CanvasObject, callback func(bool), parent fyne.Window) *escDialog {
	return newEscDialog(parent, title, content, confirm, dismiss, callback)
}

// escCustom is Fyne's dialog.NewCustom: content and one button that closes it.
func escCustom(title, dismiss string, content fyne.CanvasObject, parent fyne.Window) *escDialog {
	return newEscDialog(parent, title, content, "", dismiss, nil)
}

// escForm is Fyne's dialog.NewForm: the confirm button is disabled while an item fails its
// validation, from the start.
func escForm(title, confirm, dismiss string, items []*widget.FormItem, callback func(bool), parent fyne.Window) *escDialog {
	form := widget.NewForm(items...)
	d := newEscDialog(parent, title, form, confirm, dismiss, callback)
	d.form = form
	valid := func(err error) {
		if err != nil {
			d.confirm.Disable()
		} else {
			d.confirm.Enable()
		}
	}
	valid(form.Validate())
	form.SetOnValidationChanged(valid)
	return d
}
