package editor

import (
	"regexp"
	"testing"

	"github.com/obstinix/PyxForge/internal/ui/theme"
)

var hexColor = regexp.MustCompile(`^#[0-9a-f]{6}$`)

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
			c := TerminalColors(tk)
			for i, h := range c {
				if !hexColor.MatchString(h) {
					t.Errorf("%s/%s: terminal colour %d = %q", p.ID, a.ID, i, h)
				}
			}
			if c[0] == c[15] || c[1] == c[2] {
				t.Errorf("%s/%s: terminal colours collide: %v", p.ID, a.ID, c)
			}
		}
	}
}
