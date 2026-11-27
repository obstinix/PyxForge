package kit

import (
	"fmt"
	"image/color"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/test"
	"github.com/obstinix/PyxForge/internal/ui/icons"
	"github.com/obstinix/PyxForge/internal/ui/theme"
)

// every theme × accent combination, resolved.
func everyTokens() []theme.Tokens {
	var out []theme.Tokens
	for _, p := range theme.Palettes {
		for _, a := range theme.Accents {
			out = append(out, theme.Resolve(p, a))
		}
	}
	return out
}

// TestComponentContrast: what each component draws meets WCAG in every theme and accent:
// button labels 4.5:1 on their fill (the page under a ghost button) in every state but
// disabled, a switch's knob and track 3:1 against what is around them.
func TestComponentContrast(t *testing.T) {
	const text, ui = 4.5, 3.0
	for _, tk := range everyTokens() {
		name := tk.ID + "+" + tk.AccentID
		for _, v := range []struct {
			name string
			v    Variant
		}{{"default", ButtonDefault}, {"cta", ButtonCTA}, {"ghost", ButtonGhost}, {"danger", ButtonDanger}, {"icon", ButtonIcon}} {
			for _, st := range []struct {
				name string
				s    ButtonState
			}{{"normal", StateNormal}, {"hover", StateHover}, {"pressed", StatePressed}} {
				fill, fg, _ := ButtonColors(tk, v.v, st.s)
				for _, page := range []struct {
					name string
					c    color.NRGBA
				}{{"base", tk.Surface.Base}, {"raised", tk.Surface.Raised}, {"overlay", tk.Surface.Overlay}} {
					bg := theme.Over(fill, page.c)
					if r := theme.Contrast(fg, bg); r < text {
						t.Errorf("%s: %s button %s on %s: %.2f:1", name, v.name, st.name, page.name, r)
					}
				}
			}
			// Disabled is exempt from WCAG but must look weaker than enabled.
			_, on, _ := ButtonColors(tk, v.v, StateNormal)
			_, off, _ := ButtonColors(tk, v.v, StateDisabled)
			if off.A >= on.A {
				t.Errorf("%s: a disabled %s button is not faded", name, v.name)
			}
		}
		for _, on := range []bool{false, true} {
			track, _, knob := SwitchColors(tk, on, false)
			if r := theme.Contrast(knob, theme.Over(track, tk.Surface.Raised)); r < ui {
				t.Errorf("%s: switch (on %v) knob on track: %.2f:1", name, on, r)
			}
		}
		onTrack, _, _ := SwitchColors(tk, true, false)
		if r := theme.Contrast(onTrack, tk.Surface.Raised); r < ui {
			t.Errorf("%s: an on switch's track on raised: %.2f:1", name, r)
		}
		_, offOutline, _ := SwitchColors(tk, false, false)
		if r := theme.Contrast(theme.Over(offOutline, tk.Surface.Sunken), tk.Surface.Raised); r < 1.2 {
			t.Errorf("%s: an off switch's outline cannot be told from the card: %.2f:1", name, r)
		}
	}
}

func TestButton(t *testing.T) {
	pyxApp(t)
	n := 0
	b := NewButton("Build", func() { n++ })
	w := test.NewTempWindow(t, b)
	w.Resize(fyne.NewSize(200, 60))
	test.Tap(b)
	b.TypedRune(' ')
	b.TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})
	if n != 3 {
		t.Fatalf("a click, Space and Return ran it %d times", n)
	}
	b.Disable()
	test.Tap(b)
	b.TypedKey(&fyne.KeyEvent{Name: fyne.KeyEnter})
	if n != 3 || b.AccessibilityLabel() != "Build (unavailable)" {
		t.Fatalf("a disabled button ran: %d, %q", n, b.AccessibilityLabel())
	}
	b.Enable()

	r := test.WidgetRenderer(b).(*buttonRenderer)
	if !r.ring.Hidden {
		t.Error("focus ring before focus")
	}
	w.Canvas().Focus(b)
	if r.ring.Hidden {
		t.Error("no focus ring with keyboard focus")
	}
	if h := b.MinSize().Height; h != ButtonHeight {
		t.Errorf("height %v", h)
	}
	b.Compact = true
	b.Refresh()
	if h := b.MinSize().Height; h != ButtonHeightCompact {
		t.Errorf("compact height %v", h)
	}
	icon := NewButton("Close", nil)
	icon.Variant, icon.Icon = ButtonIcon, "x"
	if s := icon.MinSize(); s.Width != s.Height {
		t.Errorf("an icon button is %v, not square", s)
	}
	b.MouseIn(&desktop.MouseEvent{})
	if b.state() != StateHover {
		t.Error("hover state")
	}
	b.MouseDown(&desktop.MouseEvent{})
	if b.state() != StatePressed {
		t.Error("pressed state")
	}
}

func TestSwitch(t *testing.T) {
	pyxApp(t)
	old := ReduceMotion
	t.Cleanup(func() { ReduceMotion = old })
	ReduceMotion = func() bool { return true }
	var got []bool
	s := NewSwitch(false, func(on bool) { got = append(got, on) })
	s.Label = "Command blocks"
	test.NewTempWindow(t, s)
	test.Tap(s)
	if !s.On || s.KnobPosition() != 1 {
		t.Fatalf("tap: on %v, knob %v (reduce motion: it jumps)", s.On, s.KnobPosition())
	}
	s.TypedRune(' ')
	s.TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})
	if fmt.Sprint(got) != "[true false true]" || s.AccessibilityLabel() != "Command blocks (on)" {
		t.Fatalf("changes %v, label %q", got, s.AccessibilityLabel())
	}
	s.SetOn(false)
	if len(got) != 3 || s.KnobPosition() != 0 {
		t.Error("SetOn reported a change or left the knob")
	}
	s.Disable()
	test.Tap(s)
	if s.On {
		t.Error("a disabled switch toggled")
	}
}

func TestSettingRowsAndGroups(t *testing.T) {
	pyxApp(t)
	sw := NewSwitch(true, nil)
	row := NewSettingRow("Command blocks", "Marks each command with its exit status and duration.", sw).
		WithKeywords("osc 133", "shell integration")
	for q, want := range map[string]bool{"blocks": true, "EXIT status": true, "shell integration": true,
		"osc": true, "blocks git": false, "": false} {
		if row.Matches(q) != want {
			t.Errorf("Matches(%q) = %v", q, !want)
		}
	}
	other := NewSettingRow("Shell", "", NewDropdown([]string{"pwsh", "cmd.exe"}, "pwsh", nil))
	g := NewSettingGroup("Terminal", row, other)
	w := test.NewTempWindow(t, g)
	w.Resize(fyne.NewSize(600, 300))
	// The control sits at the right edge of the row, at the top of its text.
	ctlAt := fyne.CurrentApp().Driver().AbsolutePositionForObject(sw)
	rowAt := fyne.CurrentApp().Driver().AbsolutePositionForObject(row)
	if ctlAt.X+sw.Size().Width < rowAt.X+row.Size().Width-theme.Space4-1 || ctlAt.Y-rowAt.Y != theme.Space4 {
		t.Errorf("control at %v in a row at %v (size %v)", ctlAt, rowAt, row.Size())
	}
	// A row with a long description grows taller than one without.
	if row.Size().Height <= other.Size().Height {
		t.Errorf("row heights %v and %v", row.Size().Height, other.Size().Height)
	}
}

func TestSearchField(t *testing.T) {
	pyxApp(t)
	var q []string
	escaped := 0
	f := NewSearchField("Search settings…", func(s string) { q = append(q, s) })
	f.OnEscapeEmpty = func() { escaped++ }
	w := test.NewTempWindow(t, f)
	w.Resize(fyne.NewSize(300, 40))
	if f.ClearButton().Visible() || f.Icon == nil {
		t.Fatal("empty field: clear button shown or no magnifier")
	}
	test.Type(f, "font")
	if !f.ClearButton().Visible() || f.Text != "font" {
		t.Fatalf("after typing: %q, clear visible %v", f.Text, f.ClearButton().Visible())
	}
	f.TypedKey(&fyne.KeyEvent{Name: fyne.KeyEscape})
	if f.Text != "" || f.ClearButton().Visible() || escaped != 0 {
		t.Fatalf("Esc with text: %q, escaped %d", f.Text, escaped)
	}
	f.TypedKey(&fyne.KeyEvent{Name: fyne.KeyEscape})
	if escaped != 1 {
		t.Error("Esc on an empty field was not reported")
	}
	test.Type(f, "x")
	test.Tap(f.ClearButton())
	if f.Text != "" || q[len(q)-1] != "" {
		t.Errorf("the clear button left %q", f.Text)
	}
}

func TestNavList(t *testing.T) {
	pyxApp(t)
	var chosen []string
	l := NewNavList([]NavItem{{Label: "Options", Heading: true}, {ID: "general", Label: "General", Icon: "settings"},
		{ID: "editor", Label: "Editor", Icon: "file-code"}, {Label: "Core features", Heading: true},
		{ID: "graph", Label: "Graph", Icon: "list-tree"}}, func(id string) { chosen = append(chosen, id) })
	w := test.NewTempWindow(t, l)
	w.Resize(fyne.NewSize(240, 200))
	if l.Selected() != "general" {
		t.Fatalf("first selection %q", l.Selected())
	}
	w.Canvas().Focus(l)
	l.TypedKey(&fyne.KeyEvent{Name: fyne.KeyDown})
	l.TypedKey(&fyne.KeyEvent{Name: fyne.KeyDown}) // over the heading
	if l.Cursor() != "graph" || l.Selected() != "general" {
		t.Fatalf("cursor %q, selected %q", l.Cursor(), l.Selected())
	}
	l.TypedKey(&fyne.KeyEvent{Name: fyne.KeyDown}) // the end: stays
	l.TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})
	test.TapAt(l, fyne.NewPos(20, navRowHeight*2+5)) // Editor
	test.TapAt(l, fyne.NewPos(20, 5))                // a heading: nothing
	if fmt.Sprint(chosen) != "[graph editor]" || l.Selected() != "editor" {
		t.Errorf("chosen %v, selected %q", chosen, l.Selected())
	}
}

func TestDropdown(t *testing.T) {
	pyxApp(t)
	var got []string
	d := NewDropdown([]string{"Flat", "Liquid Glass"}, "Flat", func(s string) { got = append(got, s) })
	w := test.NewTempWindow(t, d)
	w.Resize(fyne.NewSize(300, 300))
	d.TypedKey(&fyne.KeyEvent{Name: fyne.KeyDown})
	d.TypedKey(&fyne.KeyEvent{Name: fyne.KeyDown}) // the last: stays
	if d.Selected != "Liquid Glass" || fmt.Sprint(got) != "[Liquid Glass]" {
		t.Fatalf("Down: %q, changes %v", d.Selected, got)
	}
	m := d.Menu()
	if !m.Items[1].Checked || m.Items[0].Checked {
		t.Error("the menu does not check the selection")
	}
	m.Items[0].Action()
	if d.Selected != "Flat" {
		t.Errorf("choosing from the menu: %q", d.Selected)
	}
	test.Tap(d) // opens the list over the window
	if w.Canvas().Overlays().Top() == nil {
		t.Error("a click did not open the list")
	}
	if wd := d.MinSize().Width; wd < fyne.MeasureText("Liquid Glass", theme.TextBody, fyne.TextStyle{}).Width {
		t.Errorf("width %v does not fit the longest option", wd)
	}
}

func TestSlider(t *testing.T) {
	pyxApp(t)
	var got []float64
	s := NewSlider("Link length", 10, 300, 5, 195, func(v float64) { got = append(got, v) })
	w := test.NewTempWindow(t, s)
	w.Resize(fyne.NewSize(240, 60))
	s.TypedKey(&fyne.KeyEvent{Name: fyne.KeyRight})
	s.TypedShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyLeft, Modifier: fyne.KeyModifierShift})
	if fmt.Sprint(got) != "[200 150]" {
		t.Fatalf("Right then Shift+Left: %v", got)
	}
	s.TypedKey(&fyne.KeyEvent{Name: fyne.KeyEnd})
	s.TypedKey(&fyne.KeyEvent{Name: fyne.KeyRight}) // already at the end
	if s.Value != 300 || len(got) != 3 {
		t.Fatalf("End: %v, changes %v", s.Value, got)
	}
	test.TapAt(s, fyne.NewPos(sliderKnob/2, s.trackY())) // the start of the track
	if s.Value != 10 {
		t.Errorf("a click at the start set %v", s.Value)
	}
	f := NewSlider("Opacity", 0, 1, 0.01, 0.06, nil)
	if f.Text() != "0.06" {
		t.Errorf("shown as %q", f.Text())
	}
}

// pyxApp is a test app drawn with PyxForge's theme, so components read the real tokens.
func pyxApp(t *testing.T) fyne.App {
	a := test.NewTempApp(t)
	a.Settings().SetTheme(theme.NewFyne(theme.Default))
	return a
}

// TestButtonIconFollowsItsLabel: a button's icon is drawn in its label's colour, faded with it
// when disabled; Fyne's own recolouring changed only an SVG's fill, and these icons are strokes
// (#50).
func TestButtonIconFollowsItsLabel(t *testing.T) {
	pyxApp(t)
	b := NewButtonWithIcon("Commit", icons.GitCommit, func() {})
	b.Variant = ButtonCTA
	test.NewTempWindow(t, b)
	r := test.WidgetRenderer(b).(*buttonRenderer)
	tk := theme.Current()
	_, on, _ := ButtonColors(tk, ButtonCTA, StateNormal)
	if r.icon.Resource != icons.Get(icons.GitCommit, on) {
		t.Errorf("enabled icon %s", r.icon.Resource.Name())
	}
	b.Disable()
	_, off, _ := ButtonColors(tk, ButtonCTA, StateDisabled)
	if r.icon.Resource != icons.Get(icons.GitCommit, off) || r.label.Color != off {
		t.Errorf("disabled icon %s, label %v", r.icon.Resource.Name(), r.label.Color)
	}
}
