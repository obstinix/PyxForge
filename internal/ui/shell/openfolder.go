package shell

import (
	"os"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"github.com/obstinix/PyxForge/internal/proc"
	"github.com/obstinix/PyxForge/internal/ui/notifications"
)

// openFolder asks for a folder and opens it in a new PyxForge window. A window holds one
// workspace, so this is how someone who started PyxForge from the Start menu reaches a project.
func (s *Shell) openFolder() {
	d := dialog.NewFolderOpen(func(u fyne.ListableURI, err error) {
		if err != nil || u == nil {
			return
		}
		s.openFolderAt(filepath.FromSlash(u.Path()))
	}, s.win)
	if here, err := storage.ListerForURI(storage.NewFileURI(s.root)); err == nil {
		d.SetLocation(here)
	}
	s.present(d, nil)
}

// openFolderAt starts a new window on dir. The window is its own process, so closing this
// one does not close it.
func (s *Shell) openFolderAt(dir string) {
	start := s.startWindow
	if start == nil {
		start = startWindow
	}
	if err := start(dir); err != nil {
		s.Notify(notifications.Error, "Could not open the folder", err.Error())
		return
	}
	s.logf("Opened %s in a new window", dir)
}

// startWindow runs PyxForge on dir. From the console build (pyxforge.com) it starts the
// windowed PyxForge.exe beside it when there is one, so no console comes with the window.
func startWindow(dir string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if strings.EqualFold(filepath.Ext(exe), ".com") {
		if gui := filepath.Join(filepath.Dir(exe), "PyxForge.exe"); fileExists(gui) {
			exe = gui
		}
	}
	return proc.StartDetached(exe, dir)
}

func fileExists(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.Mode().IsRegular()
}
