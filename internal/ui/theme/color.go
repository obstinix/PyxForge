package theme

import (
	"fmt"
	"image/color"
	"math"
	"strconv"
)

// hex parses "#rrggbb" or "#rrggbbaa". Every call site is a literal in this package and
// TestPalettesAreWellFormed loads them all, so a malformed value fails the build's tests.
func hex(s string) color.NRGBA {
	if len(s) != 7 && len(s) != 9 || s[0] != '#' {
		panic(fmt.Sprintf("theme: malformed color %q", s))
	}
	v, err := strconv.ParseUint(s[1:], 16, 32)
	if err != nil {
		panic(fmt.Sprintf("theme: malformed color %q: %v", s, err))
	}
	if len(s) == 7 {
		return color.NRGBA{R: uint8(v >> 16), G: uint8(v >> 8), B: uint8(v), A: 0xff}
	}
	return color.NRGBA{R: uint8(v >> 24), G: uint8(v >> 16), B: uint8(v >> 8), A: uint8(v)}
}

// rgba builds a color the way the design sources write it: rgba(232, 230, 225, 0.08).
func rgba(r, g, b uint8, a float64) color.NRGBA {
	return color.NRGBA{R: r, G: g, B: b, A: uint8(math.Round(a * 255))}
}

// Over composites a possibly translucent src onto dst and returns an opaque color.
func Over(src, dst color.NRGBA) color.NRGBA {
	a := float64(src.A) / 255
	mix := func(s, d uint8) uint8 { return uint8(math.Round(float64(s)*a + float64(d)*(1-a))) }
	return color.NRGBA{R: mix(src.R, dst.R), G: mix(src.G, dst.G), B: mix(src.B, dst.B), A: 0xff}
}

// luminance is the WCAG 2.x relative luminance of an opaque color.
func luminance(c color.NRGBA) float64 {
	lin := func(v uint8) float64 {
		s := float64(v) / 255
		if s <= 0.04045 {
			return s / 12.92
		}
		return math.Pow((s+0.055)/1.055, 2.4)
	}
	return 0.2126*lin(c.R) + 0.7152*lin(c.G) + 0.0722*lin(c.B)
}

// Contrast is the WCAG contrast ratio of fg drawn on bg. A translucent fg is composited
// onto bg first; bg must be opaque.
func Contrast(fg, bg color.NRGBA) float64 {
	l1, l2 := luminance(Over(fg, bg)), luminance(bg)
	if l1 < l2 {
		l1, l2 = l2, l1
	}
	return (l1 + 0.05) / (l2 + 0.05)
}
