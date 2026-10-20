package shell

import (
	"testing"

	"fyne.io/fyne/v2"
)

// TestFitSize: a window stays inside the usable screen area with room for its frame; a size
// that fits is kept.
func TestFitSize(t *testing.T) {
	for _, tc := range []struct {
		name         string
		size         fyne.Size
		workW, workH int
		scale        float32
		want         fyne.Size
	}{
		{"fits at 160%", fyne.NewSize(1440, 900), 2560, 1528, 1.6, fyne.NewSize(1440, 900)},
		{"1080p at 150%", fyne.NewSize(1440, 900), 1920, 1032, 1.5, fyne.NewSize(1264, 640)},
		{"small laptop at 100%", fyne.NewSize(1440, 900), 1366, 728, 1, fyne.NewSize(1350, 680)},
		{"saved on a bigger screen", fyne.NewSize(2400, 1400), 2560, 1528, 1.6, fyne.NewSize(1584, 907)},
		{"unknown screen", fyne.NewSize(1440, 900), 0, 0, 0, fyne.NewSize(1440, 900)},
	} {
		if got := fitSize(tc.size, tc.workW, tc.workH, tc.scale); got != tc.want {
			t.Errorf("%s: %v, want %v", tc.name, got, tc.want)
		}
	}
}
