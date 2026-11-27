package shell

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"github.com/obstinix/PyxForge/internal/ui/notifications"
	"github.com/obstinix/PyxForge/internal/ui/theme"
	"github.com/obstinix/PyxForge/internal/workspace"
)

// Dragging in the Explorer (owner review, Phase 7.2): a row dragged onto a folder moves there,
// onto the editor opens, onto the terminal types its path. A label under the pointer says which
// before the button is let go, and Ctrl+Z in the Explorer takes the last move back.

// dropKind is what letting go of a dragged row would do.
type dropKind int

const (
	dropNone dropKind = iota
	dropMove
	dropOpen
	dropInsert
)

// dropAt works out what a drop of src at the absolute position at would do, and into which
// folder for a move.
func (s *Shell) dropAt(src string, dir bool, at fyne.Position) (dropKind, string) {
	if folder, ok := s.explorer.FolderAt(at); ok {
		if canMove(src, folder) {
			return dropMove, folder
		}
		return dropNone, ""
	}
	if !dir && s.editorStack != nil && within(s.editorStack, at) {
		return dropOpen, ""
	}
	if s.term != nil && s.term.current != nil && s.term.body != nil && within(s.term.body, at) {
		return dropInsert, ""
	}
	return dropNone, ""
}

// canMove reports whether src may move into folder: not where it is, not into itself.
func canMove(src, folder string) bool {
	src, folder = filepath.Clean(src), filepath.Clean(folder)
	return filepath.Dir(src) != folder && src != folder && !workspace.Within(src, folder)
}

// within reports whether the absolute position at is on o, which is showing.
func within(o fyne.CanvasObject, at fyne.Position) bool {
	if !o.Visible() {
		return false
	}
	p := fyne.CurrentApp().Driver().AbsolutePositionForObject(o)
	sz := o.Size()
	return at.X >= p.X && at.Y >= p.Y && at.X < p.X+sz.Width && at.Y < p.Y+sz.Height
}

// dragRow follows a dragged row: the label under the pointer and the target folder's outline.
func (s *Shell) dragRow(src string, dir bool, at fyne.Position) {
	kind, folder := s.dropAt(src, dir, at)
	label := filepath.Base(src)
	switch kind {
	case dropMove:
		label = "Move " + label + " to " + s.folderLabel(folder)
		s.explorer.SetDropTarget(folder)
	case dropOpen:
		label = "Open " + label
		s.explorer.SetDropTarget("\x00")
	case dropInsert:
		label = "Type the path of " + label + " in the terminal"
		s.explorer.SetDropTarget("\x00")
	default:
		s.explorer.SetDropTarget("\x00")
	}
	s.showGhost(label, at)
}

// dropRow acts on a dragged row let go at the absolute position at.
func (s *Shell) dropRow(src string, dir bool, at fyne.Position) {
	s.hideGhost()
	s.explorer.SetDropTarget("\x00")
	kind, folder := s.dropAt(src, dir, at)
	switch kind {
	case dropMove:
		s.moveEntry(src, folder)
	case dropOpen:
		s.OpenFile(src)
	case dropInsert:
		s.term.current.write([]byte(shellQuote(src)))
		s.term.focus()
	}
}

func (s *Shell) folderLabel(folder string) string {
	if sameFile(folder, s.root) {
		return "the workspace's top"
	}
	return filepath.Base(folder)
}

// shellQuote quotes a path for the shell the terminal runs: PowerShell and cmd take double
// quotes, Unix shells single ones.
func shellQuote(p string) string {
	if !strings.ContainsAny(p, " \t'\"$`&;()|<>*?[]{}!#~") {
		return p
	}
	if runtime.GOOS == "windows" {
		return `"` + p + `"`
	}
	return "'" + strings.ReplaceAll(p, "'", `'\''`) + "'"
}

// lastMove is the last move by drag, for Undo.
type lastMove struct{ from, to string }

// moveEntry moves src into folder, closing its tabs first and opening them again where it went.
func (s *Shell) moveEntry(src, folder string) {
	s.releaseFiles(src, "move it", func(closed []string) {
		to, err := s.explorer.Move(src, folder)
		if err != nil {
			s.Notify(notifications.Error, "Cannot move "+filepath.Base(src), err.Error())
			to = src
		}
		for _, f := range closed {
			s.openFile(movedTo(f, src, to), false)
		}
		if err != nil {
			return
		}
		s.undoMove = &lastMove{from: src, to: to}
		s.explorer.Reveal(to)
		s.Notify(notifications.Success, fmt.Sprintf("Moved %s to %s", filepath.Base(src), s.folderLabel(folder)),
			"Ctrl+Z in the Explorer moves it back.")
	})
}

// undoLastMove moves the last dragged row back where it was.
func (s *Shell) undoLastMove() {
	m := s.undoMove
	if m == nil {
		s.Notify(notifications.Info, "Nothing to undo", "Only a move by dragging in the Explorer can be undone here.")
		return
	}
	s.undoMove = nil
	s.releaseFiles(m.to, "move it back", func(closed []string) {
		back, err := s.explorer.Move(m.to, filepath.Dir(m.from))
		if err != nil {
			s.Notify(notifications.Error, "Cannot move "+filepath.Base(m.to)+" back", err.Error())
			back = m.to
		}
		for _, f := range closed {
			s.openFile(movedTo(f, m.to, back), false)
		}
		if err == nil {
			s.explorer.Reveal(back)
		}
	})
}

// showGhost draws the label of a dragged row just below and right of the pointer.
func (s *Shell) showGhost(label string, at fyne.Position) {
	t := theme.Current()
	if s.ghost == nil {
		bg := canvas.NewRectangle(t.Surface.Overlay)
		bg.CornerRadius = theme.RadiusControl
		bg.StrokeWidth = 1
		text := canvas.NewText("", t.Text.Primary)
		text.TextSize = theme.TextSmall
		s.ghost = container.NewWithoutLayout(bg, text)
		s.dragLayer.Add(s.ghost)
	}
	bg := s.ghost.Objects[0].(*canvas.Rectangle)
	text := s.ghost.Objects[1].(*canvas.Text)
	bg.FillColor, bg.StrokeColor = t.Surface.Overlay, t.Accent.Primary
	text.Color, text.Text = t.Text.Primary, label
	sz := text.MinSize()
	pad := theme.Space2
	bg.Resize(fyne.NewSize(sz.Width+2*pad, sz.Height+pad))
	text.Move(fyne.NewPos(pad, pad/2))
	origin := fyne.CurrentApp().Driver().AbsolutePositionForObject(s.dragLayer)
	s.ghost.Move(at.Subtract(origin).Add(fyne.NewPos(14, 14)))
	s.ghost.Show()
	bg.Refresh()
	text.Refresh()
	s.dragLayer.Refresh()
}

func (s *Shell) hideGhost() {
	if s.ghost != nil {
		s.ghost.Hide()
		s.dragLayer.Refresh()
	}
}
