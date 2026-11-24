package termview

import (
	"image/color"

	"github.com/obstinix/PyxForge/internal/ui/theme"
)

// Palette is the 16 ANSI colours in a theme: the status and syntax colours, so a compiler's
// red error and green success read as the rest of PyxForge does. They are the colours the
// Neovim terminal had.
func Palette(t theme.Tokens) [16]color.NRGBA {
	over := func(c color.NRGBA) color.NRGBA { return theme.Over(c, t.Surface.Base) }
	sx, st := t.Syntax, t.Status
	black, white := over(t.Surface.Raised), over(t.Text.Primary)
	if t.Polarity == theme.Light {
		black, white = white, over(t.Surface.Raised)
	}
	return [16]color.NRGBA{
		black, over(st.Error), over(st.Success), over(st.Warning),
		over(st.Info), over(sx.Keyword), over(sx.Type), over(t.Text.Secondary),
		over(t.Text.Tertiary), over(sx.Error), over(sx.String), over(sx.Number),
		over(sx.Function), over(sx.Constant), over(sx.Attribute), white,
	}
}

// xterm256 is palette colour i from 16 on: the 6×6×6 cube, then 24 greys.
func xterm256(i uint8) color.NRGBA {
	if i >= 232 {
		g := 8 + 10*(i-232)
		return color.NRGBA{R: g, G: g, B: g, A: 0xff}
	}
	i -= 16
	level := func(n uint8) uint8 {
		if n == 0 {
			return 0
		}
		return 55 + 40*n
	}
	return color.NRGBA{R: level(i / 36), G: level(i / 6 % 6), B: level(i % 6), A: 0xff}
}
