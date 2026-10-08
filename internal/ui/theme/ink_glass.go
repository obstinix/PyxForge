package theme

// InkGlass is the second light theme (Q1). It differs from Ink & Paper by its cooler neutral
// paper, pure ink, and a tinted glass layer on overlays. IG:n cites the Ink & Glass DESIGN.md.
var InkGlass = Palette{
	ID:       "ink-glass",
	Name:     "Ink & Glass",
	Polarity: Light,
	Surface: Surfaces{
		Base:    hex("#f9f9f9"), // IG:4 surface
		Raised:  hex("#ffffff"), // IG:7 surface-container-lowest
		Sunken:  hex("#eeeeee"), // IG:9 surface-container
		Overlay: hex("#ffffff"), // IG:109 white glass
	},
	Text: Text{
		Primary:   hex("#000000"), // IG:108 ink
		Secondary: hex("#4c4546"), // IG:13
		Tertiary:  hex("#6e6566"), // IG:16 outline #7e7576, darkened to 4.5:1
		Disabled:  hex("#9f9798"), // rule: about 2.7:1, weaker than Tertiary
	},
	Border: Borders{
		Hairline: rgba(0, 0, 0, 0.10), // K glass panel border (computed)
		Strong:   rgba(0, 0, 0, 0.25), // L --border-hairline-strong
	},
	State: States{
		Hover:    rgba(0, 0, 0, 0.04), // rule
		Pressed:  rgba(0, 0, 0, 0.10), // rule
		Selected: rgba(0, 0, 0, 0.06), // rule
	},
	Glass: Glass{
		Fill:          rgba(255, 255, 255, 0.94), // IG:109 #FFFFFF80 raised to D3 opacity
		Edge:          hex("#ffffff"),            // IG:109 white top edge
		Rim:           rgba(0, 0, 0, 0.10),       // K glass panel border
		Scrim:         rgba(26, 28, 28, 0.18),    // rule: light scrim
		Shadow:        rgba(0, 0, 0, 0.10),       // IG:138
		ShadowBlur:    28,                        // IG:138 "large radius"
		ShadowOffsetY: 10,                        // rule
	},
	Status: Status{
		Success:  hex("#2f6b3a"), // derived, as Ink & Paper
		Warning:  hex("#8a5300"), // derived, as Ink & Paper
		Error:    hex("#ba1a1a"), // IG:32
		Info:     hex("#4c4546"), // rule: Info = Text.Secondary
		OnStatus: hex("#ffffff"),
	},
	Syntax: Syntax{
		Comment:   hex("#6e6566"), // = Text.Tertiary
		Keyword:   hex("#000000"), // IG:108, bold in the editor
		Type:      hex("#474747"), // IG:39
		Function:  hex("#1b1b1b"), // IG:38
		Variable:  hex("#1a1c1c"), // IG:12
		Constant:  hex("#464745"), // IG:43
		String:    hex("#5e5f5d"), // IG:24 secondary
		Number:    hex("#5e5f5d"), // IG:24 secondary
		Operator:  hex("#4c4546"), // IG:13
		Attribute: hex("#474747"), // IG:39
		Error:     hex("#ba1a1a"), // = Status.Error
	},
}
