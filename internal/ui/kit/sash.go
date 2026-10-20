package kit

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
	"github.com/obstinix/PyxForge/internal/ui/theme"
)

// SashGrip is how wide a sash is to the pointer: it draws a 1 px rule in its middle, so the
// rule can be dragged without aiming at a single pixel.
const SashGrip = 7

// Sash is the rule between two panels, dragged to resize them (DESIGN_SYSTEM.md §8: the
// explorer and the bottom dock are resizable). A vertical sash moves sideways.
type Sash struct {
	widget.BaseWidget
	Vertical bool
	// OnDrag receives each move, in logical pixels: rightward or downward is positive.
	OnDrag func(delta float32)
	// OnReset runs on a double click, to restore the default size.
	OnReset func()

	hovered, dragging bool
}

var (
	_ fyne.Draggable      = (*Sash)(nil)
	_ fyne.DoubleTappable = (*Sash)(nil)
	_ desktop.Hoverable   = (*Sash)(nil)
	_ desktop.Cursorable  = (*Sash)(nil)
)

// NewSash returns a sash; vertical for one between panels side by side.
func NewSash(vertical bool, onDrag func(float32)) *Sash {
	s := &Sash{Vertical: vertical, OnDrag: onDrag}
	s.ExtendBaseWidget(s)
	return s
}

func (s *Sash) Dragged(e *fyne.DragEvent) {
	s.dragging = true
	d := e.Dragged.DY
	if s.Vertical {
		d = e.Dragged.DX
	}
	if s.OnDrag != nil && d != 0 {
		s.OnDrag(d)
	}
	s.Refresh()
}

func (s *Sash) DragEnd() { s.dragging = false; s.Refresh() }

func (s *Sash) DoubleTapped(*fyne.PointEvent) {
	if s.OnReset != nil {
		s.OnReset()
	}
}

func (s *Sash) MouseIn(*desktop.MouseEvent)    { s.hovered = true; s.Refresh() }
func (s *Sash) MouseMoved(*desktop.MouseEvent) {}
func (s *Sash) MouseOut()                      { s.hovered = false; s.Refresh() }

func (s *Sash) Cursor() desktop.Cursor {
	if s.Vertical {
		return desktop.HResizeCursor
	}
	return desktop.VResizeCursor
}

func (s *Sash) CreateRenderer() fyne.WidgetRenderer {
	r := &sashRenderer{s: s, line: canvas.NewRectangle(nil)}
	r.Refresh()
	return r
}

type sashRenderer struct {
	s    *Sash
	line *canvas.Rectangle
}

func (r *sashRenderer) Refresh() {
	t := theme.Current()
	r.line.FillColor = t.Border.Hairline
	if r.s.hovered || r.s.dragging {
		r.line.FillColor = t.Border.Strong
	}
	r.Layout(r.s.Size()) // the rule thickens while in use
	r.line.Refresh()
}

// Layout draws the rule in the middle of the grip, 1 px thick, 2 px while it is in use.
func (r *sashRenderer) Layout(sz fyne.Size) {
	thick := float32(1)
	if r.s.hovered || r.s.dragging {
		thick = 2
	}
	if r.s.Vertical {
		r.line.Move(fyne.NewPos((sz.Width-thick)/2, 0))
		r.line.Resize(fyne.NewSize(thick, sz.Height))
		return
	}
	r.line.Move(fyne.NewPos(0, (sz.Height-thick)/2))
	r.line.Resize(fyne.NewSize(sz.Width, thick))
}

func (r *sashRenderer) MinSize() fyne.Size           { return fyne.NewSize(1, 1) }
func (r *sashRenderer) Objects() []fyne.CanvasObject { return []fyne.CanvasObject{r.line} }
func (r *sashRenderer) Destroy()                     {}
