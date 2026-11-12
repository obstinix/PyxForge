package kit

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/widget"
	"github.com/obstinix/PyxForge/internal/ui/theme"
)

// PaneOpacity is how opaque a workspace pane (the activity bar, the explorer, the bottom panel,
// the status bar) is with the glass setting on: enough of the Backdrop shows through to layer
// the panes, and text on them keeps the palette's contrast to within a few percent.
const PaneOpacity = 0.64

// PaneFill is a pane's fill for a surface colour: translucent with the glass setting on, the
// colour itself with it off.
func PaneFill(c color.NRGBA) color.NRGBA {
	if theme.Current().GlassOn {
		c.A = uint8(float32(c.A) * PaneOpacity)
	}
	return c
}

// Backdrop is what the workspace's glass panes lie on: the palette's base surface with two
// wide, soft glows of the accent, one from the top-left corner and a fainter one from the
// bottom-right. A window has nothing of the desktop behind it to show through, so the depth
// that glass needs comes from here. With the glass setting off it is the plain base surface.
type Backdrop struct {
	widget.BaseWidget
}

// NewBackdrop returns the workspace backdrop.
func NewBackdrop() *Backdrop {
	b := &Backdrop{}
	b.ExtendBaseWidget(b)
	return b
}

func (b *Backdrop) CreateRenderer() fyne.WidgetRenderer {
	r := &backdropRenderer{base: canvas.NewRectangle(nil),
		warm: canvas.NewRadialGradient(color.Transparent, color.Transparent),
		cool: canvas.NewRadialGradient(color.Transparent, color.Transparent)}
	r.warm.CenterOffsetX, r.warm.CenterOffsetY = -0.5, -0.5 // the top-left corner
	r.cool.CenterOffsetX, r.cool.CenterOffsetY = 0.5, 0.5   // the bottom-right corner
	r.Refresh()
	return r
}

type backdropRenderer struct {
	base       *canvas.Rectangle
	warm, cool *canvas.RadialGradient
}

func (r *backdropRenderer) Refresh() {
	t := theme.Current()
	r.base.FillColor = t.Surface.Base
	glow := func(alpha uint8) color.NRGBA {
		c := t.Accent.Primary
		c.A = alpha
		return c
	}
	switch {
	case t.GlassOn && t.Polarity == theme.Light:
		// A light surface takes a tint far more readily than a dark one: the same glow that
		// warms Smoked Kraft turned Ink & Paper pink.
		r.warm.StartColor, r.cool.StartColor = glow(40), glow(26)
	case t.GlassOn:
		r.warm.StartColor, r.cool.StartColor = glow(84), glow(58)
	default:
		r.warm.StartColor, r.cool.StartColor = color.Transparent, color.Transparent
	}
	r.base.Refresh()
	r.warm.Refresh()
	r.cool.Refresh()
}

func (r *backdropRenderer) Layout(s fyne.Size) {
	r.base.Resize(s)
	r.warm.Resize(s)
	r.cool.Resize(s)
}

func (r *backdropRenderer) MinSize() fyne.Size { return fyne.Size{} }
func (r *backdropRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.base, r.warm, r.cool}
}
func (r *backdropRenderer) Destroy() {}
