package shell

import (
	"fmt"
	"os"
	"path/filepath"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"github.com/obstinix/PyxForge/internal/textfile"
	"github.com/obstinix/PyxForge/internal/ui/hexview"
	"github.com/obstinix/PyxForge/internal/ui/icons"
	"github.com/obstinix/PyxForge/internal/ui/kit"
	"github.com/obstinix/PyxForge/internal/ui/notifications"
)

// The large-file limits (owner review F4). Above readOnlyBytes a text file opens read-only;
// above hexOnlyBytes any file opens in the paged hex view only. Large-file mode, with no
// colouring or language server, starts at codeedit.LargeFileBytes.
// Settings > Files & Explorer changes them (applyFilePrefs).
var (
	readOnlyBytes int64 = 50 << 20
	hexOnlyBytes  int64 = 512 << 20
)

// openBinary opens a binary file (textfile.IsBinary), or any file over hexOnlyBytes, in a tab
// of its own instead of the editor, so that no text-editing command can rewrite its bytes. The
// tab shows what the file is and all of it as a paged hex dump, and below readOnlyBytes offers
// to open it in the editor anyway, read-only.
func (s *Shell) openBinary(p string, st os.FileInfo, info textfile.Info, focus bool) {
	var t *container.TabItem
	t = container.NewTabItem(filepath.Base(p), s.binaryView(p, st, info, func() {
		s.removeTab(t)
		s.openAsText(p)
	}))
	s.open[p] = t
	s.addRecent(prefRecentFiles, p)
	s.selectQuietly = true
	s.editors.Append(t)
	s.selectQuietly = false
	s.syncEditor()
	s.selectTab(t, focus)
}

// openAsText opens a file in the editor whatever its content.
func (s *Shell) openAsText(p string) {
	if !s.startNative() {
		s.Notify(notifications.Error, "Cannot open as text", "The editor is not available in this window.")
		return
	}
	if t := s.open[p]; t != nil { // the binary file's own tab gives way to the text
		s.removeTab(t)
	}
	s.nat.openBinary(p)
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
	hex, err := hexview.Open(p)
	if err != nil {
		return placeholder(icons.FileCode, "Cannot read file", err.Error())
	}
	title, detail := "Binary file", "It is not opened as text, so saving cannot change its bytes. Opened as text anyway, "+
		"it is read-only and written back byte for byte."
	var actions fyne.CanvasObject = container.NewHBox()
	if st.Size() > hexOnlyBytes {
		title, detail = "Large file", fmt.Sprintf("Over %s, files open as hex only: the view reads just what is on screen.",
			humanSize(hexOnlyBytes))
	} else if st.Size() <= readOnlyBytes {
		actions = kit.NewButton("Open as Text (Read-Only)", asText)
	}
	head := container.NewBorder(nil, nil, nil, container.NewCenter(actions), container.NewVBox(facts,
		kit.NewText(title+" · "+detail, kit.Body, kit.Tertiary)))
	return container.NewBorder(container.NewPadded(head), nil, nil, nil, hex)
}
