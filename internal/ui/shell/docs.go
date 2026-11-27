package shell

import (
	"os"
	"path/filepath"
	"runtime"

	"github.com/obstinix/PyxForge/internal/shipdocs"
	"github.com/obstinix/PyxForge/internal/ui/commandpalette"
	"github.com/obstinix/PyxForge/internal/ui/icons"
	"github.com/obstinix/PyxForge/internal/ui/notifications"
)

// Documents shipped with PyxForge (D22): Help opens them from disk, so it works offline. The
// Windows packages put them in a docs folder beside the program, the Linux ones in
// /usr/share/doc/pyxforge (inside the AppImage too); a source checkout has them where the
// repository keeps them.

// docPath finds a shipped document on disk, or "".
func docPath(name string) string {
	var repo string
	for _, d := range shipdocs.List {
		if d.Name == name {
			repo = d.Repo
		}
	}
	var dirs []string
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		dirs = append(dirs, filepath.Join(dir, "docs"), filepath.Join(dir, "..", "share", "doc", "pyxforge"))
	}
	if runtime.GOOS != "windows" {
		if appdir := os.Getenv("APPDIR"); appdir != "" {
			dirs = append(dirs, filepath.Join(appdir, "usr", "share", "doc", "pyxforge"))
		}
		dirs = append(dirs, "/usr/share/doc/pyxforge")
	}
	for _, d := range dirs {
		if p := filepath.Join(d, name); fileExists(p) {
			return p
		}
	}
	// A source checkout: walk up from the program and the working folder to the repository.
	if repo == "" {
		return ""
	}
	var starts []string
	if exe, err := os.Executable(); err == nil {
		starts = append(starts, filepath.Dir(exe))
	}
	if wd, err := os.Getwd(); err == nil {
		starts = append(starts, wd)
	}
	for _, start := range starts {
		for dir := start; ; dir = filepath.Dir(dir) {
			if p := filepath.Join(dir, filepath.FromSlash(repo)); fileExists(filepath.Join(dir, "go.mod")) && fileExists(p) {
				return p
			}
			if filepath.Dir(dir) == dir {
				break
			}
		}
	}
	return ""
}

func fileExists(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && !fi.IsDir()
}

// openDoc opens a shipped document in the editor, or says it is not there.
func (s *Shell) openDoc(name string) {
	p := docPath(name)
	if p == "" {
		s.Notify(notifications.Warning, name+" is not installed", "PyxForge ships its documents beside the program; this copy has none. They are also in the repository's docs folder.")
		return
	}
	s.OpenFile(p)
}

// chooseDoc lists the shipped documents in the palette (Help > Documentation).
func (s *Shell) chooseDoc() {
	var items []commandpalette.Item
	for _, d := range shipdocs.List {
		detail := d.Name
		if docPath(d.Name) == "" {
			detail += " (not installed)"
		}
		items = append(items, commandpalette.Item{Title: d.Title, Detail: detail, Icon: icons.FileText,
			Run: func() { s.openDoc(d.Name) }})
	}
	s.palette.Show("Open a document shipped with PyxForge", items)
}
