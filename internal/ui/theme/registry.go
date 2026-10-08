package theme

import (
	"fyne.io/fyne/v2"
	fynetheme "fyne.io/fyne/v2/theme"
)

// SystemID selects a palette from the operating system's light or dark preference (Q5).
const SystemID = "system"

// Palettes lists the user-selectable themes in menu order.
var Palettes = []Palette{SmokedKraft, InkPaper, InkGlass, VerdigrisForge, Monochrome}

// Accents lists the accents in menu order.
var Accents = []Accent{Crimson, Amber}

// Default is used when no preference is stored.
var Default = Selection{PaletteID: SystemID, AccentID: Crimson.ID}

// Selection is what the user picks in Settings → Appearance and what gets persisted.
type Selection struct {
	PaletteID string // a Palette ID or SystemID
	AccentID  string
	Glass     bool // translucent, blurred overlays; off by default (Q2, revised in Phase 1)
}

// PaletteByID looks up a palette; ok is false for unknown IDs and for SystemID.
func PaletteByID(id string) (Palette, bool) {
	for _, p := range Palettes {
		if p.ID == id {
			return p, true
		}
	}
	return Palette{}, false
}

// AccentByID looks up an accent; ok is false for unknown IDs.
func AccentByID(id string) (Accent, bool) {
	for _, a := range Accents {
		if a.ID == id {
			return a, true
		}
	}
	return Accent{}, false
}

// Tokens resolves the selection for the given OS appearance. Unknown IDs fall back to the
// defaults rather than failing, so a stale preference never stops the app from starting.
func (s Selection) Tokens(v fyne.ThemeVariant) Tokens {
	p, ok := PaletteByID(s.PaletteID)
	if !ok {
		p = SmokedKraft
		if v == fynetheme.VariantLight {
			p = InkPaper
		}
	}
	a, ok := AccentByID(s.AccentID)
	if !ok {
		a = Crimson
	}
	return Resolve(p, a).WithGlass(s.Glass)
}
