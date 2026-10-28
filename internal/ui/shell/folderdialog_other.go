//go:build !windows

package shell

import (
	"path/filepath"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"github.com/obstinix/PyxForge/internal/ui/notifications"
)

func (s *Shell) pickFolder(title string, selected func(string)) {
	d := dialog.NewFolderOpen(func(u fyne.ListableURI, err error) {
		if err != nil {
			s.Notify(notifications.Error, title, err.Error())
			return
		}
		if u != nil {
			selected(filepath.FromSlash(u.Path()))
		}
	}, s.win)
	if here, err := storage.ListerForURI(storage.NewFileURI(s.processFolder())); err == nil {
		d.SetLocation(here)
	}
	s.present(d, nil)
}
