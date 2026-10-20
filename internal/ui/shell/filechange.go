package shell

import (
	"fmt"
	"path/filepath"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"github.com/obstinix/PyxForge/internal/neovim"
	"github.com/obstinix/PyxForge/internal/ui/notifications"
)

// fileChanged answers Neovim's report that an open file changed on disk (PXF-DEF-012). Neovim
// reloads a buffer without unsaved changes by itself; for one with unsaved changes it keeps
// the buffer as it is, writes nothing, and PyxForge asks the user here.
func (s *Shell) fileChanged(e neovim.Event) {
	name := filepath.Base(e.Name)
	switch e.Message {
	case "deleted":
		s.Notify(notifications.Warning, name+" was deleted on disk",
			"The editor keeps its text. Saving writes the file again.")
	case "conflict":
		// A report can arrive after the question it raised was answered (a save made while it
		// showed asks again): ask only if Neovim still has the conflict, checked on the editor's
		// queue, behind any Reload or Overwrite already sent.
		if s.ed == nil || s.ed.sess == nil || s.conflicts[e.Buffer] {
			return
		}
		sess, buf, path := s.ed.sess, e.Buffer, e.Name
		s.ed.do(func() {
			if on, err := sess.Conflicted(buf); err == nil && on {
				s.dispatch(func() { s.askConflict(buf, path) })
			}
		})
	}
}

// askConflict asks what to do with a buffer whose file changed on disk while it had unsaved
// changes: reload the file (dropping the changes), overwrite the file with them, or keep
// editing, in which case saving asks again. One question per buffer at a time.
func (s *Shell) askConflict(buf int, path string) {
	if s.ed == nil || s.ed.sess == nil || s.conflicts[buf] {
		return
	}
	s.conflicts[buf] = true
	sess := s.ed.sess
	name := filepath.Base(path)
	var d dialog.Dialog
	done := func() { d.Hide(); delete(s.conflicts, buf) }
	run := func(what string, f func(int) error) {
		s.ed.do(func() {
			if err := f(buf); err != nil {
				s.dispatch(func() { s.Notify(notifications.Error, what+" failed", err.Error()) })
			}
		})
	}
	reload := widget.NewButton("Reload from Disk", func() { done(); run("Reload", sess.Reload) })
	overwrite := widget.NewButton("Overwrite Disk", func() { done(); run("Overwrite", sess.Overwrite) })
	keep := widget.NewButton("Keep Editing", done)
	keep.Importance = widget.HighImportance
	text := widget.NewLabel(fmt.Sprintf("%s has unsaved changes here, and another program changed it on disk.\n\n"+
		"Reload from Disk shows the file as it is now and drops your changes. Overwrite Disk saves your "+
		"changes over it. Keep Editing leaves both as they are; saving asks again.", name))
	text.Wrapping = fyne.TextWrapWord
	content := container.NewVBox(text, container.NewHBox(reload, overwrite, keep))
	d = dialog.NewCustomWithoutButtons(name+" changed on disk", content, s.win)
	d.Resize(fyne.NewSize(520, d.MinSize().Height))
	d.SetOnClosed(func() { delete(s.conflicts, buf) }) // Esc too: the question can come again
	s.present(d, done)                                 // Enter: Keep Editing, the safe choice
	s.lastDialog = d
}

// checkFiles makes the editor look for open files changed on disk: when the window comes back
// to the front and when the explorer sees a file change.
func (s *Shell) checkFiles() {
	if s.ed == nil || s.ed.sess == nil {
		return
	}
	sess := s.ed.sess
	s.ed.do(func() { _ = sess.CheckFiles() })
}

// openInEditor reports whether any of paths is open in the editor.
func (s *Shell) openInEditor(paths []string) bool {
	if s.ed == nil {
		return false
	}
	for _, p := range paths {
		if s.ed.tabFor(p) != nil {
			return true
		}
	}
	return false
}
