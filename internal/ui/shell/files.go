package shell

import (
	"os"
	"path/filepath"
	"slices"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
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
	return s.processFolder()
}

// saveAsDialog asks where to save the active editor file (File > Save As…, Ctrl+Shift+S): a
// path, with Browse… to pick its folder. Fyne's own save dialog is not used because it opens,
// and so empties, the chosen file before PyxForge has written anything.
func (s *Shell) saveAsDialog() {
	if d := s.activeDoc(); d != nil {
		s.nativeSaveAs(d)
		return
	}
	s.Notify(notifications.Info, "Nothing to save", "Save As works on a file open in the editor.")
}

// closeAllTabs closes every editor tab (File > Close All, Ctrl+Shift+W). Unsaved changes are
// offered for saving first.
func (s *Shell) closeAllTabs() {
	if s.nat != nil {
		s.nativeCloseAll()
		return
	}
	for _, it := range slices.Clone(s.editors.Items) {
		s.editors.Remove(it)
		s.editors.OnClosed(it)
	}
}

// displayName is how a document is named in a question: its file's name, or for a new file
// that has none yet, "an untitled file".
func displayName(path string) string {
	if path == "" {
		return "an untitled file"
	}
	return filepath.Base(path)
}

// newUntitled opens a new, untitled file in the editor (File > New File, Ctrl+N); saving it
// asks where to.
func (s *Shell) newUntitled() {
	if !s.startNative() {
		s.Notify(notifications.Error, "No editor", "Editing is turned off in this window, so there is nowhere to write.")
		return
	}
	s.nat.newUntitled()
}
