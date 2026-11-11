package kit

import (
	"image/color"
	"testing"

	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/test"
	"github.com/obstinix/PyxForge/internal/ui/theme"
)

// TestGlassPanes: with the glass setting on (the default), a pane is translucent over a
// backdrop that glows with the accent; with it off, both are the plain opaque surfaces. A
// surface that is not a pane, the editor's, is opaque either way.
func TestGlassPanes(t *testing.T) {
	if !theme.Default.Glass {
		t.Error("glass is not the default")
	}
	a := test.NewTempApp(t)
	for _, glass := range []bool{true, false} {
		sel := theme.Default
		sel.PaletteID, sel.Glass = theme.SmokedKraft.ID, glass
		a.Settings().SetTheme(theme.NewFyne(sel))
		tok := theme.Current()

		pane := test.TempWidgetRenderer(t, NewPane(Raised, nil)).Objects()[0].(*canvas.Rectangle).FillColor.(color.NRGBA)
		editor := test.TempWidgetRenderer(t, NewSurface(Base, nil)).Objects()[0].(*canvas.Rectangle).FillColor.(color.NRGBA)
		glows := test.TempWidgetRenderer(t, NewBackdrop()).Objects()
		_, _, _, glowA := glows[1].(*canvas.RadialGradient).StartColor.RGBA()

		if editor != tok.Surface.Base {
			t.Errorf("glass %v: the editor's surface is %v, not the opaque base", glass, editor)
		}
		switch {
		case glass && (pane.A >= 255 || pane.A < 128 || glowA == 0):
			t.Errorf("glass on: pane alpha %d (want translucent, at least half opaque), glow alpha %d", pane.A, glowA)
		case !glass && (pane != tok.Surface.Raised || glowA != 0):
			t.Errorf("glass off: pane %v (want %v), glow alpha %d", pane, tok.Surface.Raised, glowA)
		}
	}
}
