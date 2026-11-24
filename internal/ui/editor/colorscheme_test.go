package editor

import (
	"testing"

	"github.com/obstinix/PyxForge/internal/ui/theme"
)

func TestColorschemeCoversEveryTheme(t *testing.T) {
	for _, p := range theme.Palettes {
		for _, a := range theme.Accents {
			tk := theme.Resolve(p, a)
			bg, groups := Colorscheme(tk)
			if want := map[theme.Polarity]string{theme.Light: "light"}[p.Polarity]; want != "" && bg != want {
				t.Errorf("%s: background %q", p.ID, bg)
			}
			if groups["Normal"]["bg"] != hexOf(tk.Surface.Base) {
				t.Errorf("%s: Normal background %v", p.ID, groups["Normal"]["bg"])
			}
		}
	}
}
