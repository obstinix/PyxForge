package shell

import (
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"

	"github.com/obstinix/PyxForge/internal/askpass"
)

// askUser puts a question from Git or ssh to the user and waits for the answer (#67). It runs
// off the UI thread, in the askpass server; the dialog runs on it.
func (s *Shell) askUser(q askpass.Question) (string, bool) {
	type reply struct {
		text string
		ok   bool
	}
	ch := make(chan reply, 1)
	s.dispatch(func() { s.askDialog(q, func(text string, ok bool) { ch <- reply{text, ok} }) })
	r := <-ch
	return r.text, r.ok
}

// askDialog shows q: a yes/no question (trusting a new server) with its fingerprint and the
// choice; a password or passphrase in a field that hides it; anything else in a plain field.
// answer is called once. The answer is never logged.
func (s *Shell) askDialog(q askpass.Question, answer func(string, bool)) {
	once := false
	reply := func(text string, ok bool) {
		if !once {
			once = true
			answer(text, ok)
		}
	}
	text := widget.NewLabel(strings.TrimSpace(q.Text))
	text.Wrapping = fyne.TextWrapWord
	if q.YesNo {
		d := escCustomConfirm("Trust this server?", "Trust and Connect", "Cancel", text, func(ok bool) {
			if ok {
				reply("yes", true)
			} else {
				reply("", false)
			}
		}, s.win)
		d.Resize(fyne.NewSize(560, d.MinSize().Height))
		s.present(d, nil) // no Enter: trusting a server is chosen by hand; Esc cancels
		s.lastDialog = d
		return
	}
	title, entry := "Git asks", s.newDialogEntry()
	if q.Secret {
		title, entry.Password = "Password", true
		if strings.Contains(strings.ToLower(q.Text), "passphrase") {
			title = "Key passphrase"
		}
	}
	d := escForm(title, "OK", "Cancel", []*widget.FormItem{widget.NewFormItem("", text), widget.NewFormItem("", entry)},
		func(ok bool) {
			if ok {
				reply(entry.Text, true)
			} else {
				reply("", false) // what was typed goes nowhere
			}
		}, s.win)
	d.Resize(fyne.NewSize(480, d.MinSize().Height))
	s.submitForm(d, entry) // Enter answers, Esc cancels (a dialog's Hide answers false)
	s.win.Canvas().Focus(entry)
	s.lastDialog = d
}
