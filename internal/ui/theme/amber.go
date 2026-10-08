package theme

// Amber is the warm alternative accent (Q6). The dark set is the Smoked Kraft amber (SK:28,
// SK:185, container SK:30, 40–50 % borders as rendered in the charcoal concept); the light set
// is derived from it, deepened until white text sits on it at 4.5:1.
var Amber = Accent{
	ID:   "amber",
	Name: "Amber",
	Dark: AccentSet{
		Primary:   hex("#f6bb84"), // SK:28
		Hover:     hex("#ffcf9f"),
		Active:    hex("#d19a66"), // SK:185
		Muted:     rgba(246, 187, 132, 0.14),
		Focus:     hex("#f6bb84"),
		Selection: rgba(246, 187, 132, 0.24),
		Border:    rgba(246, 187, 132, 0.50),
		OnPrimary: hex("#2a1400"), // SK:30
	},
	Light: AccentSet{
		Primary:   hex("#9a5b13"),
		Hover:     hex("#a86518"),
		Active:    hex("#7a470c"),
		Muted:     rgba(154, 91, 19, 0.10),
		Focus:     hex("#9a5b13"),
		Selection: rgba(154, 91, 19, 0.18),
		Border:    rgba(154, 91, 19, 0.50),
		OnPrimary: hex("#ffffff"),
	},
}
