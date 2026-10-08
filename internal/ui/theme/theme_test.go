package theme

import (
	"fmt"
	"image/color"
	"testing"

	fynetheme "fyne.io/fyne/v2/theme"
)

// WCAG 2.x thresholds (Section 16.3).
const (
	textAA = 4.5
	uiAA   = 3.0
)

func everyCombination() []Tokens {
	var out []Tokens
	for _, p := range Palettes {
		for _, a := range Accents {
			out = append(out, Resolve(p, a))
		}
	}
	return out
}

func TestRegistryIsConsistent(t *testing.T) {
	if len(Palettes) != 5 || len(Accents) != 2 {
		t.Fatalf("D2 requires 5 themes and at least 2 accents, have %d and %d", len(Palettes), len(Accents))
	}
	seen := map[string]bool{}
	for _, p := range Palettes {
		if p.ID == "" || p.ID == SystemID || seen[p.ID] {
			t.Errorf("palette %q: empty, reserved or duplicate ID", p.Name)
		}
		seen[p.ID] = true
		if got, ok := PaletteByID(p.ID); !ok || got.Name != p.Name {
			t.Errorf("PaletteByID(%q) did not round-trip", p.ID)
		}
	}
	for _, a := range Accents {
		if got, ok := AccentByID(a.ID); !ok || got.Name != a.Name {
			t.Errorf("AccentByID(%q) did not round-trip", a.ID)
		}
	}
}

func TestSystemModeFollowsAppearance(t *testing.T) {
	// Q5: OS dark → Smoked Kraft, OS light → Ink & Paper.
	sys := Selection{PaletteID: SystemID, AccentID: Amber.ID}
	if got := sys.Tokens(fynetheme.VariantDark); got.ID != SmokedKraft.ID || got.AccentID != Amber.ID {
		t.Errorf("system/dark resolved to %s/%s", got.ID, got.AccentID)
	}
	if got := sys.Tokens(fynetheme.VariantLight); got.ID != InkPaper.ID {
		t.Errorf("system/light resolved to %s", got.ID)
	}
	// A chosen theme ignores the OS.
	fixed := Selection{PaletteID: Monochrome.ID, AccentID: Crimson.ID}
	if got := fixed.Tokens(fynetheme.VariantLight); got.ID != Monochrome.ID {
		t.Errorf("fixed selection followed the OS: %s", got.ID)
	}
	// Stale preferences fall back instead of failing.
	stale := Selection{PaletteID: "retired-theme", AccentID: "retired-accent"}
	if got := stale.Tokens(fynetheme.VariantDark); got.ID != SmokedKraft.ID || got.AccentID != Crimson.ID {
		t.Errorf("stale selection resolved to %s/%s", got.ID, got.AccentID)
	}
}

func TestAccentFollowsPolarity(t *testing.T) {
	if Resolve(InkPaper, Crimson).Accent != Crimson.Light {
		t.Error("light palette did not get the light accent set")
	}
	if Resolve(VerdigrisForge, Crimson).Accent != Crimson.Dark {
		t.Error("dark palette did not get the dark accent set")
	}
}

func TestContrast(t *testing.T) {
	for _, tk := range everyCombination() {
		name := fmt.Sprintf("%s+%s", tk.ID, tk.AccentID)
		s := tk.Surface
		overlay := Over(tk.Glass.Fill, s.Base) // what an overlay looks like over the workspace

		check := func(what string, fg, bg color.NRGBA, min float64) {
			t.Helper()
			if r := Contrast(fg, bg); r < min {
				t.Errorf("%s: %s is %.2f:1, needs %.1f:1", name, what, r, min)
			}
		}

		for _, bg := range []struct {
			name string
			c    color.NRGBA
		}{{"base", s.Base}, {"raised", s.Raised}, {"sunken", s.Sunken}, {"overlay", overlay}} {
			check("text.primary on "+bg.name, tk.Text.Primary, bg.c, textAA)
			check("text.secondary on "+bg.name, tk.Text.Secondary, bg.c, textAA)
			check("text.tertiary on "+bg.name, tk.Text.Tertiary, bg.c, textAA)
		}
		if Contrast(tk.Text.Disabled, s.Base) >= Contrast(tk.Text.Tertiary, s.Base) {
			t.Errorf("%s: disabled text is not weaker than tertiary text", name)
		}

		for what, c := range map[string]color.NRGBA{
			"success": tk.Status.Success, "warning": tk.Status.Warning,
			"error": tk.Status.Error, "info": tk.Status.Info,
		} {
			check("status."+what+" on base", c, s.Base, textAA)
			check("on-status text on "+what, tk.Status.OnStatus, c, textAA)
		}

		sy := tk.Syntax
		for what, c := range map[string]color.NRGBA{
			"comment": sy.Comment, "keyword": sy.Keyword, "type": sy.Type, "function": sy.Function,
			"variable": sy.Variable, "constant": sy.Constant, "string": sy.String, "number": sy.Number,
			"operator": sy.Operator, "attribute": sy.Attribute, "error": sy.Error,
		} {
			check("syntax."+what+" on base", c, s.Base, textAA)
			check("syntax."+what+" on sunken", c, s.Sunken, textAA)
		}

		ac := tk.Accent
		check("accent.primary text on base", ac.Primary, s.Base, textAA)
		check("accent.primary on raised", ac.Primary, s.Raised, uiAA)
		check("accent.onPrimary on primary", ac.OnPrimary, ac.Primary, textAA)
		check("accent.onPrimary on hover", ac.OnPrimary, ac.Hover, textAA)
		check("focus ring on base", ac.Focus, s.Base, uiAA)
		check("focus ring on raised", ac.Focus, s.Raised, uiAA)
		check("focus ring on overlay", ac.Focus, overlay, uiAA)
		check("text.primary on selection", tk.Text.Primary, Over(ac.Selection, s.Base), textAA)
		check("text.primary on selected row", tk.Text.Primary, Over(tk.State.Selected, s.Base), textAA)
		check("text.primary on pressed row", tk.Text.Primary, Over(tk.State.Pressed, s.Base), textAA)
	}
}

func TestContrastMath(t *testing.T) {
	cases := []struct {
		fg, bg string
		want   float64
	}{
		{"#000000", "#ffffff", 21},
		{"#ffffff", "#ffffff", 1},
		{"#777777", "#ffffff", 4.48},   // the classic "just fails AA" grey
		{"#ffffff80", "#000000", 5.32}, // 50 % white composited on black is #808080
	}
	for _, c := range cases {
		if got := Contrast(hex(c.fg), hex(c.bg)); got < c.want-0.01 || got > c.want+0.01 {
			t.Errorf("Contrast(%s, %s) = %.3f, want %.2f", c.fg, c.bg, got, c.want)
		}
	}
}

// TestContrastReport logs the tightest ratio in each combination (go test -v) so the numbers in
// DESIGN_SYSTEM.md can be regenerated rather than retyped.
func TestContrastReport(t *testing.T) {
	for _, tk := range everyCombination() {
		s := tk.Surface
		t.Logf("%-16s %-8s tertiary/base %.2f  tertiary/sunken %.2f  error/base %.2f  accent/base %.2f  onAccent %.2f  focus/raised %.2f",
			tk.ID, tk.AccentID,
			Contrast(tk.Text.Tertiary, s.Base), Contrast(tk.Text.Tertiary, s.Sunken),
			Contrast(tk.Status.Error, s.Base), Contrast(tk.Accent.Primary, s.Base),
			Contrast(tk.Accent.OnPrimary, tk.Accent.Primary), Contrast(tk.Accent.Focus, s.Raised))
	}
}

func TestGlassFollowsD3(t *testing.T) {
	for _, p := range Palettes {
		if p.Glass.Blur != 0 {
			t.Errorf("%s: backdrop blur %.1f, but D3/Q2 say no blur", p.Name, p.Glass.Blur)
		}
		if a := p.Glass.Fill.A; a != 0xff && (a < 235 || a > 245) {
			t.Errorf("%s: glass fill alpha %d is outside D3's 92–96 %%", p.Name, a)
		}
	}
}
