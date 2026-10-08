package theme

// InkGlass is the second light theme (Q1). Its signature is cool: blue-grey chrome around a
// near-white page, ink with a cool cast, blue-black literals, and the clearest glass of the
// five when the glass setting is on. IG:n cites the Ink & Glass DESIGN.md; the surfaces and
// text are derived from it, because its neutral greys (#f9f9f9 / #ffffff / #eeeeee) were
// indistinguishable from Ink & Paper at shell scale (Phase 1 review).
var InkGlass = Palette{
	ID:       "ink-glass",
	Name:     "Ink & Glass",
	Polarity: Light,
	Surface: Surfaces{
		Base:    hex("#f3f6f9"), // derived from IG:4 surface, cooled
		Raised:  hex("#dde5ec"), // derived: blue-grey chrome for rail and tab bars
		Sunken:  hex("#ebeff3"), // derived from IG:9 surface-container, cooled
		Overlay: hex("#ffffff"), // IG:109 white glass
	},
	Text: Text{
		Primary:   hex("#0b0d10"), // IG:108 ink with a cool cast
		Secondary: hex("#3e4651"), // derived from IG:13, cooled
		Tertiary:  hex("#56606b"), // derived from IG:16, cooled, at 4.5:1 on every surface
		Disabled:  hex("#97a0aa"), // rule: about 2.7:1, weaker than Tertiary
	},
	Border: Borders{
		Hairline: rgba(15, 35, 55, 0.12), // derived from K glass panel border, cool
		Strong:   rgba(15, 35, 55, 0.26), // derived from L --border-hairline-strong, cool
	},
	State: States{
		Hover:    rgba(15, 35, 55, 0.05), // rule, cool
		Pressed:  rgba(15, 35, 55, 0.12), // rule, cool
		Selected: rgba(15, 35, 55, 0.08), // rule, cool
		Focus:    rgba(15, 35, 55, 0.16), // rule: focus wash
	},
	Glass: Glass{
		Fill:          rgba(255, 255, 255, 0.70), // IG:109 #FFFFFF80, raised for legibility
		Edge:          hex("#ffffff"),            // IG:109 white top edge
		Rim:           rgba(15, 35, 55, 0.14),    // K glass panel border, cool
		Scrim:         rgba(20, 30, 40, 0.18),    // rule: light scrim
		Shadow:        rgba(15, 35, 55, 0.14),    // IG:138, cool
		ShadowBlur:    28,                        // IG:138 "large radius"
		ShadowOffsetY: 10,                        // rule
		Blur:          24,                        // IG:109 backdrop blur, glass setting
	},
	Status: Status{
		Success:  hex("#2f6b3a"), // derived, as Ink & Paper
		Warning:  hex("#8a5300"), // derived, as Ink & Paper
		Error:    hex("#ba1a1a"), // IG:32
		Info:     hex("#3e4651"), // rule: Info = Text.Secondary
		OnStatus: hex("#ffffff"),
	},
	Syntax: Syntax{
		Comment:   hex("#56606b"), // = Text.Tertiary
		Keyword:   hex("#000000"), // IG:108, bold in the editor
		Type:      hex("#3a4552"), // derived from IG:39, cooled
		Function:  hex("#141a21"), // derived from IG:38, cooled
		Variable:  hex("#1d242c"), // derived from IG:12, cooled
		Constant:  hex("#2c4f73"), // derived: blue-black ink for literals
		String:    hex("#2c4f73"), // derived: blue-black ink for literals
		Number:    hex("#2c4f73"), // derived: blue-black ink for literals
		Operator:  hex("#3e4651"), // = Text.Secondary
		Attribute: hex("#3a4552"), // derived from IG:39, cooled
		Error:     hex("#ba1a1a"), // = Status.Error
	},
}
