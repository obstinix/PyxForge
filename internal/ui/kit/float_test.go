package kit

import (
	"image/color"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"

	"github.com/obstinix/PyxForge/internal/ui/theme"
)

func withMaterial(t *testing.T, m theme.Material) {
	t.Helper()
	theme.SetMaterial(m, theme.BlurNormal)
	t.Cleanup(func() { theme.SetMaterial(theme.Flat, theme.BlurNormal) })
}

func objectsOf(t *testing.T, o fyne.Widget) []fyne.CanvasObject {
	return test.TempWidgetRenderer(t, o).Objects()
}

// TestFloatMaterial: flat, a Float is the opaque body; with Liquid Glass, a glass Float is a blur
// under GlassTint and a rim, while a Float that is not glass (a menu) stays flat.
func TestFloatMaterial(t *testing.T) {
	pyxApp(t)
	content := NewText("x", Body, Primary)
	g := NewGlassFloat(content)
	if objs := objectsOf(t, g); len(objs) != 2 || g.IsGlass() {
		t.Fatalf("flat: %d objects, glass %v", len(objs), g.IsGlass())
	}
	withMaterial(t, theme.Glass)
	g.Refresh()
	objs := objectsOf(t, g)
	if !g.IsGlass() || len(objs) != 4 {
		t.Fatalf("glass: %d objects", len(objs))
	}
	if b, ok := objs[0].(*canvas.Blur); !ok || b.Radius != theme.BlurNormal {
		t.Errorf("the first glass layer is %T, not the blur", objs[0])
	}
	if tint := objs[1].(*canvas.Rectangle); tint.FillColor != theme.Current().Float.GlassTint {
		t.Errorf("tint %v, want GlassTint", tint.FillColor)
	}
	if menu := NewFloat(content); menu.IsGlass() {
		t.Error("a Float without Glass became glass")
	}
}

// TestGlassRim: the rim is brighter along the top and left than the bottom and right.
func TestGlassRim(t *testing.T) {
	pyxApp(t)
	withMaterial(t, theme.Glass)
	g := NewGlassFloat(NewText("x", Body, Primary))
	w := test.NewWindow(container.NewWithoutLayout(g))
	defer w.Close()
	w.Resize(fyne.NewSize(400, 300))
	g.Move(fyne.NewPos(20, 20))
	g.Resize(fyne.NewSize(300, 200))
	r := test.TempWidgetRenderer(t, g).(*floatRenderer)
	img := r.drawRim(300, 200)
	alpha := func(x, y int) uint8 { return color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA).A }
	top, bottom := alpha(150, 1), alpha(150, 198)
	if top == 0 || bottom == 0 || top <= bottom {
		t.Errorf("specular alpha top %d, bottom %d", top, bottom)
	}
	if alpha(150, 100) != 0 {
		t.Error("the rim draws inside the surface")
	}
}

// TestGlassOpens: a glass surface grows and fades in when opened, and appears at once under
// Reduce Motion and when flat.
func TestGlassOpens(t *testing.T) {
	pyxApp(t)
	g := NewGlassFloat(NewText("x", Body, Primary))
	g.open = 0
	g.Opened()
	if g.open != 1 {
		t.Error("a flat surface animated")
	}
	withMaterial(t, theme.Glass)
	old := ReduceMotion
	t.Cleanup(func() { ReduceMotion = old })
	ReduceMotion = func() bool { return true }
	g.open = 0
	g.Opened()
	if g.open != 1 {
		t.Error("glass animated under Reduce Motion")
	}
	ReduceMotion = func() bool { return false }
	g.Opened() // the test driver runs an animation to its end at once
	if g.open != 1 {
		t.Errorf("after opening, progress %v", g.open)
	}
}

// TestPlate: flat, a plate draws nothing and adds no margin; on glass it is the opaque Overlay
// surface with its margin.
func TestPlate(t *testing.T) {
	pyxApp(t)
	p := NewPlate(NewText("row", Body, Tertiary))
	p.Margin = Margin{Top: 8}
	flat := p.MinSize()
	if PlateColor() != color.Transparent {
		t.Error("a flat plate is painted")
	}
	withMaterial(t, theme.Glass)
	p.Refresh()
	if p.MinSize().Height != flat.Height+8 {
		t.Errorf("min height %v on glass, %v flat", p.MinSize().Height, flat.Height)
	}
	if PlateColor() != theme.Current().Surface.Overlay {
		t.Error("a glass plate is not the Overlay surface")
	}
}

// TestRefract: Refract captures the canvas for the rim's refraction band, drawn under the blur.
func TestRefract(t *testing.T) {
	pyxApp(t)
	withMaterial(t, theme.Glass)
	g := NewGlassFloat(NewText("x", Body, Primary))
	bg := canvas.NewRectangle(color.NRGBA{200, 40, 40, 255})
	layer := container.NewWithoutLayout()
	w := test.NewWindow(container.NewStack(bg, layer))
	defer w.Close()
	w.Resize(fyne.NewSize(400, 300))
	g.Refract(w.Canvas()) // before it is shown, as the palette and Settings do
	layer.Add(g)
	g.Move(fyne.NewPos(50, 50))
	g.Resize(fyne.NewSize(200, 120))
	r := test.TempWidgetRenderer(t, g).(*floatRenderer)
	if _, ok := r.Objects()[0].(*canvas.Raster); !ok {
		t.Fatalf("the first layer after Refract is %T", r.Objects()[0])
	}
	img := r.drawRefraction(200, 120)
	if c := color.NRGBAModel.Convert(img.At(2, 60)).(color.NRGBA); c.R < 150 || c.A != 255 {
		t.Errorf("the band at the left edge is %v, not the red backdrop", c)
	}
	if c := color.NRGBAModel.Convert(img.At(100, 60)).(color.NRGBA); c.A != 0 {
		t.Errorf("the middle is %v; only the band refracts", c)
	}
}
