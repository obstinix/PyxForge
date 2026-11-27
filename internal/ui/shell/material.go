package shell

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"

	"github.com/obstinix/PyxForge/internal/ui/kit"
	"github.com/obstinix/PyxForge/internal/ui/theme"
)

// Liquid Glass (D21): Settings › Appearance › Material chooses what floating surfaces are made
// of. Flat is the default.
const (
	prefMaterial  = "appearance.material"  // "flat" or "glass"
	prefGlassBlur = "appearance.glassBlur" // blur strength: 0 Subtle, 1 Normal, 2 Strong
)

const (
	materialFlat  = "Flat"
	materialGlass = "Liquid Glass"
)

// blurRadii are the blur strengths, Subtle to Strong.
var blurRadii = []float32{theme.BlurSubtle, theme.BlurNormal, theme.BlurStrong}

var blurNames = []string{"Subtle", "Normal", "Strong"}

// glassBlurStep is the saved blur strength, 0…2.
func glassBlurStep(p fyne.Preferences) int {
	return min(max(p.IntWithFallback(prefGlassBlur, 1), 0), len(blurRadii)-1)
}

// loadMaterial gives the theme the material in the user's preferences.
func loadMaterial(p fyne.Preferences) {
	m := theme.Flat
	if p.String(prefMaterial) == "glass" {
		m = theme.Glass
	}
	theme.SetMaterial(m, blurRadii[glassBlurStep(p)])
}

// setMaterial saves a material and blur strength and redraws everything that floats.
func (s *Shell) setMaterial(glass bool, step int) {
	p := s.app.Preferences()
	v := "flat"
	if glass {
		v = "glass"
	}
	p.SetString(prefMaterial, v)
	p.SetInt(prefGlassBlur, step)
	loadMaterial(p)
	s.content.Refresh()
	for _, o := range s.win.Canvas().Overlays().List() {
		o.Refresh()
	}
	for _, f := range s.settingsHooks {
		f()
	}
	s.logf("Appearance: material %s, blur %s", v, blurNames[step])
}

// materialRows are Appearance's Material, Blur strength and preview rows.
func (m *settingsModal) materialRows(p *settingsPage) []*kit.SettingRow {
	s := m.s
	pr := s.app.Preferences()
	glass := theme.GlassOn()
	step := glassBlurStep(pr)
	var blur *kit.Slider
	material := kit.NewDropdown([]string{materialFlat, materialGlass}, map[bool]string{false: materialFlat, true: materialGlass}[glass],
		func(v string) {
			s.setMaterial(v == materialGlass, glassBlurStep(pr))
			if v == materialGlass {
				blur.Enable()
			} else {
				blur.Disable()
			}
		})
	material.Label = "Material"
	blur = kit.NewSlider("Blur strength", 0, 2, 1, float64(step), func(v float64) {
		s.setMaterial(theme.GlassOn(), int(v))
	})
	blur.Format = func(v float64) string { return blurNames[int(v)] }
	if !glass {
		blur.Disable()
	}
	return []*kit.SettingRow{
		m.row(p, prefMaterial, "Material", "What the palette, Settings, notifications, tooltips and completion boxes are made of. Liquid Glass blurs what is behind them; lists and descriptions stay on a solid plate so they read as well as on Flat.",
			material, "glass", "liquid", "blur", "translucent", "flat"),
		m.row(p, prefGlassBlur, "Blur strength", "How much the glass blurs what is behind it.", blur, "glass", "blur"),
		kit.NewSettingRow("Preview", "Glass over a sample of code, as the current setting draws it.", newMaterialPreview(s)),
	}
}

// newMaterialPreview is a card of sample code with a small floating surface over it, drawn in
// the current material.
func newMaterialPreview(s *Shell) fyne.CanvasObject {
	t := theme.Current()
	code := []struct {
		text string
		c    color.NRGBA
	}{
		{"; boot sector: print a greeting over the serial port", t.Syntax.Comment},
		{"start:  mov si, greeting", t.Syntax.Keyword},
		{"        call serial_puts", t.Syntax.Function},
		{"        jmp $", t.Syntax.Keyword},
		{"greeting: db \"PyxForge\", 13, 10, 0", t.Syntax.String},
		{"times 510-($-$$) db 0", t.Syntax.Number},
		{"dw 0xaa55", t.Syntax.Constant},
	}
	lines := container.NewVBox()
	for _, l := range code {
		tx := canvas.NewText(l.text, l.c)
		tx.TextStyle.Monospace = true
		tx.TextSize = theme.TextCode
		lines.Add(tx)
	}
	card := kit.NewGlassFloat(container.NewVBox(
		kit.NewText("Go to File", kit.Strong, kit.Primary),
		kit.NewPlate(container.NewPadded(kit.NewText("boot.asm  ·  src", kit.Body, kit.Secondary))),
	))
	card.Padding = theme.Space3
	floating := container.New(&previewLayout{}, card)
	bg := kit.NewSurface(kit.Base, nil)
	frame := canvas.NewRectangle(color.Transparent)
	frame.StrokeColor, frame.StrokeWidth, frame.CornerRadius = t.Border.Hairline, 1, theme.RadiusPanel
	preview := container.NewStack(bg, container.NewPadded(lines), floating, frame)
	s.onAppearance(func() {
		card.Refresh()
		bg.Refresh()
	})
	return container.NewGridWrap(fyne.NewSize(420, 150), preview)
}

// previewLayout floats the preview's card over the middle of the sample code, so the code
// shows through the glass.
type previewLayout struct{}

func (previewLayout) Layout(objs []fyne.CanvasObject, s fyne.Size) {
	c := objs[0]
	ms := c.MinSize()
	w := max(ms.Width, 200)
	c.Resize(fyne.NewSize(w, ms.Height))
	c.Move(fyne.NewPos(s.Width*0.3, (s.Height-ms.Height)/2))
}

func (previewLayout) MinSize([]fyne.CanvasObject) fyne.Size { return fyne.Size{} }

// SetMaterial chooses Flat or Liquid Glass and the blur strength (0 Subtle, 1 Normal, 2
// Strong), as Settings › Appearance does.
func (s *Shell) SetMaterial(glass bool, step int) {
	s.setMaterial(glass, min(max(step, 0), len(blurRadii)-1))
}
