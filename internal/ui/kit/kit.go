// Package kit holds PyxForge's design-system primitives: themed text, icons, surfaces, rules,
// icon buttons and the glass panel. Each primitive stores a semantic role, not a colour, and
// re-reads the tokens in Refresh, which Fyne runs on every theme change.
package kit

import (
	"image/color"

	"fyne.io/fyne/v2"
	"github.com/obstinix/PyxForge/internal/ui/theme"
)

// Role is a semantic colour for text and icons.
type Role int

const (
	Primary Role = iota
	Secondary
	Tertiary
	Disabled
	Accent
	OnAccent
	Success
	Warning
	Error
	Inverse // Surface.Base: marks drawn on a Text.Primary fill
)

func (r Role) color(t theme.Tokens) color.NRGBA {
	switch r {
	case Secondary:
		return t.Text.Secondary
	case Tertiary:
		return t.Text.Tertiary
	case Disabled:
		return t.Text.Disabled
	case Accent:
		return t.Accent.Primary
	case OnAccent:
		return t.Accent.OnPrimary
	case Success:
		return t.Status.Success
	case Warning:
		return t.Status.Warning
	case Error:
		return t.Status.Error
	case Inverse:
		return t.Surface.Base
	}
	return t.Text.Primary
}

// Face is a typographic role (DESIGN_SYSTEM.md §7).
type Face int

const (
	Body    Face = iota // Geist 13
	Strong              // Geist SemiBold 13
	Label               // Geist Medium 11, section labels and captions
	Mono                // JetBrains Mono 13
	Display             // Syne SemiBold, panel and dialog titles
	Heading             // Geist SemiBold 16
)

func (f Face) font() fyne.Resource {
	switch f {
	case Strong, Heading:
		return theme.FontUIStrong
	case Label:
		return theme.FontUIMedium
	case Mono:
		return theme.FontMono
	case Display:
		return theme.FontDisplay
	}
	return theme.FontUI
}

func (f Face) size() float32 {
	switch f {
	case Label:
		return theme.TextCaption
	case Heading:
		return theme.TextHeading
	case Mono:
		return theme.TextCode
	}
	return theme.TextBody
}

// SurfaceRole picks one of the four surface levels.
type SurfaceRole int

const (
	Base SurfaceRole = iota
	Raised
	Sunken
	Overlay
)

func (s SurfaceRole) color(t theme.Tokens) color.NRGBA {
	switch s {
	case Raised:
		return t.Surface.Raised
	case Sunken:
		return t.Surface.Sunken
	case Overlay:
		return t.Surface.Overlay
	}
	return t.Surface.Base
}
