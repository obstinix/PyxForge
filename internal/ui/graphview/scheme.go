// Package graphview draws the project graph (D23, internal/graph) in one canvas.Raster with
// rasterx: no canvas object per node. Labels are the only text objects, and only for nodes
// large enough on screen, the hovered one and its neighbours.
package graphview

import (
	"image/color"
	"math"

	"github.com/obstinix/PyxForge/internal/ui/theme"
)

// Scheme is how nodes are coloured.
type Scheme int

const (
	SchemeTheme    Scheme = iota // the theme's own colours: accent and syntax
	SchemeSpectrum               // perceptually even hues at the theme's lightness
	SchemeGalaxy                 // a deep background, hues cycling through clusters
)

// SchemeNames are the schemes as Settings and the controls name them.
var SchemeNames = []string{"Theme", "Spectrum", "Galaxy"}

// Colours are a scheme resolved against the current theme.
type Colours struct {
	Background color.NRGBA
	Nodes      []color.NRGBA // the group colours, cycled
	Edge       color.NRGBA
	Label      color.NRGBA
	Halo       color.NRGBA // the focus ring around the keyboard's node
}

// MinNodeContrast is the contrast every node colour keeps against the background: WCAG's 3:1
// for graphics that carry meaning.
const MinNodeContrast = 3.0

// Resolve gives a scheme's colours in the theme. Every node colour is moved in lightness
// until it meets MinNodeContrast against the background; no colour is a fixed literal.
func Resolve(s Scheme, t theme.Tokens) Colours {
	light := t.Polarity == theme.Light
	c := Colours{Background: t.Surface.Base, Edge: t.Text.Tertiary, Label: t.Text.Secondary, Halo: t.Accent.Focus}
	switch s {
	case SchemeTheme:
		// The most distinct first (groups are numbered largest first); the quiet ones last.
		c.Nodes = []color.NRGBA{t.Syntax.Type, t.Syntax.Function, t.Syntax.String, t.Accent.Primary, t.Syntax.Number,
			t.Syntax.Constant, t.Syntax.Attribute, t.Status.Info, t.Status.Success, t.Status.Warning, t.Syntax.Keyword, t.Syntax.Variable}
	case SchemeSpectrum:
		l := 0.74
		if light {
			l = 0.50
		}
		c.Nodes = hues(12, l, 0.13, 25)
	case SchemeGalaxy:
		// A deep background: the theme's base taken most of the way to its darkest, whatever
		// the polarity, with light, saturated stars.
		deep := theme.Over(color.NRGBA{A: 0xd9}, t.Surface.Base)
		if light {
			deep = theme.Over(color.NRGBA{A: 0xe6}, t.Text.Primary)
		}
		c.Background, c.Edge, c.Label = deep, withAlpha(t.Accent.Muted, 0xff), color.NRGBA{0xe8, 0xe6, 0xf0, 0xff}
		c.Edge = mix(deep, c.Label, 0.35)
		c.Nodes = hues(12, 0.82, 0.12, 260)
		c.Halo = mix(c.Label, t.Accent.Primary, 0.5)
	}
	for i, n := range c.Nodes {
		c.Nodes[i] = ensureContrast(n, c.Background, MinNodeContrast)
	}
	c.Edge = ensureContrast(c.Edge, c.Background, 1.6) // links recede; they need not be read
	c.Label = ensureContrast(c.Label, c.Background, 4.5)
	return c
}

func withAlpha(c color.NRGBA, a uint8) color.NRGBA { c.A = a; return c }

// mix is a at weight 1-f, b at f.
func mix(a, b color.NRGBA, f float64) color.NRGBA {
	m := func(x, y uint8) uint8 { return uint8(math.Round(float64(x)*(1-f) + float64(y)*f)) }
	return color.NRGBA{m(a.R, b.R), m(a.G, b.G), m(a.B, b.B), 0xff}
}

// ensureContrast moves a colour toward white or black (whichever the background is not) until
// it meets ratio against the background.
func ensureContrast(c, bg color.NRGBA, ratio float64) color.NRGBA {
	c.A = 0xff
	if theme.Contrast(c, bg) >= ratio {
		return c
	}
	target := color.NRGBA{0xff, 0xff, 0xff, 0xff}
	if luminance(bg) > 0.4 {
		target = color.NRGBA{0, 0, 0, 0xff}
	}
	for f := 0.05; f <= 1; f += 0.05 {
		m := mix(c, target, f)
		if theme.Contrast(m, bg) >= ratio {
			return m
		}
	}
	return target
}

func luminance(c color.NRGBA) float64 {
	lin := func(v uint8) float64 {
		x := float64(v) / 255
		if x <= 0.04045 {
			return x / 12.92
		}
		return math.Pow((x+0.055)/1.055, 2.4)
	}
	return 0.2126*lin(c.R) + 0.7152*lin(c.G) + 0.0722*lin(c.B)
}

// hues are n colours evenly spaced in hue at OKLCH lightness l and chroma c, starting at
// start degrees: even to the eye, unlike HSL.
func hues(n int, l, c, start float64) []color.NRGBA {
	out := make([]color.NRGBA, n)
	for i := range n {
		h := (start + 360*float64(i)/float64(n)) * math.Pi / 180
		out[i] = oklch(l, c, h)
	}
	return out
}

// oklch converts an OKLCH colour to sRGB, chroma reduced until it fits the gamut.
func oklch(l, c, h float64) color.NRGBA {
	for ; c >= 0; c -= 0.01 {
		a, b := c*math.Cos(h), c*math.Sin(h)
		l_ := l + 0.3963377774*a + 0.2158037573*b
		m_ := l - 0.1055613458*a - 0.0638541728*b
		s_ := l - 0.0894841775*a - 1.2914855480*b
		L, M, S := l_*l_*l_, m_*m_*m_, s_*s_*s_
		r := 4.0767416621*L - 3.3077115913*M + 0.2309699292*S
		g := -1.2684380046*L + 2.6097574011*M - 0.3413193965*S
		bl := -0.0041960863*L - 0.7034186147*M + 1.7076147010*S
		if r >= 0 && r <= 1 && g >= 0 && g <= 1 && bl >= 0 && bl <= 1 {
			enc := func(x float64) uint8 {
				if x <= 0.0031308 {
					x *= 12.92
				} else {
					x = 1.055*math.Pow(x, 1/2.4) - 0.055
				}
				return uint8(math.Round(math.Max(0, math.Min(1, x)) * 255))
			}
			return color.NRGBA{enc(r), enc(g), enc(bl), 0xff}
		}
	}
	v := uint8(math.Round(l * 255))
	return color.NRGBA{v, v, v, 0xff}
}
