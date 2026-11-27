package kit

import (
	"image"
	"image/color"
	"math"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/widget"
	"github.com/obstinix/PyxForge/internal/ui/theme"
)

// Float is the material of things that float over the workspace (palette, menus, dialogs,
// notifications, tooltips, the floating inspector). Flat, the default, it is the opaque
// Overlay surface, a 1 px rim and one soft shadow (D18). With the Liquid Glass material (D21)
// and Glass set, it is a blurred backdrop under the theme's GlassTint, with a specular rim,
// a refraction band at the edge (after Refract) and the same shadow. Text-dense content on
// glass goes on a Plate.
type Float struct {
	widget.BaseWidget
	Content fyne.CanvasObject
	Padding float32
	// Glass makes this surface Liquid Glass when that material is chosen. Surfaces D21 does
	// not list (menus, the floating inspector) leave it false and stay flat.
	Glass bool

	open     float32     // the opening animation's progress, 0…1 (1 when settled)
	backdrop image.Image // the canvas captured by Refract, in device pixels
	scale    float32
}

// NewFloat wraps content in the floating material.
func NewFloat(content fyne.CanvasObject) *Float {
	g := &Float{Content: content, Padding: theme.Space2, open: 1}
	g.ExtendBaseWidget(g)
	return g
}

// NewGlassFloat wraps content in a floating surface that becomes Liquid Glass when that
// material is chosen.
func NewGlassFloat(content fyne.CanvasObject) *Float {
	g := NewFloat(content)
	g.Glass = true
	return g
}

// IsGlass reports whether the surface is drawn as glass now.
func (g *Float) IsGlass() bool { return g.Glass && theme.GlassOn() }

// GlassOpenDuration is how long a glass surface takes to open: it grows from 98 % and its
// glass fades in (none under Reduce Motion).
const GlassOpenDuration = 140 * time.Millisecond

// Opened plays the opening motion; call it when the surface is shown. Flat surfaces, and
// every surface under Reduce Motion, appear at once.
func (g *Float) Opened() {
	if !g.IsGlass() || (ReduceMotion != nil && ReduceMotion()) {
		g.open = 1
		return
	}
	g.open = 0
	a := fyne.NewAnimation(GlassOpenDuration, func(p float32) {
		g.open = p
		g.Refresh()
	})
	a.Curve = fyne.AnimationEaseOut
	a.Start()
}

// Refract captures what is under the surface now, before it is shown, for the refraction band
// along its rim: content there is magnified by 4 %, as a lens bends it. The band keeps that
// picture until the surface is next opened, and drops it if the window is resized; the blur
// under the tint is live.
func (g *Float) Refract(c fyne.Canvas) {
	if !g.IsGlass() || c == nil {
		g.backdrop = nil
		return
	}
	g.backdrop, g.scale = c.Capture(), c.Scale()
	g.Refresh()
}

// CreateRenderer implements fyne.Widget.
func (g *Float) CreateRenderer() fyne.WidgetRenderer {
	r := &floatRenderer{g: g, body: canvas.NewRectangle(nil), blur: canvas.NewBlur(theme.BlurNormal),
		tint: canvas.NewRectangle(nil)}
	r.rim = canvas.NewRaster(r.drawRim)
	r.refract = canvas.NewRaster(r.drawRefraction)
	r.Refresh()
	return r
}

type floatRenderer struct {
	g             *Float
	body          *canvas.Rectangle // flat
	blur          *canvas.Blur      // glass: the live blurred backdrop
	tint          *canvas.Rectangle // glass: GlassTint and the shadow
	rim, refract  *canvas.Raster
	rimImg        *image.NRGBA
	rimFor        [3]int // width, height, polarity the rim was drawn for
	refractImg    *image.NRGBA
	refractFor    [2]int
	refractSource image.Image
}

// StyleFloat gives a rectangle the floating material's flat body, rim and shadow.
func StyleFloat(body *canvas.Rectangle) {
	t := theme.Current()
	body.FillColor = t.Surface.Overlay
	body.StrokeColor = t.Float.Rim
	body.StrokeWidth = 1
	body.CornerRadius = theme.RadiusOverlay
	body.Shadow = floatShadow(t)
}

func floatShadow(t theme.Tokens) canvas.Shadow {
	return canvas.Shadow{
		Color:      t.Float.Shadow,
		BlurRadius: t.Float.ShadowBlur,
		Offset:     fyne.NewPos(0, t.Float.ShadowOffsetY),
		Variant:    canvas.DropShadow, // outside the surface only, so none shows through glass
	}
}

func (r *floatRenderer) Refresh() {
	if r.g.IsGlass() {
		t := theme.Current()
		r.blur.Radius = theme.GlassRadius()
		r.blur.CornerRadius = theme.RadiusOverlay
		tint := t.Float.GlassTint
		tint.A = uint8(float32(tint.A) * r.g.open)
		r.tint.FillColor = tint
		r.tint.CornerRadius = theme.RadiusOverlay
		sh := floatShadow(t)
		c := t.Float.Shadow
		c.A = uint8(float32(c.A) * r.g.open)
		sh.Color = c
		r.tint.Shadow = sh
		r.blur.Refresh()
		r.tint.Refresh()
		r.rim.Refresh()
		r.refract.Refresh()
	} else {
		StyleFloat(r.body)
		r.body.Refresh()
	}
	r.Layout(r.g.Size())
	r.g.Content.Refresh()
}

func (r *floatRenderer) Layout(s fyne.Size) {
	// While opening, the surface grows from 98 % about its centre.
	k := 0.98 + 0.02*r.g.open
	if !r.g.IsGlass() {
		k = 1
	}
	in := fyne.NewSize(s.Width*k, s.Height*k)
	off := fyne.NewPos((s.Width-in.Width)/2, (s.Height-in.Height)/2)
	for _, o := range []fyne.CanvasObject{r.body, r.blur, r.tint, r.rim, r.refract} {
		o.Move(off)
		o.Resize(in)
	}
	p := r.g.Padding
	r.g.Content.Move(off.AddXY(p, p))
	r.g.Content.Resize(fyne.NewSize(in.Width-2*p, in.Height-2*p))
}

func (r *floatRenderer) MinSize() fyne.Size {
	p := 2 * r.g.Padding
	return r.g.Content.MinSize().Add(fyne.NewSize(p, p))
}

func (r *floatRenderer) Objects() []fyne.CanvasObject {
	if !r.g.IsGlass() {
		return []fyne.CanvasObject{r.body, r.g.Content}
	}
	objs := []fyne.CanvasObject{}
	if r.g.backdrop != nil {
		objs = append(objs, r.refract) // under the blur, which softens it with the rest
	}
	return append(objs, r.blur, r.tint, r.rim, r.g.Content)
}

func (r *floatRenderer) Destroy() {}

// roundedDist is the signed distance from (x, y) to the edge of a w×h rectangle with corner
// radius rad: negative inside.
func roundedDist(x, y, w, h, rad float64) float64 {
	qx := math.Abs(x-w/2) - (w/2 - rad)
	qy := math.Abs(y-h/2) - (h/2 - rad)
	out := math.Hypot(math.Max(qx, 0), math.Max(qy, 0))
	return out + math.Min(math.Max(qx, qy), 0) - rad
}

// drawRim draws the glass's edge: a 1 px dark hairline outermost, for separation, and inside
// it a 1 px specular line, white at 35 % along the top and left fading to 8 % at the bottom
// and right. It is the one gradient D21 allows.
func (r *floatRenderer) drawRim(w, h int) image.Image {
	pol := int(theme.Current().Polarity)
	if r.rimImg != nil && r.rimFor == [3]int{w, h, pol} {
		return r.rimImg
	}
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	sc := float64(w) / math.Max(float64(r.rim.Size().Width), 1)
	if sc <= 0 || math.IsInf(sc, 0) {
		sc = 1
	}
	rad := float64(theme.RadiusOverlay) * sc
	hairA := 0.45
	if pol == int(theme.Light) {
		hairA = 0.16
	}
	fw, fh := float64(w), float64(h)
	for y := range h {
		for x := range w {
			d := roundedDist(float64(x)+0.5, float64(y)+0.5, fw, fh, rad) / sc // in points
			var c color.NRGBA
			switch {
			case d > 0:
				continue
			case d > -1: // the outer hairline
				c = color.NRGBA{A: uint8(hairA * 255 * math.Min(1, d+1.5))}
			case d > -2: // the specular line
				t := (float64(x)/fw + float64(y)/fh) / 2
				a := 0.35 + (0.08-0.35)*t
				c = color.NRGBA{R: 255, G: 255, B: 255, A: uint8(a * 255 * math.Min(1, d+2.5))}
			default:
				continue
			}
			img.SetNRGBA(x, y, c)
		}
	}
	r.rimImg, r.rimFor = img, [3]int{w, h, pol}
	return img
}

// refractBand is the width of the refraction band, in points.
const refractBand = 10

// drawRefraction samples the captured backdrop inside a 10 px band along the rim, magnified
// by 4 % about the surface's centre, so content bends at the edge like a lens.
func (r *floatRenderer) drawRefraction(w, h int) image.Image {
	src := r.g.backdrop
	if src == nil {
		return image.NewNRGBA(image.Rect(0, 0, 1, 1))
	}
	if r.refractImg != nil && r.refractFor == [2]int{w, h} && r.refractSource == src {
		return r.refractImg
	}
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	sc := float64(r.g.scale)
	if sc <= 0 {
		sc = 1
	}
	if c := fyne.CurrentApp().Driver().CanvasForObject(r.g); c != nil &&
		src.Bounds().Dx() != int(math.Round(float64(c.Size().Width)*sc)) {
		return image.NewNRGBA(image.Rect(0, 0, 1, 1)) // captured before the window was resized
	}
	at := fyne.CurrentApp().Driver().AbsolutePositionForObject(r.refract)
	ox, oy := float64(at.X)*sc, float64(at.Y)*sc
	b := src.Bounds()
	fw, fh := float64(w), float64(h)
	band := refractBand * sc
	cx, cy := fw/2, fh/2
	for y := range h {
		for x := range w {
			fx, fy := float64(x)+0.5, float64(y)+0.5
			if fx > band && fx < fw-band && fy > band && fy < fh-band {
				continue
			}
			sx := int(ox + cx + (fx-cx)/1.04)
			sy := int(oy + cy + (fy-cy)/1.04)
			if !(image.Point{sx, sy}.In(b)) {
				continue
			}
			cr, cg, cb, _ := src.At(sx, sy).RGBA()
			img.SetNRGBA(x, y, color.NRGBA{uint8(cr >> 8), uint8(cg >> 8), uint8(cb >> 8), 0xff})
		}
	}
	r.refractImg, r.refractFor, r.refractSource = img, [2]int{w, h}, src
	return img
}

// Plate is the opaque inner surface that text-dense content sits on inside a glass surface
// (D21 as revised: lists, setting rows and message bodies stay readable over any backdrop).
// On a flat surface it draws nothing, so Flat looks as it did.
type Plate struct {
	widget.BaseWidget
	Content fyne.CanvasObject
	// Margin is space left around the plate on glass, so the glass shows between it and what
	// is next to it; flat, there is none.
	Margin Margin
}

// Margin is space on each side.
type Margin struct{ Top, Bottom, Left, Right float32 }

// NewPlate puts content on a plate.
func NewPlate(content fyne.CanvasObject) *Plate {
	p := &Plate{Content: content}
	p.ExtendBaseWidget(p)
	return p
}

// CreateRenderer implements fyne.Widget.
func (p *Plate) CreateRenderer() fyne.WidgetRenderer {
	r := &plateRenderer{p: p, bg: canvas.NewRectangle(nil)}
	r.Refresh()
	return r
}

type plateRenderer struct {
	p  *Plate
	bg *canvas.Rectangle
}

// PlateColor is a plate's fill: the opaque Overlay surface on glass, nothing when flat.
func PlateColor() color.Color {
	if !theme.GlassOn() {
		return color.Transparent
	}
	return theme.Current().Surface.Overlay
}

func (r *plateRenderer) Refresh() {
	r.bg.FillColor = PlateColor()
	r.bg.CornerRadius = theme.RadiusControl
	r.bg.Refresh()
	r.Layout(r.p.Size())
	r.p.Content.Refresh()
}

func (r *plateRenderer) margin() Margin {
	if theme.GlassOn() {
		return r.p.Margin
	}
	return Margin{}
}

func (r *plateRenderer) Layout(s fyne.Size) {
	m := r.margin()
	at := fyne.NewPos(m.Left, m.Top)
	in := fyne.NewSize(s.Width-m.Left-m.Right, s.Height-m.Top-m.Bottom)
	r.bg.Move(at)
	r.bg.Resize(in)
	r.p.Content.Move(at)
	r.p.Content.Resize(in)
}

func (r *plateRenderer) MinSize() fyne.Size {
	m := r.margin()
	return r.p.Content.MinSize().AddWidthHeight(m.Left+m.Right, m.Top+m.Bottom)
}

func (r *plateRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.bg, r.p.Content}
}

func (r *plateRenderer) Destroy() {}
