package shell

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/widget"
	"github.com/obstinix/PyxForge/internal/ui/notifications"
)

// openFileDialog asks for a file anywhere on disk and opens it in the editor (File > Open
// File…, Ctrl+O). It starts in the active file's folder, else the workspace.
func (s *Shell) openFileDialog() {
	d := dialog.NewFileOpen(func(r fyne.URIReadCloser, err error) {
		if err != nil {
			s.Notify(notifications.Error, "Cannot open file", err.Error())
			return
		}
		if r == nil {
			return // cancelled
		}
		p := filepath.FromSlash(r.URI().Path())
		_ = r.Close()
		s.OpenFile(p)
	}, s.win)
	if here, err := storage.ListerForURI(storage.NewFileURI(s.startFolder())); err == nil {
		d.SetLocation(here)
	}
	s.present(d, nil)
	d.Resize(fyne.NewSize(720, 480)) // after Show: a FileDialog has no window to size before it
}

// startFolder is where file dialogs start: the active file's folder, else the workspace.
func (s *Shell) startFolder() string {
	if p := s.activeFile(); p != "" {
		if fi, err := os.Stat(filepath.Dir(p)); err == nil && fi.IsDir() {
			return filepath.Dir(p)
		}
	}
	return s.root
}

// saveAsDialog asks where to save the active editor file (File > Save As…, Ctrl+Shift+S): a
// path, with Browse… to pick its folder. Fyne's own save dialog is not used because it opens,
// and so empties, the chosen file before PyxForge has written anything.
func (s *Shell) saveAsDialog() {
	bt := s.activeBuffer()
	if bt == nil {
		s.Notify(notifications.Info, "Nothing to save", "Save As works on a file open in the editor.")
		return
	}
	entry := s.newDialogEntry()
	entry.SetText(bt.path)
	var d dialog.Dialog
	submit := func() {
		p := strings.TrimSpace(entry.Text)
		if p == "" {
			return
		}
		if !filepath.IsAbs(p) {
			p = filepath.Join(filepath.Dir(bt.path), p)
		}
		d.Hide()
		s.saveAsTo(bt, filepath.Clean(p))
	}
	entry.OnSubmitted = func(string) { submit() }
	browse := widget.NewButton("Browse…", func() {
		fd := dialog.NewFolderOpen(func(u fyne.ListableURI, err error) {
			if err != nil || u == nil {
				return
			}
			entry.SetText(filepath.Join(filepath.FromSlash(u.Path()), filepath.Base(entry.Text)))
		}, s.win)
		if here, err := storage.ListerForURI(storage.NewFileURI(filepath.Dir(entry.Text))); err == nil {
			fd.SetLocation(here)
		}
		s.present(fd, nil)
	})
	d = dialog.NewForm("Save As", "Save", "Cancel",
		[]*widget.FormItem{widget.NewFormItem("File", container.NewBorder(nil, nil, nil, browse, entry))},
		func(ok bool) {
			if ok {
				submit()
			}
		}, s.win)
	d.Resize(fyne.NewSize(620, d.MinSize().Height))
	s.present(d, submit)
	s.win.Canvas().Focus(entry)
	s.lastDialog = d
}

// saveAsTo writes a buffer to path and makes path its file. An existing file is replaced only
// after the user says so.
func (s *Shell) saveAsTo(bt *bufTab, path string) {
	if sameFile(path, bt.path) {
		s.saveCurrent()
		return
	}
	if fi, err := os.Stat(path); err == nil {
		if fi.IsDir() {
			s.Notify(notifications.Error, "Cannot save there", path+" is a folder.")
			return
		}
		d := dialog.NewConfirm("Replace "+filepath.Base(path)+"?",
			"A file with this name already exists in "+filepath.Dir(path)+". Saving replaces it.",
			func(ok bool) {
				if ok {
					s.runSaveAs(bt, path, true)
				}
			}, s.win)
		d.SetConfirmText("Replace")
		s.present(d, func() { d.Hide(); s.runSaveAs(bt, path, true) })
		s.lastDialog = d
		return
	}
	s.runSaveAs(bt, path, false)
}

func (s *Shell) runSaveAs(bt *bufTab, path string, overwrite bool) {
	sess, buf := s.ed.sess, bt.buf
	s.ed.do(func() {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			s.dispatch(func() { s.Notify(notifications.Error, "Save As failed", err.Error()) })
			return
		}
		if err := sess.SaveAs(buf, path, overwrite); err != nil {
			s.dispatch(func() { s.Notify(notifications.Error, "Save As failed", err.Error()) })
		}
	})
}

// activeBuffer is the editor tab's buffer, or nil when the selected tab is not a Neovim
// buffer (Settings, a binary file, no tab).
func (s *Shell) activeBuffer() *bufTab {
	if s.ed == nil || s.ed.sess == nil {
		return nil
	}
	return s.ed.byTab[s.editors.Selected()]
}

// closeAllTabs closes every editor tab (File > Close All, Ctrl+Shift+W). Unsaved changes are
// offered for saving first; a file that changed on disk under them stops the close.
func (s *Shell) closeAllTabs() {
	var names []string
	if s.ed != nil && s.ed.sess != nil {
		modified, err := s.ed.sess.ModifiedFiles()
		if err != nil {
			s.Notify(notifications.Error, "Tabs not closed", err.Error())
			return
		}
		for _, n := range modified {
			names = append(names, filepath.Base(n))
		}
	}
	if len(names) == 0 {
		s.closeTabs(false)
		return
	}
	sort.Strings(names)
	s.confirmUnsaved(fmt.Sprintf("Save changes to %s?", strings.Join(names, ", ")),
		func() {
			left, err := s.ed.sess.SaveAll()
			switch {
			case err != nil:
				s.Notify(notifications.Error, "Not saved; the tabs stay open", err.Error())
			case len(left) > 0: // changed on disk: the user is asked about each first
			default:
				s.closeTabs(false)
			}
		},
		func() { s.closeTabs(true) })
}

// closeTabs closes every editor tab; with discard, unsaved changes are dropped.
func (s *Shell) closeTabs(discard bool) {
	var bufs []int
	for _, it := range append([]*container.TabItem(nil), s.editors.Items...) {
		if s.ed != nil && s.ed.sess != nil {
			if bt := s.ed.byTab[it]; bt != nil {
				bufs = append(bufs, bt.buf)
				continue
			}
		}
		s.editors.Remove(it)
		s.editors.OnClosed(it)
	}
	if len(bufs) == 0 {
		return
	}
	sess := s.ed.sess
	s.ed.do(func() {
		for _, b := range bufs {
			if err := sess.CloseBuffer(b, false, discard); err != nil {
				s.dispatch(func() { s.Notify(notifications.Error, "A tab did not close", err.Error()) })
				return
			}
		}
	})
}
