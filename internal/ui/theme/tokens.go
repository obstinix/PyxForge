// Package theme holds PyxForge's design tokens and the Fyne theme built from them.
//
// A theme is data: a Palette (surfaces, text, borders, glass, status, syntax) plus an Accent
// chosen independently. Widgets read semantic tokens through Current and never branch on a
// theme's name. Every value cites its source in docs/design/DESIGN_SYSTEM.md.
package theme

import "image/color"

// Polarity says whether a palette is drawn light-on-dark or dark-on-light. Accents keep one
// value set per polarity so they stay readable on every palette.
type Polarity int

const (
	Dark Polarity = iota
	Light
)

// Surfaces run from the sunken wells up to floating overlays.
type Surfaces struct {
	Base    color.NRGBA // editor and panel background
	Raised  color.NRGBA // rail, tab bars, panel headers
	Sunken  color.NRGBA // terminal, status bar, input wells
	Overlay color.NRGBA // opaque body colour of palette, dialogs, notifications
}

// Text is the four-step reading hierarchy.
type Text struct {
	Primary   color.NRGBA
	Secondary color.NRGBA
	Tertiary  color.NRGBA // metadata; still meets 4.5:1
	Disabled  color.NRGBA // exempt from WCAG contrast, kept visibly weaker than Tertiary
}

// Borders are 1 px rules. Dividers between panels use Hairline.
type Borders struct {
	Hairline color.NRGBA
	Strong   color.NRGBA
}

// States are translucent washes laid over whatever surface is underneath.
type States struct {
	Hover    color.NRGBA
	Pressed  color.NRGBA
	Selected color.NRGBA
}

// Glass is the overlay material. Decision D3 and Q2: tinted, near-opaque, no backdrop blur;
// Blur stays 0 unless that decision changes.
type Glass struct {
	Fill          color.NRGBA // 92–96 % opaque tint
	Edge          color.NRGBA // 1 px specular line on the top edge
	Rim           color.NRGBA // 1 px outline
	Scrim         color.NRGBA // dims the workspace behind a modal
	Shadow        color.NRGBA
	ShadowBlur    float32
	ShadowOffsetY float32
	Blur          float32 // backdrop blur radius in px; 0 disables it
}

// Status colours are functional only: diagnostics, notifications, process state.
type Status struct {
	Success  color.NRGBA
	Warning  color.NRGBA
	Error    color.NRGBA
	Info     color.NRGBA
	OnStatus color.NRGBA // text drawn on a solid status fill
}

// Syntax is the semantic palette shared by the editor and the terminal. Keywords are also
// set in bold by the editor theme, so their colour can stay quiet.
type Syntax struct {
	Comment   color.NRGBA
	Keyword   color.NRGBA
	Type      color.NRGBA
	Function  color.NRGBA
	Variable  color.NRGBA
	Constant  color.NRGBA
	String    color.NRGBA
	Number    color.NRGBA
	Operator  color.NRGBA
	Attribute color.NRGBA
	Error     color.NRGBA
}

// Palette is everything a theme defines except its accent.
type Palette struct {
	ID       string
	Name     string
	Polarity Polarity
	Surface  Surfaces
	Text     Text
	Border   Borders
	State    States
	Glass    Glass
	Status   Status
	Syntax   Syntax
}

// AccentSet is one accent's values for one polarity.
type AccentSet struct {
	Primary   color.NRGBA // links, active markers, primary buttons
	Hover     color.NRGBA
	Active    color.NRGBA
	Muted     color.NRGBA // tinted fills behind accented content
	Focus     color.NRGBA // the single 2 px focus ring colour
	Selection color.NRGBA // selected rows and text
	Border    color.NRGBA
	OnPrimary color.NRGBA // text on a Primary fill
}

// Accent is chosen independently of the palette (decision D2).
type Accent struct {
	ID    string
	Name  string
	Dark  AccentSet
	Light AccentSet
}

// Tokens are a palette resolved with an accent: what widgets actually read.
type Tokens struct {
	Palette
	AccentID string
	Accent   AccentSet
}

// Resolve pairs a palette with the accent values for its polarity.
func Resolve(p Palette, a Accent) Tokens {
	set := a.Dark
	if p.Polarity == Light {
		set = a.Light
	}
	return Tokens{Palette: p, AccentID: a.ID, Accent: set}
}
