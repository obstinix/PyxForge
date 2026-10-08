package theme

import (
	_ "embed"

	"fyne.io/fyne/v2"
)

// Fonts are bundled, never fetched (Section 11.3, Section 12). Licenses sit next to the files
// and are listed in docs/design/FONT_LICENSES.md.

//go:embed fonts/Geist-Regular.ttf
var geistRegular []byte

//go:embed fonts/Geist-Medium.ttf
var geistMedium []byte

//go:embed fonts/Geist-SemiBold.ttf
var geistSemiBold []byte

//go:embed fonts/JetBrainsMono-Regular.ttf
var monoRegular []byte

//go:embed fonts/JetBrainsMono-Bold.ttf
var monoBold []byte

//go:embed fonts/JetBrainsMono-Italic.ttf
var monoItalic []byte

//go:embed fonts/Syne-SemiBold.ttf
var syneSemiBold []byte

// Font resources. Fyne's theme maps text styles to the UI and mono families; the display face
// and the medium weight are applied per text object through canvas.Text.FontSource.
var (
	FontUI         = fyne.NewStaticResource("Geist-Regular.ttf", geistRegular)
	FontUIMedium   = fyne.NewStaticResource("Geist-Medium.ttf", geistMedium)
	FontUIStrong   = fyne.NewStaticResource("Geist-SemiBold.ttf", geistSemiBold)
	FontMono       = fyne.NewStaticResource("JetBrainsMono-Regular.ttf", monoRegular)
	FontMonoBold   = fyne.NewStaticResource("JetBrainsMono-Bold.ttf", monoBold)
	FontMonoItalic = fyne.NewStaticResource("JetBrainsMono-Italic.ttf", monoItalic)
	FontDisplay    = fyne.NewStaticResource("Syne-SemiBold.ttf", syneSemiBold)
)
