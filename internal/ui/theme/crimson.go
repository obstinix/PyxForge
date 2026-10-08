package theme

// Crimson is PyxForge's signature accent. No reference concept contains a crimson, so both
// sets are derived (Q6): the dark set is lifted until it reads as text on every dark Base
// (4.5:1), the light set is deepened until white text sits on it at 4.5:1. TestContrast pins it.
var Crimson = Accent{
	ID:   "crimson",
	Name: "Crimson",
	Dark: AccentSet{
		Primary:   hex("#e0566a"),
		Hover:     hex("#e86f80"),
		Active:    hex("#c94458"),
		Muted:     rgba(224, 86, 106, 0.16),
		Focus:     hex("#e0566a"),
		Selection: rgba(224, 86, 106, 0.30),
		Border:    rgba(224, 86, 106, 0.55),
		OnPrimary: hex("#1a0a0d"),
	},
	Light: AccentSet{
		Primary:   hex("#b0243a"),
		Hover:     hex("#c02f46"),
		Active:    hex("#8f1c2f"),
		Muted:     rgba(176, 36, 58, 0.10),
		Focus:     hex("#b0243a"),
		Selection: rgba(176, 36, 58, 0.18),
		Border:    rgba(176, 36, 58, 0.50),
		OnPrimary: hex("#ffffff"),
	},
}
