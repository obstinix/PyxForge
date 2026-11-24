package kit

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/widget"
	"github.com/obstinix/PyxForge/internal/ui/theme"
)

// Float is the material of things that float over the workspace (palette, menus, dialogs,
// notifications, tooltips, the floating inspector): the opaque Overlay surface, a 1 px rim
// and one soft shadow. Nothing behind it shows through or is blurred (decision D18).
type Float struct {
	widget.BaseWidget
	Content fyne.CanvasObject
	Padding float32
}

// NewFloat wraps content in the floating material.
func NewFloat(content fyne.CanvasObject) *Float {
	g := &Float{Content: content, Padding: theme.Space2}
	g.ExtendBaseWidget(g)
	return g
}

// CreateRenderer implements fyne.Widget.
func (g *Float) CreateRenderer() fyne.WidgetRenderer {
	r := &floatRenderer{g: g, body: canvas.NewRectangle(nil)}
	r.Refresh()
	return r
}

type floatRenderer struct {
	g    *Float
	body *canvas.Rectangle
}

// StyleFloat gives a rectangle the floating material's body, rim and shadow.
func StyleFloat(body *canvas.Rectangle) {
	t := theme.Current()
	body.FillColor = t.Surface.Overlay
	body.StrokeColor = t.Float.Rim
	body.StrokeWidth = 1
	body.CornerRadius = theme.RadiusOverlay
	body.Shadow = canvas.Shadow{
		Color:      t.Float.Shadow,
		BlurRadius: t.Float.ShadowBlur,
		Offset:     fyne.NewPos(0, t.Float.ShadowOffsetY),
		Variant:    canvas.DropShadow,
	}
}

func (r *floatRenderer) Refresh() {
	StyleFloat(r.body)
	r.body.Refresh()
	r.g.Content.Refresh()
}

func (r *floatRenderer) Layout(s fyne.Size) {
	r.body.Resize(s)
	p := r.g.Padding
	r.g.Content.Move(fyne.NewPos(p, p))
	r.g.Content.Resize(fyne.NewSize(s.Width-2*p, s.Height-2*p))
}

func (r *floatRenderer) MinSize() fyne.Size {
	p := 2 * r.g.Padding
	return r.g.Content.MinSize().Add(fyne.NewSize(p, p))
}

func (r *floatRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.body, r.g.Content}
}

func (r *floatRenderer) Destroy() {}
