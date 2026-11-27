package shell

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/obstinix/PyxForge/internal/ui/notifications"
)

// nativeSaveAs asks where to save a native document, then writes it there; an existing file
// is replaced only after the user says so.
func (s *Shell) nativeSaveAs(d *nativeDoc) {
	if s.editors.Selected() != d.item {
		s.selectTab(d.item, false)
	}
	start := d.path
	if start == "" {
		start = filepath.Join(s.startFolder(), "untitled.txt")
	}
	entry := s.newDialogEntry()
	entry.SetText(start)
	var dlg dialog.Dialog
	write := func(p string) {
		s.nat.saveAs(d, p)
	}
	submit := func() {
		p := strings.TrimSpace(entry.Text)
		if p == "" {
			return
		}
		if !filepath.IsAbs(p) {
			p = filepath.Join(filepath.Dir(start), p)
		}
		p = filepath.Clean(p)
		dlg.Hide()
		if d.path != "" && sameFile(p, d.path) {
			s.nat.save(d)
			return
		}
		if fi, err := os.Stat(p); err == nil {
			if fi.IsDir() {
				s.Notify(notifications.Error, "Cannot save there", p+" is a folder.")
				return
			}
			c := escConfirm("Replace "+filepath.Base(p)+"?",
				"A file with this name already exists in "+filepath.Dir(p)+". Saving replaces it.",
				func(ok bool) {
					if ok {
						write(p)
					}
				}, s.win)
			c.SetConfirmText("Replace")
			s.present(c, func() { c.Hide(); write(p) })
			s.lastDialog = c
			return
		}
		write(p)
	}
	entry.OnSubmitted = func(string) { submit() }
	dlg = escForm("Save As", "Save", "Cancel", []*widget.FormItem{widget.NewFormItem("File", entry)},
		func(ok bool) {
			if ok {
				submit()
			}
		}, s.win)
	dlg.Resize(fyne.NewSize(620, dlg.MinSize().Height))
	s.present(dlg, submit)
	s.win.Canvas().Focus(entry)
	s.lastDialog = dlg
}

// nativeConfirm runs then after offering to save every document with unsaved changes. When
// a save does not work out (save already said why, or a new file is being named) it stops.
func (s *Shell) nativeConfirm(then func()) {
	mod := s.nat.modified()
	if len(mod) == 0 {
		then()
		return
	}
	var names []string
	for _, d := range mod {
		names = append(names, displayName(d.path))
	}
	s.confirmUnsaved(fmt.Sprintf("Save changes to %s?", strings.Join(names, ", ")), func() {
		if s.nat.saveAll() {
			then()
		}
	}, then)
}

// nativeQuit offers to save unsaved documents, then quits.
func (s *Shell) nativeQuit(quit func()) { s.nativeConfirm(quit) }

// nativeRename asks for a symbol's new name and renames it through the language server.
func (s *Shell) nativeRename(d *nativeDoc) {
	if d.client == nil {
		s.Notify(notifications.Info, "No language server", "Rename Symbol needs a language server for this file type.")
		return
	}
	word := d.ed.Buf.Slice(d.ed.WordAt(d.ed.Cur))
	entry := s.newDialogEntry()
	entry.SetText(word)
	dlg := escForm("Rename Symbol", "Rename", "Cancel", []*widget.FormItem{widget.NewFormItem("New name", entry)},
		func(ok bool) {
			if ok {
				s.nat.rename(d, entry.Text)
			}
		}, s.win)
	dlg.Resize(fyne.NewSize(420, dlg.MinSize().Height))
	s.submitForm(dlg, entry)
	s.win.Canvas().Focus(entry)
	s.lastDialog = dlg
}
