package shell

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/widget"

	"github.com/obstinix/PyxForge/internal/trash"
	"github.com/obstinix/PyxForge/internal/ui/notifications"
	"github.com/obstinix/PyxForge/internal/workspace"
)

// moveToTrash is trash.Move; tests replace it so as not to fill the system's trash.
var moveToTrash = trash.Move

// trashName is what the system calls its trash, for menu items and messages.
func trashName() string {
	if runtime.GOOS == "windows" {
		return "Recycle Bin"
	}
	return "Trash"
}

// explorerMenu is the menu for a right-click on the explorer's row for path.
func (s *Shell) explorerMenu(path string, dir bool) *fyne.Menu {
	folder := filepath.Dir(path)
	if dir {
		folder = path
	}
	var items []*fyne.MenuItem
	if !dir {
		items = append(items, fyne.NewMenuItem("Open", func() { s.OpenFile(path) }), fyne.NewMenuItemSeparator())
	}
	items = append(items,
		fyne.NewMenuItem("New File…", func() { s.newEntryIn(false, folder) }),
		fyne.NewMenuItem("New Folder…", func() { s.newEntryIn(true, folder) }),
		fyne.NewMenuItemSeparator(),
		withKey(fyne.NewMenuItem("Rename…", func() { s.renameEntry(path) }), fyne.KeyF2),
		withKey(fyne.NewMenuItem("Move to "+trashName(), func() { s.trashEntry(path, dir) }), fyne.KeyDelete),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("Copy Path", func() { s.app.Clipboard().SetContent(path) }),
		fyne.NewMenuItem("Copy Relative Path", func() {
			if rel, err := filepath.Rel(s.root, path); err == nil {
				s.app.Clipboard().SetContent(filepath.ToSlash(rel))
			}
		}),
		fyne.NewMenuItem(revealLabel(), func() { s.revealInFileManager(path, dir) }),
	)
	return fyne.NewMenu("", items...)
}

// withKey shows key beside a menu item: the explorer handles it itself while it has focus.
func withKey(it *fyne.MenuItem, key fyne.KeyName) *fyne.MenuItem {
	it.Shortcut = &desktop.CustomShortcut{KeyName: key}
	return it
}

// revealLabel names the system's file manager the way its users know it.
func revealLabel() string {
	switch runtime.GOOS {
	case "windows":
		return "Reveal in File Explorer"
	case "darwin":
		return "Reveal in Finder"
	}
	return "Open Containing Folder"
}

// revealInFileManager opens the folder that holds path (or the folder itself) in the
// system's file manager.
func (s *Shell) revealInFileManager(path string, dir bool) {
	if !dir {
		path = filepath.Dir(path)
	}
	s.openURL(storage.NewFileURI(path).String())
}

// openUnder lists the editor tabs showing path or, for a folder, a file in it.
func (s *Shell) openUnder(path string) (bufs []*bufTab, other []*container.TabItem) {
	under := func(p string) bool { return p != "" && (sameFile(p, path) || workspace.Within(path, p)) }
	if s.ed != nil && s.ed.sess != nil {
		for _, bt := range s.ed.bufs {
			if under(bt.path) {
				bufs = append(bufs, bt)
			}
		}
	}
	for p, it := range s.open {
		if under(p) {
			other = append(other, it)
		}
	}
	return bufs, other
}

// releaseFiles closes the editor tabs on path, or on files in it, and then runs change. A tab
// with unsaved changes stops it instead: those changes would be written back to the old place.
// change gets the files whose tabs were closed.
func (s *Shell) releaseFiles(path, action string, change func(closed []string)) {
	bufs, other := s.openUnder(path)
	var closed []string
	for _, bt := range bufs {
		if bt.modified {
			s.Notify(notifications.Warning, "Save "+bt.name()+" first", fmt.Sprintf("It has unsaved changes; save or close it, then %s again.", action))
			return
		}
		closed = append(closed, bt.path)
	}
	for _, it := range other {
		closed = append(closed, s.tabFile(it))
		s.editors.Remove(it)
		s.editors.OnClosed(it)
	}
	if len(bufs) == 0 {
		change(closed)
		return
	}
	sess := s.ed.sess
	s.ed.do(func() {
		for _, bt := range bufs {
			if err := sess.CloseBuffer(bt.buf, false, false); err != nil {
				s.dispatch(func() { s.Notify(notifications.Error, "Cannot "+action, err.Error()) })
				return
			}
		}
		s.dispatch(func() { change(closed) })
	})
}

// renameEntry asks for a new name for path and renames it; open tabs follow the file.
func (s *Shell) renameEntry(path string) {
	entry := s.newDialogEntry()
	entry.SetText(filepath.Base(path))
	d := escForm("Rename "+filepath.Base(path), "Rename", "Cancel",
		[]*widget.FormItem{widget.NewFormItem("New name", entry)}, func(ok bool) {
			if !ok || strings.TrimSpace(entry.Text) == filepath.Base(path) {
				return
			}
			s.renameTo(path, entry.Text)
		}, s.win)
	d.Resize(fyne.NewSize(420, d.MinSize().Height))
	s.submitForm(d, entry)
	s.win.Canvas().Focus(entry)
	s.lastDialog = d
}

// renameTo renames path to name, closing its tabs first and opening them again at the new place.
func (s *Shell) renameTo(path, name string) {
	s.releaseFiles(path, "rename it", func(closed []string) {
		to, err := s.explorer.Rename(path, name)
		if err != nil {
			s.Notify(notifications.Error, "Cannot rename "+filepath.Base(path), err.Error())
			to = path // the files stayed where they were
		}
		for _, f := range closed {
			s.openFile(movedTo(f, path, to), false)
		}
		if err == nil {
			s.explorer.Reveal(to)
		}
	})
}

// movedTo is where file is once from has been renamed to: from itself, or a file in it.
func movedTo(file, from, to string) string {
	if rel, err := filepath.Rel(from, file); err == nil && rel != "." {
		return filepath.Join(to, rel)
	}
	return to
}

// trashEntry asks, then moves path to the system's trash, closing its tabs first.
func (s *Shell) trashEntry(path string, dir bool) {
	what := "file"
	if dir {
		what = "folder and everything in it"
	}
	move := func() {
		s.releaseFiles(path, "move it", func([]string) {
			if err := moveToTrash(path); err != nil {
				s.Notify(notifications.Error, "Not moved to the "+trashName(), filepath.Base(path)+": "+err.Error())
				return
			}
			s.explorer.Reload()
			s.Notify(notifications.Success, "Moved to the "+trashName(), filepath.Base(path)+" can be restored from there.")
		})
	}
	d := escConfirm("Move to "+trashName(),
		fmt.Sprintf("Move the %s %s to the %s?", what, filepath.Base(path), trashName()),
		func(ok bool) {
			if ok {
				move()
			}
		}, s.win)
	s.present(d, func() { d.Hide(); move() })
	s.lastDialog = d
}

// copyActivePath copies the active file's path, or its path in the workspace with forward
// slashes, to the clipboard.
func (s *Shell) copyActivePath(relative bool) {
	p := s.activeFile()
	if p == "" {
		s.Notify(notifications.Info, "No file to copy the path of", "Open a file in the editor first.")
		return
	}
	if relative {
		rel, err := filepath.Rel(s.root, p)
		if err != nil || s.root == "" || strings.HasPrefix(rel, "..") {
			s.Notify(notifications.Info, "Not in the workspace", p+" is outside the open folder; Copy Path copies all of it.")
			return
		}
		p = filepath.ToSlash(rel)
	}
	s.app.Clipboard().SetContent(p)
	s.Notify(notifications.Success, "Copied", p)
}
