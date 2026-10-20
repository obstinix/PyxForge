package shell

import (
	"fmt"
	"os"
	"path/filepath"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"github.com/obstinix/PyxForge/internal/inspect"
	"github.com/obstinix/PyxForge/internal/textfile"
	"github.com/obstinix/PyxForge/internal/ui/icons"
	"github.com/obstinix/PyxForge/internal/ui/kit"
	"github.com/obstinix/PyxForge/internal/ui/notifications"
	"github.com/obstinix/PyxForge/internal/ui/theme"
)

// binaryPreview is how many of a binary file's first bytes its tab shows.
const binaryPreview = 256

// openBinary opens a binary file (textfile.IsBinary) in a tab of its own instead of the
// editor, so that no text-editing command can rewrite its bytes. The tab shows what the file
// is, its first bytes as a hex dump, and offers to open it in the editor anyway, where it is
// read in binary mode and read-only (nvim/lua/pyxforge/files.lua).
func (s *Shell) openBinary(p string, st os.FileInfo, info textfile.Info, focus bool) {
	var t *container.TabItem
	t = container.NewTabItem(filepath.Base(p), s.binaryView(p, st, info, func() {
		s.editors.Remove(t)
		s.editors.OnClosed(t)
		s.openAsText(p)
	}))
	s.open[p] = t
	s.selectQuietly = true
	s.editors.Append(t)
	s.selectQuietly = false
	s.syncEditor()
	s.selectTab(t, focus)
}

// openAsText opens a file in the editor whatever its content.
func (s *Shell) openAsText(p string) {
	if !s.startEditor() {
		s.Notify(notifications.Error, "Cannot open as text", "The editor is not available in this window.")
		return
	}
	s.intent = &openIntent{path: p, focus: true}
	s.ed.open(p)
}

func (s *Shell) binaryView(p string, st os.FileInfo, info textfile.Info, asText func()) fyne.CanvasObject {
	rel, err := filepath.Rel(s.root, p)
	if err != nil {
		rel = p
	}
	facts := container.NewVBox(
		kit.NewText(filepath.ToSlash(rel), kit.Mono, kit.Secondary),
		kit.NewText(fmt.Sprintf("%s · modified %s", humanSize(st.Size()), st.ModTime().Format("2006-01-02 15:04")),
			kit.Body, kit.Tertiary),
	)
	head := info.Head[:min(len(info.Head), binaryPreview)]
	dump := container.NewVBox()
	for _, l := range inspect.Dump(head, 0) {
		t := kit.NewText(l.String(), kit.Mono, kit.Secondary)
		t.TextSize = theme.TextCaption + 1
		dump.Add(t)
	}
	if len(head) < len(info.Head) || int64(len(info.Head)) < st.Size() {
		dump.Add(kit.NewText(fmt.Sprintf("First %d of %d bytes.", len(head), st.Size()), kit.Body, kit.Tertiary))
	}
	open := widget.NewButton("Open as Text (Read-Only)", asText)
	note := placeholder(icons.FileCode, "Binary file",
		"It is not opened as text, so saving cannot change its bytes. Opened as text anyway, it is read-only and "+
			"written back byte for byte.")
	note.(*fyne.Container).Objects[0].(*fyne.Container).Add(container.NewCenter(open))
	return container.NewBorder(container.NewPadded(facts), nil, nil, nil,
		container.NewVScroll(container.NewVBox(note, container.NewPadded(container.NewCenter(dump)))))
}
