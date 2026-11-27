package shell

import (
	"os"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

// sameFile reports whether two paths name the same file: spelled alike (ignoring case), or,
// for files that exist, the same file on disk, so that one path spelled two ways (a Windows
// 8.3 short name and its long name, as Git and a temporary folder can give) still matches.
func sameFile(a, b string) bool {
	if strings.EqualFold(filepath.Clean(a), filepath.Clean(b)) {
		return true
	}
	fa, err := os.Stat(a)
	if err != nil {
		return false
	}
	fb, err := os.Stat(b)
	return err == nil && os.SameFile(fa, fb)
}

// confirmUnsaved asks Save / Don't Save / Cancel.
func (s *Shell) confirmUnsaved(question string, save, discard func()) {
	var d dialog.Dialog
	saveBtn := newDialogButton(s, "Save", nil, func() { d.Hide(); save() })
	saveBtn.Importance = widget.HighImportance
	discardBtn := newDialogButton(s, "Don't Save", nil, func() { d.Hide(); discard() })
	cancelBtn := newDialogButton(s, "Cancel", nil, func() { d.Hide() })
	content := container.NewVBox(widget.NewLabel("Your changes will be lost if you don't save them."),
		container.NewHBox(discardBtn, cancelBtn, saveBtn))
	d = dialog.NewCustomWithoutButtons(question, content, s.win)
	s.present(d, func() { d.Hide(); save() }) // Enter saves, as the highlighted button says
	s.lastDialog = d
}

// confirmQuit runs before the window closes: unsaved documents are offered for saving, then
// every process the shell started is stopped so none outlives the window.
func (s *Shell) confirmQuit() {
	quit := func() {
		s.Shutdown()
		s.win.Close()
	}
	if s.nat != nil {
		s.nativeQuit(quit)
		return
	}
	quit()
}

// Shutdown saves the workspace layout and ends every process the shell started. The window's
// close path runs it, and so does the app when it is told to stop by a signal (Ctrl+C in the
// terminal that started it, SIGTERM), which would otherwise leave QEMU running.
func (s *Shell) Shutdown() {
	current := s.group.currentShell()
	if current == nil {
		return
	}
	current.shutdown()
}

func (s *Shell) shutdown() {
	if s.closed.Swap(true) {
		return
	}
	s.saveState()
	s.stopAll()
	s.group.clear(s)
}

func (s *Shell) stopAll() {
	if s.gitp != nil && s.gitp.netStop != nil {
		s.gitp.netStop()
	}
	if s.cloneStop != nil {
		s.cloneStop()
	}
	s.stopToolchainInstalls()
	s.buildp.shutdown()
	s.mach.shutdown()
	s.explorer.Close()
	s.stopEditor()
	s.term.stop()
}

// StopAll ends every process the current shell started without closing the window.
func (s *Shell) StopAll() {
	if current := s.group.currentShell(); current != nil {
		current.stopAll()
	}
}

// stopEditor ends the editor's language servers.
func (s *Shell) stopEditor() {
	if s.nat != nil {
		s.nat.stop()
	}
}

// runChord runs the shell command bound to a shortcut, for keys the editor hands back.
func (s *Shell) runChord(sc fyne.Shortcut) bool {
	if run, ok := s.chordRuns[sc.ShortcutName()]; ok {
		run()
		return true
	}
	return false
}
