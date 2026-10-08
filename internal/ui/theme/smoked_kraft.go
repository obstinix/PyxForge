package theme

// SmokedKraft is the primary dark theme. SK:n cites line n of the Smoked Kraft DESIGN.md;
// "rule" values follow the shared alpha ladder documented in DESIGN_SYSTEM.md.
var SmokedKraft = Palette{
	ID:       "smoked-kraft",
	Name:     "Smoked Kraft",
	Polarity: Dark,
	Surface: Surfaces{
		// Warmed toward kraft from SK:173/9/174/217 (#1a1918, #201f1e, #141312, #1c1b1a): at
		// shell scale the reference greys read as Monochrome (Phase 1 review).
		Base:    hex("#1c1916"), // derived from SK:173 canvas
		Raised:  hex("#26211b"), // derived from SK:9: kraft chrome for rail and tab bars
		Sunken:  hex("#14120f"), // derived from SK:174 sub-surface
		Overlay: hex("#221e1a"), // derived from SK:217 L2 plate
	},
	Text: Text{
		Primary:   hex("#e8e6e1"), // SK:180 chalk
		Secondary: hex("#b5b0a8"), // SK:181 graphite
		Tertiary:  hex("#969088"), // SK:16 outline
		Disabled:  hex("#54514c"), // SK:239 placeholder
	},
	Border: Borders{
		Hairline: rgba(232, 230, 225, 0.08), // SK:216
		Strong:   rgba(232, 230, 225, 0.18), // SK:217 rim
	},
	State: States{
		Hover:    rgba(232, 230, 225, 0.04), // SK:251
		Pressed:  rgba(232, 230, 225, 0.12), // SK:182 chalk wash
		Selected: rgba(232, 230, 225, 0.08), // SK:251
		Focus:    rgba(232, 230, 225, 0.16), // rule: focus wash
	},
	Glass: Glass{
		Fill:          rgba(34, 30, 26, 0.80),    // SK:217 L2 fill, translucent
		Edge:          rgba(232, 230, 225, 0.14), // SK:176 specular lip
		Rim:           rgba(232, 230, 225, 0.18), // SK:217
		Scrim:         rgba(0, 0, 0, 0.45),       // rule: dark scrim
		Shadow:        rgba(0, 0, 0, 0.65),       // SK:217
		ShadowBlur:    36,                        // SK:217
		ShadowOffsetY: 16,                        // SK:217
		Blur:          20,                        // rule: glass setting
	},
	Status: Status{
		Success:  hex("#8a9a7b"), // SK:187 lichen sage
		Warning:  hex("#d19a66"), // SK:185 amber filament
		Error:    hex("#d4685a"), // SK:186 clay #c75a4a, lightened to 4.5:1 on Base
		Info:     hex("#b5b0a8"), // rule: Info = Text.Secondary
		OnStatus: hex("#141312"), // SK:174
	},
	Syntax: Syntax{
		Comment:   hex("#969088"), // SK:16
		Keyword:   hex("#e8e6e1"), // SK:180, bold in the editor
		Type:      hex("#cbc6bd"), // SK:24 secondary
		Function:  hex("#e4e2dd"), // SK:36 primary-fixed
		Variable:  hex("#ccc5bd"), // SK:13 on-surface-variant
		Constant:  hex("#ffdcbf"), // SK:44 tertiary-fixed
		String:    hex("#8a9a7b"), // SK:187 lichen sage
		Number:    hex("#f6bb84"), // SK:28 tertiary
		Operator:  hex("#b5b0a8"), // SK:181
		Attribute: hex("#c0bbb3"), // SK:27 on-secondary-container
		Error:     hex("#d4685a"), // = Status.Error
	},
}
