package kit

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/widget"
	"github.com/obstinix/PyxForge/internal/ui/theme"
)

// FocusFrame draws the 2 px Accent.Focus ring around a region whose own widget cannot draw one:
// Fyne's tree and list mark their keyboard-highlighted row with the hover wash only. The ring
// sits inside the frame's bounds, so it never overlaps a neighbouring panel, and it is drawn
// above the content without taking input.
type FocusFrame struct {
	widget.BaseWidget
	Content fyne.CanvasObject
	focused bool
}

// NewFocusFrame wraps content.
func NewFocusFrame(content fyne.CanvasObject) *FocusFrame {
	f := &FocusFrame{Content: content}
	f.ExtendBaseWidget(f)
	return f
}

// SetFocused shows or hides the ring.
func (f *FocusFrame) SetFocused(on bool) {
	if f.focused == on {
		return
	}
	f.focused = on
	f.Refresh()
}

// Focused reports whether the ring is showing.
func (f *FocusFrame) Focused() bool { return f.focused }

func (f *FocusFrame) CreateRenderer() fyne.WidgetRenderer {
	r := &focusFrameRenderer{f: f, ring: canvas.NewRectangle(color.Transparent)}
	r.Refresh()
	return r
}

type focusFrameRenderer struct {
	f    *FocusFrame
	ring *canvas.Rectangle
}

func (r *focusFrameRenderer) Refresh() {
	styleRing(r.ring, theme.Current())
	r.ring.Hidden = !r.f.focused
	r.ring.Refresh()
	r.f.Content.Refresh()
}

func (r *focusFrameRenderer) Layout(s fyne.Size) {
	r.f.Content.Resize(s)
	r.ring.Move(fyne.NewPos(1, 1))
	r.ring.Resize(fyne.NewSize(s.Width-2, s.Height-2))
}

func (r *focusFrameRenderer) MinSize() fyne.Size { return r.f.Content.MinSize() }
func (r *focusFrameRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.f.Content, r.ring}
}
func (r *focusFrameRenderer) Destroy() {}

// styleRing makes a rectangle the focus ring: a 2 px Accent.Focus stroke at the control radius
// (DESIGN_SYSTEM.md §12). Every PyxForge focus indicator is drawn with it.
func styleRing(ring *canvas.Rectangle, t theme.Tokens) {
	ring.FillColor = color.Transparent
	ring.StrokeColor = t.Accent.Focus
	ring.StrokeWidth = theme.FocusRingWidth
	ring.CornerRadius = theme.RadiusControl
}

// NewFocusRing returns a rectangle styled as the focus ring, hidden until shown. Widgets
// outside kit (cards, status items) call StyleFocusRing from their Refresh.
func NewFocusRing() *canvas.Rectangle {
	r := canvas.NewRectangle(color.Transparent)
	r.Hidden = true
	return r
}

// StyleFocusRing restyles a ring from NewFocusRing for the current theme and shows or hides it.
func StyleFocusRing(ring *canvas.Rectangle, focused bool) {
	styleRing(ring, theme.Current())
	ring.Hidden = !focused
	ring.Refresh()
}
