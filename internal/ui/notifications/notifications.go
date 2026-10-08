// Package notifications shows short-lived glass toasts in the bottom-right corner. They live
// in a layer of the main window content, not in a canvas overlay, so the workspace stays
// clickable around them.
package notifications

import (
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"github.com/obstinix/PyxForge/internal/ui/icons"
	"github.com/obstinix/PyxForge/internal/ui/kit"
	"github.com/obstinix/PyxForge/internal/ui/theme"
)

// Level is a notification's severity.
type Level int

const (
	Info Level = iota
	Success
	Warning
	Error
)

const (
	maxVisible = 4
	width      = 360
	lifetime   = 6 * time.Second
	errLife    = 10 * time.Second
)

// Center owns the toast layer.
type Center struct {
	layer *fyne.Container
	// Margin keeps toasts clear of the status bar.
	Margin float32
}

// New returns an empty notification layer.
func New() *Center {
	n := &Center{Margin: theme.StatusBarHeight + theme.Space4}
	n.layer = container.New(&stackLayout{n: n})
	return n
}

// Layer is the object to stack above the workspace.
func (n *Center) Layer() fyne.CanvasObject { return n.layer }

// Count reports how many toasts are showing.
func (n *Center) Count() int { return len(n.layer.Objects) }

// Post shows a toast. It must be called on the UI goroutine (use fyne.Do from elsewhere).
func (n *Center) Post(l Level, title, body string) {
	var toast *kit.Glass
	closeBtn := kit.NewIconButton(icons.X, "Dismiss notification", func() { n.remove(toast) })
	closeBtn.Side = 24
	text := container.NewVBox(kit.NewText(title, kit.Strong, kit.Primary))
	if body != "" {
		text.Add(kit.NewText(body, kit.Body, kit.Secondary))
	}
	icon, role := levelIcon(l)
	toast = kit.NewGlass(container.NewBorder(nil, nil,
		container.NewVBox(kit.NewIcon(icon, role)),
		container.NewVBox(closeBtn), text))
	toast.Padding = theme.Space3

	if len(n.layer.Objects) >= maxVisible {
		n.layer.Objects = n.layer.Objects[1:]
	}
	n.layer.Add(toast)

	life := lifetime
	if l == Error {
		life = errLife
	}
	time.AfterFunc(life, func() { fyne.Do(func() { n.remove(toast) }) })
}

func (n *Center) remove(t fyne.CanvasObject) { n.layer.Remove(t) }

func levelIcon(l Level) (icons.Name, kit.Role) {
	switch l {
	case Success:
		return icons.CircleCheck, kit.Success
	case Warning:
		return icons.TriangleAlert, kit.Warning
	case Error:
		return icons.CircleX, kit.Error
	}
	return icons.Info, kit.Secondary
}

// stackLayout stacks toasts upward from the bottom-right corner, newest at the bottom.
type stackLayout struct{ n *Center }

func (l *stackLayout) Layout(objs []fyne.CanvasObject, s fyne.Size) {
	w := min(float32(width), s.Width-2*theme.Space4)
	y := s.Height - l.n.Margin
	for i := len(objs) - 1; i >= 0; i-- {
		h := objs[i].MinSize().Height
		y -= h
		objs[i].Resize(fyne.NewSize(w, h))
		objs[i].Move(fyne.NewPos(s.Width-w-theme.Space4, y))
		y -= theme.Space2
	}
}

func (l *stackLayout) MinSize([]fyne.CanvasObject) fyne.Size { return fyne.Size{} }
