// Package notifications shows glass toasts in the bottom-right corner. They live in a layer of
// the main window content, not in a canvas overlay, so the workspace stays clickable around
// them. Information goes after a few seconds; an error stays until it is dismissed, so it can
// be read whole (PXF-DEF-006, PXF-DEF-021).
package notifications

import (
	"strings"
	"time"
	"unicode/utf8"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	fynetheme "fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
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
	// maxBody is how much of a body a toast shows; the shell writes the whole of every
	// notification to its Log panel.
	maxBody = 480
)

// Lifetime is how long a toast of a level stays up; 0 means until it is dismissed.
func Lifetime(l Level) time.Duration {
	switch l {
	case Error:
		return 0
	case Warning:
		return 10 * time.Second
	}
	return 6 * time.Second
}

// shorten cuts a long body at a word boundary and says where the rest is.
func shorten(body string) string {
	if len(body) <= maxBody {
		return body
	}
	cut := strings.LastIndexAny(body[:maxBody], " \n")
	if cut < maxBody/2 {
		cut = maxBody
		for cut > 0 && !utf8.RuneStart(body[cut]) { // never split a character
			cut--
		}
	}
	return strings.TrimSpace(body[:cut]) + "… (the full message is in the Log panel)"
}

// Center owns the toast layer.
type Center struct {
	layer *fyne.Container
	// Margin keeps toasts clear of the status bar.
	Margin float32
	// Dispatch runs a toast's expiry on the UI thread; nil means fyne.Do. Tests pass their own,
	// because Fyne's test driver runs fyne.Do on the timer's goroutine.
	Dispatch func(func())
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
		// Wrapped to the toast's width, not cut off at one line.
		b := widget.NewRichText(&widget.TextSegment{Text: shorten(body),
			Style: widget.RichTextStyle{ColorName: fynetheme.ColorNamePlaceHolder}})
		b.Wrapping = fyne.TextWrapWord
		text.Add(b)
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

	life := Lifetime(l)
	if life == 0 {
		return // an error stays until dismissed
	}
	do := n.Dispatch
	if do == nil {
		do = fyne.Do
	}
	time.AfterFunc(life, func() { do(func() { n.remove(toast) }) })
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
		objs[i].Resize(fyne.NewSize(w, objs[i].MinSize().Height)) // let wrapped text settle at w
		h := objs[i].MinSize().Height
		y -= h
		objs[i].Resize(fyne.NewSize(w, h))
		objs[i].Move(fyne.NewPos(s.Width-w-theme.Space4, y))
		y -= theme.Space2
	}
}

func (l *stackLayout) MinSize([]fyne.CanvasObject) fyne.Size { return fyne.Size{} }
