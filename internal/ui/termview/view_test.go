package termview

import (
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/test"

	"github.com/obstinix/PyxForge/internal/ui/theme"
	"github.com/obstinix/PyxForge/internal/vt"
)

// setup is a view showing a 2-column-wider-than-needed screen in a test window, its input
// collected.
func setup(t *testing.T) (*View, *vt.Screen, *[]string, fyne.App) {
	t.Helper()
	a := test.NewTempApp(t)
	v := New()
	var in []string
	v.Input = func(b []byte) { in = append(in, string(b)) }
	v.Dispatch = func(f func()) { f() }
	scr := vt.New(10, 4)
	v.OnResize = func(c, r int) { scr.Resize(c, r) }
	v.SetScreen(scr)
	w := test.NewTempWindow(t, v)
	cell := v.CellSize()
	w.Resize(fyne.NewSize(cell.Width*40+8, cell.Height*6+8))
	v.Resize(fyne.NewSize(cell.Width*40, cell.Height*6))
	return v, scr, &in, a
}

func texts(v *View) []*canvas.Text {
	var out []*canvas.Text
	for _, o := range test.WidgetRenderer(v).Objects() {
		if tx, ok := o.(*canvas.Text); ok {
			out = append(out, tx)
		}
	}
	return out
}

func TestResizeReportsCells(t *testing.T) {
	v, scr, _, _ := setup(t)
	if c, r := v.GridSize(); c != 40 || r != 6 {
		t.Errorf("grid %dx%d, want 40x6", c, r)
	}
	if c, r := scr.Size(); c != 40 || r != 6 {
		t.Errorf("the screen was resized to %dx%d", c, r)
	}
}

func TestKeysReachTheProgram(t *testing.T) {
	v, scr, in, _ := setup(t)
	took := ""
	v.ShellTakes = func(sc fyne.Shortcut) bool {
		cs, ok := sc.(*desktop.CustomShortcut)
		return ok && cs.KeyName == fyne.KeyP && cs.Modifier == fyne.KeyModifierControl|fyne.KeyModifierShift
	}
	v.OnShortcut = func(sc fyne.Shortcut) { took = sc.ShortcutName() }
	ctrl := fyne.KeyModifierControl
	v.TypedRune('a')
	v.TypedRune('é')
	v.TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})
	v.TypedKey(&fyne.KeyEvent{Name: fyne.KeyUp})
	v.TypedKey(&fyne.KeyEvent{Name: fyne.KeyF5})
	v.TypedShortcut(&fyne.ShortcutCopy{})                                        // no selection: interrupt
	v.TypedShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyD, Modifier: ctrl}) // EOF
	v.TypedShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyLeft, Modifier: ctrl})
	v.TypedShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyB, Modifier: fyne.KeyModifierAlt})
	v.TypedShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyP, Modifier: ctrl | fyne.KeyModifierShift})
	scr.Write([]byte("\x1b[?1h")) // application cursor keys
	v.TypedKey(&fyne.KeyEvent{Name: fyne.KeyUp})
	want := []string{"a", "é", "\r", "\x1b[A", "\x1b[15~", "\x03", "\x04", "\x1b[1;5D", "\x1bb", "\x1bOA"}
	if strings.Join(*in, "|") != strings.Join(want, "|") {
		t.Errorf("sent %q\n want %q", *in, want)
	}
	if took == "" {
		t.Error("the shell's chord reached the program")
	}
}

func TestDrawsColoursAndCursor(t *testing.T) {
	v, scr, _, _ := setup(t)
	scr.Write([]byte("plain \x1b[31mRED\x1b[0m"))
	v.FocusGained()
	v.Refresh()
	pal := Palette(theme.Current())
	var red, plain bool
	for _, tx := range texts(v) {
		switch tx.Text {
		case "RED":
			red = tx.Color == pal[1]
		case "plain":
			plain = true
		}
	}
	if !red || !plain {
		var got []string
		for _, tx := range texts(v) {
			got = append(got, tx.Text)
		}
		t.Errorf("drawn %q (red in the palette's red: %v)", got, red)
	}
}

func TestScrollbackAndSelection(t *testing.T) {
	v, scr, in, a := setup(t)
	for i := range 20 {
		scr.Write([]byte("line-" + string(rune('a'+i)) + "\r\n"))
	}
	v.Scrolled(&fyne.ScrollEvent{Scrolled: fyne.Delta{DY: v.CellSize().Height * 4}})
	if v.ScrolledBack() == 0 {
		t.Fatal("the wheel did not scroll back")
	}
	// More output while scrolled back keeps the same lines on view.
	before := v.ScrolledBack()
	scr.Write([]byte("more\r\n"))
	v.Refresh()
	if v.ScrolledBack() != before+1 {
		t.Errorf("scrolled back %d after a line of output, want %d", v.ScrolledBack(), before+1)
	}
	// A double click selects a word; Ctrl+Shift+C copies it.
	cell := v.CellSize()
	v.DoubleTapped(&fyne.PointEvent{Position: fyne.NewPos(cell.Width*2, cell.Height/2)})
	got := v.SelectedText()
	if !strings.HasPrefix(got, "line-") {
		t.Fatalf("double click selected %q", got)
	}
	v.TypedShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyC, Modifier: fyne.KeyModifierControl | fyne.KeyModifierShift})
	if c := a.Clipboard().Content(); c != got {
		t.Errorf("clipboard %q, want %q", c, got)
	}
	// Typing returns to the end.
	v.TypedRune('x')
	if v.ScrolledBack() != 0 || v.SelectedText() != "" || (*in)[len(*in)-1] != "x" {
		t.Errorf("after typing: back %d, selection %q", v.ScrolledBack(), v.SelectedText())
	}
}

func TestPasteAndMouse(t *testing.T) {
	v, scr, in, a := setup(t)
	a.Clipboard().SetContent("one\ntwo")
	scr.Write([]byte("\x1b[?2004h"))
	v.TypedShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyV, Modifier: fyne.KeyModifierControl | fyne.KeyModifierShift})
	if got := (*in)[len(*in)-1]; got != "\x1b[200~one\rtwo\x1b[201~" {
		t.Errorf("bracketed paste %q", got)
	}
	// A program that asked for the mouse gets clicks and the wheel, in the 1006 form.
	scr.Write([]byte("\x1b[?1000h\x1b[?1006h"))
	cell := v.CellSize()
	at := fyne.NewPos(cell.Width*3+1, cell.Height*2+1)
	v.MouseDown(&desktop.MouseEvent{PointEvent: fyne.PointEvent{Position: at}, Button: desktop.MouseButtonPrimary})
	v.MouseUp(&desktop.MouseEvent{PointEvent: fyne.PointEvent{Position: at}, Button: desktop.MouseButtonPrimary})
	v.Scrolled(&fyne.ScrollEvent{PointEvent: fyne.PointEvent{Position: at}, Scrolled: fyne.Delta{DY: 1}})
	got := strings.Join((*in)[len(*in)-3:], "|")
	if got != "\x1b[<0;4;3M|\x1b[<0;4;3m|\x1b[<64;4;3M" {
		t.Errorf("mouse reports %q", got)
	}
}

func TestKeyEncodings(t *testing.T) {
	for _, c := range []struct {
		k    fyne.KeyName
		m    fyne.KeyModifier
		want string
	}{
		{fyne.KeyTab, fyne.KeyModifierShift, "\x1b[Z"},
		{fyne.KeyDelete, 0, "\x1b[3~"},
		{fyne.KeyPageUp, fyne.KeyModifierControl, "\x1b[5;5~"},
		{fyne.KeyF1, 0, "\x1bOP"},
		{fyne.KeyF1, fyne.KeyModifierShift, "\x1b[1;2P"},
		{fyne.KeyBackspace, 0, "\x7f"},
		{fyne.KeyBackspace, fyne.KeyModifierControl, "\x08"},
		{fyne.KeyHome, 0, "\x1b[H"},
	} {
		if got := string(keyBytes(c.k, c.m, vt.Modes{})); got != c.want {
			t.Errorf("%s with %v: %q, want %q", c.k, c.m, got, c.want)
		}
	}
	if b, _ := ctrlBytes(fyne.KeyLeftBracket, false); string(b) != "\x1b" {
		t.Error("Ctrl+[ is not ESC")
	}
	if b, _ := ctrlBytes(fyne.KeySpace, true); string(b) != "\x1b\x00" {
		t.Error("Ctrl+Alt+Space")
	}
	if got := xterm256(196); got.R != 255 || got.G != 0 || got.B != 0 {
		t.Errorf("colour 196 is %v", got)
	}
	if got := xterm256(244); got.R != 128 {
		t.Errorf("grey 244 is %v", got)
	}
}
