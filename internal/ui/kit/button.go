package kit

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
	"github.com/obstinix/PyxForge/internal/ui/icons"
	"github.com/obstinix/PyxForge/internal/ui/theme"
)

// Marker says where a selected IconButton draws its 2 px accent bar.
type Marker int

const (
	NoMarker Marker = iota
	LeadingMarker
)

// IconButton is a square, focusable icon control (rail items, panel actions). Label is its
// accessible name; the command palette lists the same action by that name.
type IconButton struct {
	widget.BaseWidget
	Icon     icons.Name
	Label    string
	Selected bool
	Marker   Marker
	Side     float32 // edge length; 0 means 32
	IconSize float32 // 0 means theme.IconSize
	OnTapped func()

	hovered, pressed, focused bool
}

// NewIconButton returns an icon button.
func NewIconButton(n icons.Name, label string, tapped func()) *IconButton {
	b := &IconButton{Icon: n, Label: label, OnTapped: tapped}
	b.ExtendBaseWidget(b)
	return b
}

// SetSelected marks the button as the current item.
func (b *IconButton) SetSelected(on bool) {
	b.Selected = on
	b.Refresh()
}

func (b *IconButton) Tapped(*fyne.PointEvent) {
	if b.OnTapped != nil {
		b.OnTapped()
	}
}

func (b *IconButton) MouseIn(*desktop.MouseEvent)    { b.hovered = true; b.Refresh() }
func (b *IconButton) MouseMoved(*desktop.MouseEvent) {}
func (b *IconButton) MouseOut()                      { b.hovered, b.pressed = false, false; b.Refresh() }
func (b *IconButton) MouseDown(*desktop.MouseEvent)  { b.pressed = true; b.Refresh() }
func (b *IconButton) MouseUp(*desktop.MouseEvent)    { b.pressed = false; b.Refresh() }

func (b *IconButton) FocusGained() { b.focused = true; b.Refresh() }
func (b *IconButton) FocusLost()   { b.focused = false; b.Refresh() }
func (b *IconButton) TypedRune(r rune) {
	if r == ' ' {
		b.Tapped(nil)
	}
}
func (b *IconButton) TypedKey(e *fyne.KeyEvent) {
	if e.Name == fyne.KeyReturn || e.Name == fyne.KeyEnter {
		b.Tapped(nil)
	}
}

func (b *IconButton) CreateRenderer() fyne.WidgetRenderer {
	r := &iconButtonRenderer{
		b:      b,
		wash:   canvas.NewRectangle(nil),
		marker: canvas.NewRectangle(nil),
		ring:   canvas.NewRectangle(nil),
		icon:   NewIcon(b.Icon, Secondary),
	}
	r.Refresh()
	return r
}

type iconButtonRenderer struct {
	b                  *IconButton
	wash, marker, ring *canvas.Rectangle
	icon               *Icon
}

func (r *iconButtonRenderer) side() float32 {
	if r.b.Side > 0 {
		return r.b.Side
	}
	return 32
}

func (r *iconButtonRenderer) Refresh() {
	t := theme.Current()
	b := r.b
	r.wash.CornerRadius = theme.RadiusControl
	switch {
	case b.pressed:
		r.wash.FillColor = t.State.Pressed
	case b.hovered:
		r.wash.FillColor = t.State.Hover
	case b.Selected:
		r.wash.FillColor = t.State.Selected
	default:
		r.wash.FillColor = nil
	}
	r.ring.FillColor = nil
	r.ring.StrokeWidth = theme.FocusRingWidth
	r.ring.CornerRadius = theme.RadiusControl
	r.ring.StrokeColor = t.Accent.Focus
	r.ring.Hidden = !b.focused
	r.marker.FillColor = t.Accent.Primary
	r.marker.Hidden = !(b.Selected && b.Marker == LeadingMarker)

	r.icon.Name, r.icon.IconSize = b.Icon, b.IconSize
	r.icon.Role = Secondary
	if b.Selected || b.hovered {
		r.icon.Role = Primary
	}
	r.wash.Refresh()
	r.ring.Refresh()
	r.marker.Refresh()
	r.icon.Refresh()
}

func (r *iconButtonRenderer) Layout(s fyne.Size) {
	d := r.side()
	pos := fyne.NewPos((s.Width-d)/2, (s.Height-d)/2)
	for _, o := range []fyne.CanvasObject{r.wash, r.ring, r.icon} {
		o.Resize(fyne.NewSquareSize(d))
		o.Move(pos)
	}
	// The selection bar sits on the container's leading edge, like the active-row marker.
	r.marker.Resize(fyne.NewSize(2, d-theme.Space2))
	r.marker.Move(fyne.NewPos(0, pos.Y+theme.Space1))
}

func (r *iconButtonRenderer) MinSize() fyne.Size { return fyne.NewSquareSize(r.side()) }

func (r *iconButtonRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.wash, r.icon, r.marker, r.ring}
}

func (r *iconButtonRenderer) Destroy() {}
