package kit

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/widget"
	"github.com/obstinix/PyxForge/internal/ui/theme"
)

// Glass is the overlay material (DESIGN_SYSTEM.md §9): a body, a 1 px rim, a 1 px specular top
// edge and one soft shadow. The body is opaque unless the glass setting is on; then it is a
// translucent tint over a backdrop blur of whatever is behind it. Use it only for things that
// float: palette, dialogs, notifications, workspace switcher, floating inspector, agent overlays.
type Glass struct {
	widget.BaseWidget
	Content fyne.CanvasObject
	Padding float32
}

// NewGlass wraps content in the overlay material.
func NewGlass(content fyne.CanvasObject) *Glass {
	g := &Glass{Content: content, Padding: theme.Space2}
	g.ExtendBaseWidget(g)
	return g
}

func (g *Glass) CreateRenderer() fyne.WidgetRenderer {
	r := &glassRenderer{g: g, blur: canvas.NewBlur(0), body: canvas.NewRectangle(nil),
		edge: canvas.NewRectangle(nil)}
	r.Refresh()
	return r
}

type glassRenderer struct {
	g          *Glass
	blur       *canvas.Blur
	body, edge *canvas.Rectangle
}

func (r *glassRenderer) Refresh() {
	gl := theme.Current().Glass
	SetBackdrop(r.blur, gl.Blur)
	r.body.FillColor = gl.Fill
	r.body.StrokeColor = gl.Rim
	r.body.StrokeWidth = 1
	r.body.CornerRadius = theme.RadiusOverlay
	r.body.Shadow = canvas.Shadow{
		Color:      gl.Shadow,
		BlurRadius: gl.ShadowBlur,
		Offset:     fyne.NewPos(0, gl.ShadowOffsetY),
		Variant:    canvas.DropShadow,
	}
	r.edge.FillColor = gl.Edge
	r.body.Refresh()
	r.edge.Refresh()
	r.g.Content.Refresh()
}

func (r *glassRenderer) Layout(s fyne.Size) {
	r.blur.Resize(s)
	r.body.Resize(s)
	// The specular line runs between the rounded corners, one pixel inside the rim.
	r.edge.Move(fyne.NewPos(theme.RadiusOverlay, 1))
	r.edge.Resize(fyne.NewSize(s.Width-2*theme.RadiusOverlay, 1))
	p := r.g.Padding
	r.g.Content.Move(fyne.NewPos(p, p))
	r.g.Content.Resize(fyne.NewSize(s.Width-2*p, s.Height-2*p))
}

func (r *glassRenderer) MinSize() fyne.Size {
	p := 2 * r.g.Padding
	return r.g.Content.MinSize().Add(fyne.NewSize(p, p))
}

func (r *glassRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.blur, r.body, r.edge, r.g.Content}
}

func (r *glassRenderer) Destroy() {}

// SetBackdrop configures a backdrop blur under an overlay body; radius 0 hides it.
func SetBackdrop(b *canvas.Blur, radius float32) {
	b.Radius, b.CornerRadius = radius, theme.RadiusOverlay
	b.Hidden = radius <= 0
	b.Refresh()
}
