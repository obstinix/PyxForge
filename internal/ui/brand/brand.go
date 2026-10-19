// Package brand carries PyxForge's icon: the { ☼ } bracket-sun mark from the owner's logo
// concept, drawn in icon.svg and rendered by tools/appicon. The app shows it as its window
// and taskbar icon; the Windows executable and installer carry the same image as an .ico.
package brand

import (
	_ "embed"

	"fyne.io/fyne/v2"
)

//go:embed icon.png
var iconPNG []byte

// Icon is the application icon, 256 pixels square.
var Icon fyne.Resource = fyne.NewStaticResource("pyxforge.png", iconPNG)
