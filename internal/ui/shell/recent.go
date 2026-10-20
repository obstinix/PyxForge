package shell

import (
	"os"
	"path/filepath"
	"slices"

	"fyne.io/fyne/v2"
	"github.com/obstinix/PyxForge/internal/ui/notifications"
)

// Recent files and folders (PXF-DEF-020): kept per user in the app's preferences, newest
// first, across workspaces. A file opens in this window, a folder in a new one.
const (
	prefRecentFiles   = "recent.files"
	prefRecentFolders = "recent.folders"
	maxRecent         = 10
)

// addRecent puts path first in a recent list.
func (s *Shell) addRecent(key, path string) {
	if path == "" {
		return
	}
	p := s.app.Preferences()
	list := slices.DeleteFunc(p.StringList(key), func(o string) bool { return sameFile(o, path) })
	list = append([]string{path}, list...)
	if len(list) > maxRecent {
		list = list[:maxRecent]
	}
	p.SetStringList(key, list)
}

func (s *Shell) dropRecent(key, path string) {
	p := s.app.Preferences()
	p.SetStringList(key, slices.DeleteFunc(p.StringList(key), func(o string) bool { return sameFile(o, path) }))
}

// recentMenu is File > Open Recent.
func (s *Shell) recentMenu() *fyne.MenuItem {
	p := s.app.Preferences()
	var items []*fyne.MenuItem
	for _, f := range p.StringList(prefRecentFiles) {
		items = append(items, fyne.NewMenuItem(recentLabel(f), func() { s.openRecent(prefRecentFiles, f) }))
	}
	folders := p.StringList(prefRecentFolders)
	if len(items) > 0 && len(folders) > 0 {
		items = append(items, fyne.NewMenuItemSeparator())
	}
	for _, d := range folders {
		if sameFile(d, s.root) {
			continue // this window's own
		}
		items = append(items, fyne.NewMenuItem(recentLabel(d)+" (folder)", func() { s.openRecent(prefRecentFolders, d) }))
	}
	if len(items) > 0 {
		items = append(items, fyne.NewMenuItemSeparator(), fyne.NewMenuItem("Clear Recent", func() {
			p.SetStringList(prefRecentFiles, nil)
			p.SetStringList(prefRecentFolders, nil)
		}))
	}
	it := fyne.NewMenuItem("Open Recent", nil)
	if len(items) == 0 {
		it.Disabled = true
		return it
	}
	it.ChildMenu = fyne.NewMenu("", items...)
	return it
}

// recentLabel names a recent path by its last element and the folder holding it.
func recentLabel(p string) string {
	return filepath.Base(p) + "   " + filepath.Dir(p)
}

// openRecent opens a recent file or folder; one that is gone is said so and dropped.
func (s *Shell) openRecent(key, path string) {
	fi, err := os.Stat(path)
	if err != nil || fi.IsDir() != (key == prefRecentFolders) {
		s.dropRecent(key, path)
		s.Notify(notifications.Warning, filepath.Base(path)+" is no longer there", path+" was moved or deleted, so it was taken off the recent list.")
		return
	}
	if key == prefRecentFolders {
		s.openFolderAt(path)
		return
	}
	s.OpenFile(path)
}
