package shell

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"

	"github.com/obstinix/PyxForge/internal/ui/kit"
	"github.com/obstinix/PyxForge/internal/ui/notifications"
	"github.com/obstinix/PyxForge/internal/ui/theme"
)

// busyFile is assembly with long, coloured lines: a busy editor for glass to float over.
func busyFile(t *testing.T, root string) string {
	t.Helper()
	var b strings.Builder
	for i := range 60 {
		fmt.Fprintf(&b, "label_%02d:  mov ax, 0x%04x   ; \"string %d\" serial_put lodsb test al, al jz .done call print_hex\n", i, i*97, i)
	}
	p := filepath.Join(root, "busy.asm")
	if err := os.WriteFile(p, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// glassBackdrop renders the canvas with f's content hidden and returns the colours of f's
// glass: the blurred workspace actually behind it under the tint, without the rim.
func glassBackdrop(t *testing.T, c fyne.Canvas, f *kit.Float) []color.NRGBA {
	t.Helper()
	f.Content.Hide()
	img := c.Capture()
	f.Content.Show()
	d := fyne.CurrentApp().Driver()
	at, sz, sc := d.AbsolutePositionForObject(f), f.Size(), c.Scale()
	r := image.Rect(int(at.X*sc)+3, int(at.Y*sc)+3, int((at.X+sz.Width)*sc)-3, int((at.Y+sz.Height)*sc)-3).Intersect(img.Bounds())
	if r.Empty() {
		t.Fatalf("the glass surface is not on the canvas (%v %v)", at, sz)
	}
	// Inside the rounded corners only: outside them is the workspace itself.
	rad := float64(theme.RadiusOverlay*sc) + 3
	inside := func(x, y int) bool {
		cx := math.Max(float64(r.Min.X)+rad-3, math.Min(float64(x), float64(r.Max.X)-rad+3))
		cy := math.Max(float64(r.Min.Y)+rad-3, math.Min(float64(y), float64(r.Max.Y)-rad+3))
		return math.Hypot(float64(x)-cx, float64(y)-cy) <= rad-3
	}
	var out []color.NRGBA
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			if !inside(x, y) {
				continue
			}
			out = append(out, color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA))
		}
	}
	return out
}

// TestGlassContrast (D21 as revised by the owner): text on bare glass (the Settings page list
// and header, notification titles, tooltips) is primary and meets 4.5:1 against the glass as
// drawn over the workspace actually behind it, a busy editor with a selection and an accent
// button, at the weakest blur, in a narrow and a wide window; secondary and tertiary text do not
// sit on bare glass (measured at 3.10:1 over the accent button); denser text sits on a plate,
// the opaque Overlay surface, where every role meets 4.5:1 whatever is behind. Large text needs
// only 3:1, so it passes with primary.
//
// It renders and blurs in software, which the race detector slows past CI's timeout, and starts
// no goroutines, so it runs without the race detector (CI's test job on every OS).
func TestGlassContrast(t *testing.T) {
	if raceEnabled {
		t.Skip("software rendering is too slow under -race; runs in the job without it")
	}
	sizes := []fyne.Size{{Width: 1024, Height: 640}, {Width: 1440, Height: 900}}
	for _, size := range sizes {
		t.Run(fmt.Sprintf("%vx%v", size.Width, size.Height), func(t *testing.T) { glassContrastAt(t, size) })
	}
}

func glassContrastAt(t *testing.T, size fyne.Size) {
	s, root, q := newEditorShell(t)
	noToastExpiry(s)
	s.win.Resize(size)
	s.SetMaterial(true, 0) // Subtle: the least blur keeps the most of the backdrop
	s.OpenFile(busyFile(t, root))
	q.pumpUntil(t, "the file to open", func() bool { return s.activeDoc() != nil })
	s.Commands().Run("edit.selectAll")
	drainQueue(q)
	for _, p := range theme.Palettes {
		for _, a := range theme.Accents {
			t.Run(p.ID+"-"+a.ID, func(t *testing.T) {
				s.SetSelection(theme.Selection{PaletteID: p.ID, AccentID: a.ID})
				drainQueue(q)
				tk := theme.Current()

				if pc, ok := kit.PlateColor().(color.NRGBA); !ok || pc != tk.Surface.Overlay || pc.A != 0xff {
					t.Errorf("a plate is %v, not the opaque Overlay surface", kit.PlateColor())
				}
				for name, fg := range map[string]color.NRGBA{"primary": tk.Text.Primary, "secondary": tk.Text.Secondary, "tertiary": tk.Text.Tertiary} {
					if c := theme.Contrast(fg, tk.Surface.Overlay); c < 4.5 {
						t.Errorf("%s text on a plate: %.2f:1", name, c)
					}
				}

				surfaces := map[string]func() *kit.Float{
					"palette": func() *kit.Float {
						s.ShowCommands()
						return s.palette.Float()
					},
					"settings": func() *kit.Float {
						s.palette.Hide()
						s.OpenSettings()
						return s.settingsUI.panel
					},
					"notification": func() *kit.Float {
						s.closeSettings()
						s.Notify(notifications.Info, "Glass", "")
						objs := s.notes.Layer().(*fyne.Container).Objects
						return objs[len(objs)-1].(*kit.Float)
					},
				}
				for _, name := range []string{"palette", "settings", "notification"} {
					f := surfaces[name]()
					if !f.IsGlass() {
						t.Fatalf("the %s is not glass", name)
					}
					worst := map[string]float64{"primary": 99, "secondary": 99, "tertiary": 99}
					for _, bg := range glassBackdrop(t, s.win.Canvas(), f) {
						for role, fg := range map[string]color.NRGBA{"primary": tk.Text.Primary, "secondary": tk.Text.Secondary, "tertiary": tk.Text.Tertiary} {
							worst[role] = min(worst[role], theme.Contrast(fg, bg))
						}
					}
					for _, role := range []string{"primary"} {
						if c := worst[role]; c < 4.5 {
							t.Errorf("%s text on the %s's bare glass: %.2f:1 at worst", role, name, c)
						}
					}
					t.Logf("%s: worst primary %.2f, secondary %.2f, tertiary %.2f (only primary sits on bare glass)", name, worst["primary"], worst["secondary"], worst["tertiary"])
				}
				// The page list, the one list on bare glass, is all primary on glass.
				s.OpenSettings()
				labels := 0
				for _, o := range test.LaidOutObjects(s.settingsUI.nav) {
					if tx, ok := o.(*kit.Text); ok {
						labels++
						if tx.Role != kit.Primary {
							t.Errorf("%q is not primary text on bare glass", tx.Text)
						}
					}
				}
				if labels < 5 {
					t.Errorf("found %d labels in the page list", labels)
				}
				s.closeSettings()
				s.palette.Hide()
			})
		}
	}
}

// TestFlatIsDefault: with no preference saved, every floating surface is flat (D21).
func TestFlatIsDefault(t *testing.T) {
	s, _ := newTestShell(t)
	if theme.CurrentMaterial() != theme.Flat {
		t.Fatal("the default material is not Flat")
	}
	s.ShowCommands()
	if s.palette.Float().IsGlass() {
		t.Error("the palette is glass by default")
	}
	s.palette.Hide()
	s.openSettingsAt("appearance")
	d := s.settingsUI.rows[prefMaterial].Control.(*kit.Dropdown)
	if d.Selected != materialFlat {
		t.Errorf("Material shows %q", d.Selected)
	}
	if !s.settingsUI.rows[prefGlassBlur].Control.(*kit.Slider).Disabled() {
		t.Error("Blur strength is enabled while the material is Flat")
	}
}

// TestGlassIsOptIn: Liquid Glass comes only from Settings › Appearance › Material; it applies
// to the surfaces D21 lists and not to menus; the blur strength sets the radius; Flat brings
// everything back. The glass preference of the 2.x stack (D18) is still dropped.
func TestGlassIsOptIn(t *testing.T) {
	s, _ := newTestShell(t)
	if _, ok := s.Commands().Get("prefs.glass"); ok {
		t.Error("a glass command is registered; the material is a setting")
	}
	s.app.Preferences().SetBool("appearance.glass", true)
	loadSelection(s.app.Preferences())
	if s.app.Preferences().Bool("appearance.glass") || theme.GlassOn() {
		t.Error("the 2.x glass preference was kept or turned glass on")
	}

	s.openSettingsAt("appearance")
	d := s.settingsUI.rows[prefMaterial].Control.(*kit.Dropdown)
	blur := s.settingsUI.rows[prefGlassBlur].Control.(*kit.Slider)
	d.SetSelected(materialGlass)
	if !theme.GlassOn() || s.app.Preferences().String(prefMaterial) != "glass" || blur.Disabled() {
		t.Fatal("choosing Liquid Glass did not turn it on, save it and enable Blur strength")
	}
	if !s.settingsUI.panel.IsGlass() {
		t.Error("Settings is not glass")
	}
	blur.SetValue(2)
	if theme.GlassRadius() != theme.BlurStrong || s.app.Preferences().Int(prefGlassBlur) != 2 || blur.Text() != "Strong" {
		t.Errorf("Strong blur: radius %v, saved %d, shown %q", theme.GlassRadius(), s.app.Preferences().Int(prefGlassBlur), blur.Text())
	}
	s.closeSettings()
	s.ShowCommands()
	if !s.palette.Float().IsGlass() {
		t.Error("the palette is not glass")
	}
	s.palette.Hide()
	if kit.NewFloat(nil).IsGlass() {
		t.Error("a surface D21 does not list (a menu) became glass")
	}
	s.app.Preferences().SetString(prefMaterial, "flat")
	loadMaterial(s.app.Preferences())
	if theme.GlassOn() || s.palette.Float().IsGlass() {
		t.Error("Flat did not bring the flat surfaces back")
	}
}

// drainQueue runs the UI work already posted.
func drainQueue(q queue) {
	for {
		select {
		case f := <-q:
			f()
		default:
			return
		}
	}
}
