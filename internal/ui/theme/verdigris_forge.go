package theme

// VerdigrisForge is the tinted industrial dark theme. VF:n cites the Verdigris Forge
// DESIGN.md. Surfaces use the frontmatter ladder the concept actually renders (SOURCES.md,
// "Computed values"); the prose canvas #142822 tints the glass.
var VerdigrisForge = Palette{
	ID:       "verdigris-forge",
	Name:     "Verdigris Forge",
	Polarity: Dark,
	Surface: Surfaces{
		Base:    hex("#111e1a"), // VF:8, rendered panels
		Raised:  hex("#1f2d28"), // VF:10
		Sunken:  hex("#091612"), // VF:4, rendered page
		Overlay: hex("#142822"), // VF:157 canvas
	},
	Text: Text{
		Primary:   hex("#f4ede2"), // VF:159 cream
		Secondary: hex("#c2c8c4"), // VF:13
		Tertiary:  hex("#8c928f"), // VF:16
		Disabled:  hex("#4e786b"), // VF:203 placeholder
	},
	Border: Borders{
		Hairline: rgba(244, 237, 226, 0.08), // rule: Text.Primary at 8 %
		Strong:   hex("#2d5a4c"),            // VF:158 division lines
	},
	State: States{
		Hover:    rgba(244, 237, 226, 0.04), // rule
		Pressed:  rgba(244, 237, 226, 0.12), // rule
		Selected: rgba(244, 237, 226, 0.08), // rule
		Focus:    rgba(244, 237, 226, 0.16), // rule: focus wash
	},
	Glass: Glass{
		Fill:          rgba(20, 40, 34, 0.80),    // VF:161 tint, translucent
		Edge:          rgba(244, 237, 226, 0.18), // VF:162 specular edge
		Rim:           rgba(244, 237, 226, 0.12), // rule
		Scrim:         rgba(0, 0, 0, 0.45),       // rule: dark scrim
		Shadow:        rgba(0, 0, 0, 0.6),        // rule: dark shadow
		ShadowBlur:    32,                        // rule
		ShadowOffsetY: 12,                        // rule
		Blur:          20,                        // rule: glass setting
	},
	Status: Status{
		Success:  hex("#72b896"), // VF:164 moss jade
		Warning:  hex("#d48243"), // VF:163 copper-amber
		Error:    hex("#ffb4ab"), // VF:32
		Info:     hex("#a1d1bf"), // VF:24 secondary
		OnStatus: hex("#091612"), // VF:4
	},
	Syntax: Syntax{
		Comment:   hex("#8c928f"), // VF:16
		Keyword:   hex("#a1d1bf"), // VF:204 sage statements
		Type:      hex("#b5ccc2"), // VF:19
		Function:  hex("#bcedda"), // VF:40
		Variable:  hex("#c2c8c4"), // VF:13
		Constant:  hex("#f4ede2"), // VF:204 cream constants
		String:    hex("#90bfae"), // VF:27
		Number:    hex("#ccc6bb"), // VF:28
		Operator:  hex("#d48243"), // VF:204 copper-amber operators
		Attribute: hex("#d0e8de"), // VF:36
		Error:     hex("#ffb4ab"), // = Status.Error
	},
}
