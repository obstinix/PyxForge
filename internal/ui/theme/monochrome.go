package theme

// Monochrome is the minimal dark theme (Q4). D cites the 2.x docs/DESIGN.md §3 tokens; M cites
// the 2.x desktop/src/themes/mono.css. Syntax is a neutral ramp: weight, not hue, separates
// keywords.
var Monochrome = Palette{
	ID:       "monochrome",
	Name:     "Monochrome",
	Polarity: Dark,
	Surface: Surfaces{
		// Deepened from D (#111111, #161616, #0a0a0a) so it never reads as Smoked Kraft: true
		// neutral greys over a black well (Phase 1 review).
		Base:    hex("#0c0c0c"), // derived from D --bg-surface
		Raised:  hex("#151515"), // derived from D --bg-surface-raised
		Sunken:  hex("#000000"), // derived from D --bg-void
		Overlay: hex("#171717"), // derived from D --bg-surface-raised
	},
	Text: Text{
		Primary:   hex("#ededed"), // D --text-primary
		Secondary: hex("#a0a0a0"), // M --text-secondary
		Tertiary:  hex("#8c8c8c"), // D --text-secondary #8a8f98, neutralized
		Disabled:  hex("#5a5a5a"), // D --text-tertiary #5a5f68, neutralized
	},
	Border: Borders{
		Hairline: rgba(255, 255, 255, 0.10), // D --border-hairline, crisper on the deeper base
		Strong:   rgba(255, 255, 255, 0.20), // D --border-hairline-strong, crisper
	},
	State: States{
		Hover:    rgba(255, 255, 255, 0.04), // rule
		Pressed:  rgba(255, 255, 255, 0.12), // rule
		Selected: rgba(255, 255, 255, 0.08), // rule
		Focus:    rgba(255, 255, 255, 0.16), // rule: focus wash
	},
	Glass: Glass{
		Fill:          rgba(23, 23, 23, 0.80),    // D --bg-surface-raised, translucent
		Edge:          rgba(255, 255, 255, 0.14), // rule: specular lip
		Rim:           rgba(255, 255, 255, 0.20), // = Border.Strong
		Scrim:         rgba(0, 0, 0, 0.5),        // rule: dark scrim
		Shadow:        rgba(0, 0, 0, 0.6),        // rule: dark shadow
		ShadowBlur:    32,                        // rule
		ShadowOffsetY: 12,                        // rule
		Blur:          20,                        // rule: glass setting
	},
	Status: Status{
		Success:  hex("#10b981"), // D --status-success
		Warning:  hex("#f59e0b"), // D --status-warning
		Error:    hex("#ef4444"), // D --status-error
		Info:     hex("#a0a0a0"), // rule: Info = Text.Secondary
		OnStatus: hex("#0a0a0a"),
	},
	Syntax: Syntax{
		Comment:   hex("#8c8c8c"), // = Text.Tertiary
		Keyword:   hex("#ffffff"), // ramp top, bold in the editor
		Type:      hex("#d4d4d4"), // ramp
		Function:  hex("#ededed"), // = Text.Primary
		Variable:  hex("#c8c8c8"), // ramp
		Constant:  hex("#bdbdbd"), // ramp
		String:    hex("#b0b0b0"), // ramp
		Number:    hex("#bdbdbd"), // ramp
		Operator:  hex("#a0a0a0"), // = Text.Secondary
		Attribute: hex("#c8c8c8"), // ramp
		Error:     hex("#ef4444"), // = Status.Error
	},
}
