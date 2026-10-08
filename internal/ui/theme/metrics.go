package theme

// Metrics shared by every theme, in device-independent pixels. Sources are in
// DESIGN_SYSTEM.md; Section n refers to refactor-v2.md.

// Spacing on a 4 px grid (Section 11.4).
const (
	Space1 float32 = 4
	Space2 float32 = 8
	Space3 float32 = 12
	Space4 float32 = 16
	Space6 float32 = 24
)

// Layout sizes.
const (
	RailWidth        float32 = 48  // Section 11.6, VF:181
	ExplorerWidth    float32 = 256 // rendered width of every concept's sidebar
	InspectorWidth   float32 = 296 // Verdigris inspector at 1440 px, rounded to the grid
	DockHeight       float32 = 220
	TabBarHeight     float32 = 32
	StatusBarHeight  float32 = 24
	RowHeight        float32 = 24 // SK:251 tree rows
	PaletteRowHeight float32 = 36 // SK:240
	PaletteWidth     float32 = 640
	IconSize         float32 = 16 // Section 11.4
	IconSizeLarge    float32 = 20 // Section 11.4: rail icons
	FocusRingWidth   float32 = 2  // Section 11.2: one width everywhere
	SplitDragWidth   float32 = 6  // Section 11.4: 6–8 px drag zone around a 1 px rule
)

// Corner radii (Section 11.4).
const (
	RadiusControl float32 = 4
	RadiusPanel   float32 = 6
	RadiusOverlay float32 = 8
)

// Type scale (Section 11.3), with the 11 px floor from the Impeccable detector.
const (
	TextCaption float32 = 11
	TextBody    float32 = 13
	TextLarge   float32 = 14
	TextHeading float32 = 16
	TextCode    float32 = 13
)

// Motion: color and focus transitions only, never longer than this (Section 11.8).
const TransitionMillis = 120
