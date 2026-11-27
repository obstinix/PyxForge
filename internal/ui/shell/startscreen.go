package shell

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"github.com/obstinix/PyxForge/internal/ui/icons"
	"github.com/obstinix/PyxForge/internal/ui/kit"
)

// The start screen's state (owner review F1, F6).
const (
	prefPinnedFolders = "recent.pinned"
	// prefReopenLast opens the last workspace at start instead of the Welcome page
	// (Settings > Files & Explorer). Off by default: a start that opens nothing cannot freeze.
	prefReopenLast = "files.reopenLast"
)

// broadFolder says why dir is too broad to open without asking: a drive's root, the file
// system's root, or the home folder. Listing and watching any of them means every file the
// user has, which is how a start could freeze (owner review F1).
func broadFolder(dir string) (string, bool) {
	if dir == "" {
		return "", false
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", false
	}
	abs = filepath.Clean(abs)
	if filepath.Dir(abs) == abs { // C:\, \server\share\, /
		if runtime.GOOS == "windows" {
			return "the whole drive " + abs, true
		}
		return "the whole file system", true
	}
	if home, err := os.UserHomeDir(); err == nil && sameFile(abs, home) {
		return "your whole home folder", true
	}
	return "", false
}

// IsBroadFolder reports a folder PyxForge asks about before opening it (broadFolder).
func IsBroadFolder(dir string) bool {
	_, broad := broadFolder(dir)
	return broad
}

// confirmBroad asks before opening a folder that broad. Cancel is the default: Enter and
// Escape both cancel, and only a click on Open Anyway goes on.
func (s *Shell) confirmBroad(dir, what string, open func()) {
	var d dialog.Dialog
	cancel := newDialogButton(s, "Cancel", func() { d.Hide() })
	cancel.Variant = kit.ButtonCTA
	anyway := newDialogButton(s, "Open Anyway", func() { d.Hide(); open() })
	text := widget.NewLabel(fmt.Sprintf("%s is %s. PyxForge would list, search and watch every file in it, "+
		"which can take minutes and a lot of memory. Open a project folder inside it instead.", dir, what))
	text.Wrapping = fyne.TextWrapWord
	content := container.NewBorder(nil, container.NewHBox(cancel, anyway), nil, nil, text)
	d = dialog.NewCustomWithoutButtons("Open "+what+"?", content, s.win)
	d.Resize(fyne.NewSize(480, content.MinSize().Height+80))
	s.present(d, func() { d.Hide() }) // Enter cancels
	s.lastDialog = d
}

// OpenFolder opens dir as the workspace in this window, asking first if it is broad.
func (s *Shell) OpenFolder(dir string) { s.openFolderAt(dir) }

// ReopenFolder is the folder to open at start when the app is given none: the last workspace,
// if Reopen Last Workspace is on and it is still there; else "".
func ReopenFolder(a fyne.App) string {
	p := a.Preferences()
	if !p.Bool(prefReopenLast) {
		return ""
	}
	for _, d := range p.StringList(prefRecentFolders) {
		if fi, err := os.Stat(d); err == nil && fi.IsDir() {
			return d
		}
		break // only the last one: an older one was not what the user left
	}
	return ""
}

// pinned are the pinned recent folders.
func (s *Shell) pinned() []string {
	return slices.Clone(s.app.Preferences().StringList(prefPinnedFolders))
}

func (s *Shell) isPinned(dir string) bool {
	return slices.ContainsFunc(s.pinned(), func(p string) bool { return sameFile(p, dir) })
}

// togglePin pins a recent folder to the top of the Welcome page, or unpins it.
func (s *Shell) togglePin(dir string) {
	p := s.app.Preferences()
	was := s.isPinned(dir)
	list := slices.DeleteFunc(slices.Clone(s.pinned()), func(o string) bool { return sameFile(o, dir) })
	if !was {
		list = append([]string{dir}, list...)
	}
	p.SetStringList(prefPinnedFolders, list)
	s.refreshWelcomeRecent()
}

// welcomeFolders are the folders the Welcome page lists: pinned ones first, then recent ones.
func (s *Shell) welcomeFolders() []string {
	out := slices.Clone(s.pinned())
	for _, d := range s.app.Preferences().StringList(prefRecentFolders) {
		if !slices.ContainsFunc(out, func(o string) bool { return sameFile(o, d) }) {
			out = append(out, d)
		}
	}
	return out[:min(len(out), maxRecent+len(s.pinned()))]
}

// locate asks where a recent folder that is gone went, and puts the answer in its place.
func (s *Shell) locate(old string) {
	s.pickFolder("Locate "+filepath.Base(old), func(dir string) {
		p := s.app.Preferences()
		for _, key := range []string{prefRecentFolders, prefPinnedFolders} {
			list := slices.Clone(p.StringList(key))
			for i, o := range list {
				if sameFile(o, old) {
					list[i] = dir
				}
			}
			p.SetStringList(key, list)
		}
		s.refreshWelcomeRecent()
	})
}

// dropped opens what is dropped on the window: a folder as the workspace, files in the editor.
func (s *Shell) dropped(_ fyne.Position, uris []fyne.URI) {
	var files []string
	for _, u := range uris {
		if u.Scheme() != "file" {
			continue
		}
		p := u.Path()
		if runtime.GOOS == "windows" {
			p = filepath.FromSlash(strings.TrimPrefix(p, "/"))
		}
		fi, err := os.Stat(p)
		if err != nil {
			continue
		}
		if fi.IsDir() {
			s.openFolderAt(p) // the first folder wins; a window has one workspace
			return
		}
		files = append(files, p)
	}
	for _, f := range files {
		s.OpenFile(f)
	}
}

// recentRow is one folder on the Welcome page: its name and path, a pin, and Remove, or
// Locate… when it is gone.
func (s *Shell) recentRow(dir string) fyne.CanvasObject {
	_, err := os.Stat(dir)
	missing := err != nil
	name := welcomeLink(filepath.Base(dir), func() { s.openRecent(prefRecentFolders, dir) })
	detail := widget.NewLabel(dir)
	detail.Importance, detail.Truncation = widget.LowImportance, fyne.TextTruncateEllipsis
	pinLabel := "Pin " + filepath.Base(dir)
	if s.isPinned(dir) {
		pinLabel = "Unpin " + filepath.Base(dir)
	}
	pin := kit.NewIconButton(icons.Pin, pinLabel, func() { s.togglePin(dir) })
	pin.Selected = s.isPinned(dir)
	remove := kit.NewIconButton(icons.X, "Remove "+filepath.Base(dir)+" from Recent Projects", func() {
		s.dropRecent(prefRecentFolders, dir)
		if s.isPinned(dir) {
			s.togglePin(dir)
		}
	})
	actions := container.NewHBox(pin, remove)
	if missing {
		detail.SetText(dir + " (not found)")
		actions = container.NewHBox(kit.NewButton("Locate…", func() { s.locate(dir) }), remove)
	}
	return container.NewBorder(nil, nil, nil, container.NewCenter(actions), container.NewVBox(name, detail))
}

// App is the app the shell runs in, for review renders.
func (s *Shell) App() fyne.App { return s.app }

// ModalOpen reports whether a dialog is showing, for review renders.
func (s *Shell) ModalOpen() bool { return len(s.modals) > 0 }
