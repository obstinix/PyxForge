package vt

import (
	"strings"
	"testing"
)

func feed(s *Screen, parts ...string) {
	for _, p := range parts {
		s.Write([]byte(p))
	}
}

// rows is the screen's rows as text, trailing blanks dropped.
func rows(s *Screen) []string {
	v := s.View(0)
	out := make([]string, len(v.Lines))
	for i, l := range v.Lines {
		out[i] = cellText(l.Cells, true)
	}
	return out
}

func cell(s *Screen, x, y int) Cell { return s.View(0).Lines[y].Cells[x] }

func wantRows(t *testing.T, s *Screen, want ...string) {
	t.Helper()
	got := rows(s)
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("screen\n got %q\nwant %q", got, want)
	}
}

func TestTextWrapAndCarriageReturn(t *testing.T) {
	s := New(5, 3)
	feed(s, "abcdefg\r\n", "50%\rdone")
	wantRows(t, s, "abcde", "fg", "done")
	if got := s.Lines(); strings.Join(got, "|") != "abcdefg|done" {
		t.Errorf("Lines %q: a wrapped line is one line", got)
	}
	// Writing the last column leaves the cursor there until the next character.
	s = New(3, 2)
	feed(s, "abc")
	if v := s.View(0); v.CursorX != 2 || v.CursorY != 0 {
		t.Errorf("cursor at %d,%d after filling a line", v.CursorX, v.CursorY)
	}
	feed(s, "\r\nx")
	wantRows(t, s, "abc", "x")
}

func TestScrollbackHistory(t *testing.T) {
	s := New(10, 3)
	for i := range 6 {
		feed(s, "line", string(rune('0'+i)), "\r\n")
	}
	wantRows(t, s, "line4", "line5", "")
	v := s.View(2)
	if v.Back != 2 || cellText(v.Lines[0].Cells, true) != "line2" || v.History != 4 {
		t.Errorf("back 2: %+v", v)
	}
	if v.CursorVisible {
		t.Error("the cursor shows while scrolled back past it")
	}
	if v := s.View(99); v.Back != 4 {
		t.Errorf("scrolling past the history: back %d", v.Back)
	}
	s.SetMaxHistory(2)
	if got := s.Lines(); strings.Join(got, "|") != "line2|line3|line4|line5" {
		t.Errorf("after trimming the history: %q", got)
	}
	// Absolute positions keep their lines as the history is trimmed.
	if got := s.Text(Pos{Line: 2, Col: 0}, Pos{Line: 3, Col: 5}); got != "line2\nline3" {
		t.Errorf("Text %q", got)
	}
	s.ClearHistory()
	if got := s.Lines(); strings.Join(got, "|") != "line4|line5" {
		t.Errorf("after ClearHistory: %q", got)
	}
}

func TestCursorAndErase(t *testing.T) {
	s := New(6, 3)
	feed(s, "aaaaaa", "bbbbbb", "cccccc")
	feed(s, "\x1b[2;3H\x1b[K") // erase from the cursor to the line's end
	wantRows(t, s, "aaaaaa", "bb", "cccccc")
	feed(s, "\x1b[1;4H\x1b[1K") // from the start to the cursor
	wantRows(t, s, "    aa", "bb", "cccccc")
	feed(s, "\x1b[3;2H\x1b[J")
	wantRows(t, s, "    aa", "bb", "c")
	feed(s, "\x1b[2J")
	wantRows(t, s, "", "", "")
	feed(s, "\x1b[2;2Hx\x1b[Ay\x1b[3Cz\x1b[10Dw")
	wantRows(t, s, "w y  z", " x", "")
	feed(s, "\x1b[3;6Hq\x1b[G<")
	wantRows(t, s, "w y  z", " x", "<    q")
}

func TestColoursAndAttributes(t *testing.T) {
	s := New(20, 2)
	feed(s, "\x1b[1;31ma\x1b[0mb\x1b[38;5;200mc\x1b[48;2;1;2;3md\x1b[38:2::10:20:30me\x1b[7;4mf\x1b[27;24;39;49mg\x1b[92mh")
	check := func(x int, fg, bg Color, a Attr) {
		t.Helper()
		if c := cell(s, x, 0); c.FG != fg || c.BG != bg || c.A != a {
			t.Errorf("cell %d (%q): fg %x bg %x attrs %b, want %x %x %b", x, c.R, c.FG, c.BG, c.A, fg, bg, a)
		}
	}
	check(0, Indexed(1), Default, Bold)
	check(1, Default, Default, 0)
	check(2, Indexed(200), Default, 0)
	check(3, Indexed(200), RGB(1, 2, 3), 0)
	check(4, RGB(10, 20, 30), RGB(1, 2, 3), 0)
	check(5, RGB(10, 20, 30), RGB(1, 2, 3), Inverse|Underline)
	check(6, Default, Default, 0)
	check(7, Indexed(10), Default, 0)
	if i, ok := Indexed(200).Index(); !ok || i != 200 {
		t.Error("Index")
	}
	if r, g, b, ok := RGB(1, 2, 3).RGB(); !ok || r != 1 || g != 2 || b != 3 {
		t.Error("RGB")
	}
	// Erasing fills with the pen's background.
	feed(s, "\r\n\x1b[44m\x1b[K")
	if c := cell(s, 10, 1); c.BG != Indexed(4) {
		t.Errorf("erased cell's background %x", c.BG)
	}
}

func TestAlternateScreen(t *testing.T) {
	s := New(8, 3)
	feed(s, "shell$ ", "\x1b[?1049h\x1b[H", "FULL")
	wantRows(t, s, "FULL", "", "")
	if !s.Modes().AltScreen {
		t.Error("not on the alternate screen")
	}
	for range 5 {
		feed(s, "\r\nx")
	}
	if v := s.View(3); v.Back != 0 {
		t.Error("the alternate screen scrolled into the history")
	}
	feed(s, "\x1b[?1049l")
	wantRows(t, s, "shell$", "", "")
	if v := s.View(0); v.CursorX != 7 || v.CursorY != 0 {
		t.Errorf("cursor not restored: %d,%d", v.CursorX, v.CursorY)
	}
}

func TestReplies(t *testing.T) {
	s := New(10, 5)
	var got []string
	s.SetReply(func(b []byte) { got = append(got, string(b)) })
	feed(s, "\x1b[3;4H\x1b[6n\x1b[5n\x1b[c\x1b[>c")
	want := []string{"\x1b[3;4R", "\x1b[0n", "\x1b[?62;22c", "\x1b[>0;10;1c"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("replies %q, want %q", got, want)
	}
}

func TestSplitUTF8AndWideCharacters(t *testing.T) {
	s := New(6, 2)
	b := []byte("é漢x")
	s.Write(b[:1])
	s.Write(b[1:3])
	s.Write(b[3:])
	wantRows(t, s, "é漢x", "")
	if c := cell(s, 1, 0); c.Width != 2 || c.R != '漢' {
		t.Errorf("wide cell %+v", c)
	}
	// A wide character that does not fit in the last column goes to the next line.
	s = New(4, 2)
	feed(s, "abc漢")
	wantRows(t, s, "abc", "漢")
	// Overwriting half of one clears the other half.
	s = New(4, 1)
	feed(s, "漢\x1b[2Gz")
	wantRows(t, s, " z")
}

func TestLineDrawingAndTitle(t *testing.T) {
	s := New(6, 1)
	feed(s, "\x1b(0lqk\x1b(Bq", "\x1b]0;my title\x07", "\x1b]2;other\x1b\\")
	wantRows(t, s, "┌─┐q")
	if s.Title() != "other" {
		t.Errorf("title %q", s.Title())
	}
	feed(s, "\x1bP1$r ignored \x1b\\!") // a DCS string is skipped
	wantRows(t, s, "┌─┐q!")
}

func TestScrollRegionInsertDelete(t *testing.T) {
	s := New(4, 4)
	feed(s, "a\r\nb\r\nc\r\nd")
	feed(s, "\x1b[2;3r") // rows 2–3
	feed(s, "\x1b[3;1H\n")
	wantRows(t, s, "a", "c", "", "d")
	feed(s, "\x1b[2;1H\x1bM") // reverse index at the region's top
	wantRows(t, s, "a", "", "c", "d")
	feed(s, "\x1b[r\x1b[1;1H\x1b[L")
	wantRows(t, s, "", "a", "", "c")
	feed(s, "\x1b[2M")
	wantRows(t, s, "", "c", "", "")
	feed(s, "\x1b[2;1Hwxyz\x1b[2;2H\x1b[2P")
	wantRows(t, s, "", "wz", "", "")
	feed(s, "\x1b[2;2H\x1b[2@")
	wantRows(t, s, "", "w  z", "", "")
	feed(s, "\x1b[4;1H-\x1b[3b")
	wantRows(t, s, "", "w  z", "", "----")
}

func TestModes(t *testing.T) {
	s := New(10, 2)
	feed(s, "\x1b[?1h\x1b[?2004h\x1b[?1002h\x1b[?1006h\x1b[?25l")
	m := s.Modes()
	if !m.AppCursor || !m.BracketedPaste || m.Mouse != MouseDrag || !m.SGRMouse {
		t.Errorf("modes %+v", m)
	}
	if s.View(0).CursorVisible {
		t.Error("a hidden cursor shows")
	}
	feed(s, "\x1bc")
	if m := s.Modes(); m.AppCursor || m.BracketedPaste || m.Mouse != MouseOff {
		t.Errorf("after a reset %+v", m)
	}
}

func TestTabs(t *testing.T) {
	s := New(20, 1)
	feed(s, "a\tb\tc")
	wantRows(t, s, "a       b       c")
}

func TestResize(t *testing.T) {
	s := New(6, 4)
	feed(s, "one\r\ntwo\r\nthree\r\n$ ")
	s.Resize(4, 2) // the lines go from the top; the prompt stays
	wantRows(t, s, "thre", "$")
	if v := s.View(0); v.CursorY != 1 || v.History != 2 {
		t.Errorf("after shrinking: cursor row %d, history %d", v.CursorY, v.History)
	}
	s.Resize(8, 3)
	wantRows(t, s, "thre", "$", "")
	feed(s, "ok")
	wantRows(t, s, "thre", "$ ok", "")
	// Lines below the cursor go before lines above it.
	s = New(4, 4)
	feed(s, "a\r\nb\x1b[4;1Hd\x1b[2;2H")
	s.Resize(4, 2)
	wantRows(t, s, "a", "b")
}

// TestNoPanicOnNoise feeds arbitrary bytes: the emulator must never panic or leave its cursor
// off the screen.
func TestNoPanicOnNoise(t *testing.T) {
	s := New(7, 3)
	seed := uint32(1)
	b := make([]byte, 20000)
	for i := range b {
		seed = seed*1664525 + 1013904223
		b[i] = byte(seed >> 24)
		if i%7 == 0 {
			b[i] = "\x1b[;?0123456789mHJKLMPXr@hl"[seed%26]
		}
	}
	for i := 0; i < len(b); i += 13 {
		s.Write(b[i:min(i+13, len(b))])
		if i%1000 == 0 {
			s.Resize(int(seed%9)+2, int(seed%5)+1)
		}
		v := s.View(0)
		if v.CursorX < 0 || v.CursorY < 0 || v.CursorX >= len(v.Lines[0].Cells) || v.CursorY >= len(v.Lines) {
			t.Fatalf("cursor %d,%d off a %dx%d screen", v.CursorX, v.CursorY, len(v.Lines[0].Cells), len(v.Lines))
		}
	}
}
