package shell

import (
	"os"
	"path/filepath"
	"slices"

	"github.com/obstinix/PyxForge/internal/ui/commandpalette"
	"github.com/obstinix/PyxForge/internal/ui/icons"
	"github.com/obstinix/PyxForge/internal/ui/notifications"
)

// Recent files and folders (PXF-DEF-020): kept per user in the app's preferences, newest
// first, across workspaces. Files and folders open in this window.
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
	s.refreshWelcomeRecent()
}

func (s *Shell) dropRecent(key, path string) {
	p := s.app.Preferences()
	p.SetStringList(key, slices.DeleteFunc(p.StringList(key), func(o string) bool { return sameFile(o, path) }))
	s.refreshWelcomeRecent()
}

// recentFolders are the recent folders other than this window's own.
func (s *Shell) recentFolders() []string {
	var out []string
	for _, d := range s.app.Preferences().StringList(prefRecentFolders) {
		if !sameFile(d, s.root) {
			out = append(out, d)
		}
	}
	return out
}

// chooseRecent is File > Open Recent…: the recent files and folders in the palette, to pick
// from or search, and Clear Recent.
func (s *Shell) chooseRecent() {
	p := s.app.Preferences()
	var items []commandpalette.Item
	for _, f := range p.StringList(prefRecentFiles) {
		items = append(items, commandpalette.Item{Title: filepath.Base(f), Detail: filepath.Dir(f), Icon: icons.FileText,
			Run: func() { s.openRecent(prefRecentFiles, f) }})
	}
	for _, d := range s.recentFolders() {
		items = append(items, commandpalette.Item{Title: filepath.Base(d), Detail: d + " (folder)",
			Icon: icons.Folder, Run: func() { s.openRecent(prefRecentFolders, d) }})
	}
	if len(items) == 0 {
		s.Notify(notifications.Info, "Nothing recent yet", "Files and folders you open are listed here.")
		return
	}
	items = append(items, commandpalette.Item{Title: "Clear Recent", Icon: icons.X, Run: func() {
		p.SetStringList(prefRecentFiles, nil)
		p.SetStringList(prefRecentFolders, nil)
		s.refreshWelcomeRecent()
	}})
	s.palette.Show("Open a recent file or folder", items)
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
