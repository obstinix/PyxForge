package theme

// InkPaper is the primary light theme: flat, opaque, editorial. K cites the project kit's
// ink-and-paper main-workspace page; L cites the 2.x desktop/src/themes/ink-and-paper.css.
var InkPaper = Palette{
	ID:       "ink-paper",
	Name:     "Ink & Paper",
	Polarity: Light,
	Surface: Surfaces{
		Base:    hex("#faf9f6"), // K background, L --bg-void
		Raised:  hex("#ffffff"), // K surface-container-lowest
		Sunken:  hex("#f3f3f4"), // L --bg-inset
		Overlay: hex("#ffffff"), // flat paper overlay
	},
	Text: Text{
		Primary:   hex("#1a1c1c"), // L --text-primary
		Secondary: hex("#4c4546"), // L --text-secondary
		Tertiary:  hex("#6e6566"), // L --text-tertiary #7e7576, darkened to 4.5:1
		Disabled:  hex("#9f9798"), // rule: about 2.7:1, weaker than Tertiary
	},
	Border: Borders{
		Hairline: rgba(0, 0, 0, 0.12), // L --border-hairline
		Strong:   rgba(0, 0, 0, 0.25), // L --border-hairline-strong
	},
	State: States{
		Hover:    rgba(0, 0, 0, 0.04), // rule
		Pressed:  rgba(0, 0, 0, 0.10), // rule
		Selected: rgba(0, 0, 0, 0.06), // L --accent-dim
	},
	Glass: Glass{
		Fill:          hex("#ffffff"),        // flat: Ink & Paper has no glass
		Edge:          rgba(0, 0, 0, 0),      // none
		Rim:           rgba(0, 0, 0, 0.12),   // L --border-hairline
		Scrim:         rgba(26, 28, 28, 0.2), // rule: light scrim
		Shadow:        rgba(0, 0, 0, 0.10),   // IG:138
		ShadowBlur:    24,                    // rule
		ShadowOffsetY: 8,                     // rule
	},
	Status: Status{
		Success:  hex("#2f6b3a"), // derived: light-theme green at 6:1
		Warning:  hex("#8a5300"), // derived: light-theme amber at 6:1
		Error:    hex("#ba1a1a"), // K error, IG:32
		Info:     hex("#4c4546"), // rule: Info = Text.Secondary
		OnStatus: hex("#ffffff"),
	},
	Syntax: Syntax{
		Comment:   hex("#6e6566"), // = Text.Tertiary
		Keyword:   hex("#000000"), // IG:19 ink, bold in the editor
		Type:      hex("#474747"), // IG:39
		Function:  hex("#1b1b1b"), // IG:38
		Variable:  hex("#4c4546"), // L --text-secondary
		Constant:  hex("#6b4e2e"), // derived: sepia ink for literals
		String:    hex("#6b4e2e"), // derived: sepia ink for literals
		Number:    hex("#6b4e2e"), // derived: sepia ink for literals
		Operator:  hex("#4c4546"), // L --text-secondary
		Attribute: hex("#474747"), // IG:39
		Error:     hex("#ba1a1a"), // = Status.Error
	},
}
