package shell

import (
	"fmt"
	"github.com/obstinix/PyxForge/internal/ui/kit"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

// askConflict asks what to do with a document whose file changed on disk while it had unsaved
// changes (PXF-DEF-012): reload the file (dropping the changes), overwrite the file with them,
// or keep editing, in which case saving asks again. One question per document at a time.
func (s *Shell) askConflict(d *nativeDoc) {
	if s.nat == nil || d.asking {
		return
	}
	d.asking = true
	name := d.name()
	var dlg dialog.Dialog
	done := func() { dlg.Hide(); d.asking = false }
	reload := newDialogButton(s, "Reload from Disk", func() { done(); s.nat.reload(d) })
	overwrite := newDialogButton(s, "Overwrite Disk", func() { done(); s.nat.write(d) })
	keep := newDialogButton(s, "Keep Editing", done)
	keep.Variant = kit.ButtonCTA
	text := widget.NewLabel(fmt.Sprintf("%s has unsaved changes here, and another program changed it on disk.\n\n"+
		"Reload from Disk shows the file as it is now and drops your changes. Overwrite Disk saves your "+
		"changes over it. Keep Editing leaves both as they are; saving asks again.", name))
	text.Wrapping = fyne.TextWrapWord
	content := container.NewVBox(text, container.NewHBox(reload, overwrite, keep))
	dlg = dialog.NewCustomWithoutButtons(name+" changed on disk", content, s.win)
	dlg.Resize(fyne.NewSize(520, dlg.MinSize().Height))
	dlg.SetOnClosed(func() { d.asking = false }) // Esc too: the question can come again
	s.present(dlg, done)                         // Enter: Keep Editing, the safe choice
	s.lastDialog = dlg
}

// checkFiles looks for open files changed on disk: when the window comes back to the front
// and when the explorer sees a file change.
func (s *Shell) checkFiles() {
	if s.nat == nil {
		return
	}
	for _, d := range s.nat.docs {
		s.nat.checkDisk(d)
	}
}

// openInEditor reports whether any of paths is open in the editor.
func (s *Shell) openInEditor(paths []string) bool {
	if s.nat == nil {
		return false
	}
	for _, p := range paths {
		if s.nat.docFor(p) != nil {
			return true
		}
	}
	return false
}
