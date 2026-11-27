package vt

import (
	"fmt"
	"runtime"
	"testing"
)

// prompt writes what an integrated shell writes for one command: prompt, command, output, end.
func prompt(s *Screen, cmd, out string, exit int) {
	fmt.Fprintf(s, "\x1b]133;A\x07$ \x1b]133;B\x07%s\r\n\x1b]133;C\x07%s\x1b]133;D;%d\x07", cmd, out, exit)
}

func kinds(ms []Mark) string {
	b := make([]byte, len(ms))
	for i, m := range ms {
		b[i] = byte(m.Kind)
	}
	return string(b)
}

// TestOSC133Marks: each mark is recorded where the cursor is, D with its exit status, both
// with BEL and with ST as the terminator.
func TestOSC133Marks(t *testing.T) {
	s := New(40, 10)
	prompt(s, "make", "ok\r\n", 0)
	s.Write([]byte("\x1b]133;A\x1b\\$ \x1b]133;B\x1b\\false\r\n\x1b]133;C\x1b\\\x1b]133;D;2\x1b\\"))
	ms := s.Marks()
	if kinds(ms) != "ABCDABCD" {
		t.Fatalf("marks %q", kinds(ms))
	}
	if ms[0].At != (Pos{0, 0}) || ms[1].At != (Pos{0, 2}) || ms[2].At != (Pos{1, 0}) || ms[3].At != (Pos{2, 0}) {
		t.Errorf("positions %v %v %v %v", ms[0].At, ms[1].At, ms[2].At, ms[3].At)
	}
	if ms[3].Exit != 0 || ms[7].Exit != 2 || ms[0].Exit != -1 {
		t.Errorf("exits %d %d %d", ms[3].Exit, ms[7].Exit, ms[0].Exit)
	}
	if ms[7].Seq != 8 || s.MarkSeq() != 8 {
		t.Errorf("seq %d %d", ms[7].Seq, s.MarkSeq())
	}
	s.Write([]byte("\x1b]133;D\x07\x1b]133;Z\x07\x1b]133;\x07"))
	if ms := s.Marks(); len(ms) != 9 || ms[8].Exit != -1 {
		t.Errorf("a D without status: %v; unknown kinds kept: %d marks", ms[len(ms)-1], len(ms))
	}
}

// TestMarksSplitAcrossReads: a mark arriving a byte at a time is the same mark.
func TestMarksSplitAcrossReads(t *testing.T) {
	s := New(40, 10)
	for _, c := range []byte("x\x1b]133;D;17\x1b\\y\x1b]7;file://host/tmp/a%20b\x07") {
		s.Write([]byte{c})
	}
	ms := s.Marks()
	if len(ms) != 1 || ms[0].Kind != MarkEnd || ms[0].Exit != 17 || ms[0].At != (Pos{0, 1}) {
		t.Fatalf("marks %+v", ms)
	}
	want := "/tmp/a b"
	if runtime.GOOS == "windows" {
		want = `tmp\a b`
	}
	if s.Cwd() != want {
		t.Errorf("cwd %q", s.Cwd())
	}
}

// TestOSC7: the folder is decoded, Windows drive paths included, and later marks carry it.
func TestOSC7(t *testing.T) {
	s := New(40, 10)
	s.Write([]byte("\x1b]7;file://pc/C:/Users/me/My%20Code\x07\x1b]133;A\x07"))
	want := "/C:/Users/me/My Code"
	if runtime.GOOS == "windows" {
		want = `C:\Users\me\My Code`
	}
	if s.Cwd() != want || s.Marks()[0].Cwd != want {
		t.Errorf("cwd %q, mark's %q", s.Cwd(), s.Marks()[0].Cwd)
	}
	s.Write([]byte("\x1b]7;http://x/y\x07\x1b]7;not a url %zz\x07"))
	if s.Cwd() != want {
		t.Errorf("a non-file URL changed the folder to %q", s.Cwd())
	}
}

// TestMarksSurviveScrolling: a mark keeps its absolute line as its line scrolls into the
// history, and its line's text is still there.
func TestMarksSurviveScrolling(t *testing.T) {
	s := New(20, 4)
	prompt(s, "seq 30", "", 0)
	first := s.Marks()[1]
	for i := range 30 {
		fmt.Fprintf(s, "%d\r\n", i)
	}
	if got := s.Marks()[1]; got.At != first.At {
		t.Errorf("the command mark moved from %v to %v", first.At, got.At)
	}
	if text, _ := s.LineText(first.At.Line); text != "$ seq 30" {
		t.Errorf("its line reads %q", text)
	}
	s.Resize(10, 2)
	s.Resize(30, 8)
	if got := s.Marks()[1]; got.At != first.At {
		t.Errorf("a resize moved the mark to %v", got.At)
	}
}

// TestMarksDroppedWithHistory: marks on lines the history limit trims, or a clear removes, go
// with them; marks on lines kept stay.
func TestMarksDroppedWithHistory(t *testing.T) {
	s := New(20, 3)
	s.SetMaxHistory(5)
	prompt(s, "old", "", 0)
	for i := range 10 {
		fmt.Fprintf(s, "%d\r\n", i)
	}
	prompt(s, "new", "", 1)
	ms := s.Marks()
	if kinds(ms) != "ABCD" || ms[1].At.Line < s.FirstLine() {
		t.Fatalf("after trimming: %q, first line %d, marks %+v", kinds(ms), s.FirstLine(), ms)
	}
	for range 3 {
		s.Write([]byte("\r\n"))
	}
	s.ClearHistory()
	for _, m := range s.Marks() {
		if m.At.Line < s.FirstLine() {
			t.Errorf("a mark on a cleared line: %+v", m)
		}
	}
	prompt(s, "x", "", 0)
	s.Write([]byte("\x1b[H\x1b[2J")) // clear: the screen's commands go
	if len(s.Marks()) != 0 {
		t.Errorf("marks after the screen was erased: %+v", s.Marks())
	}
}

// TestMarksIgnoredOnAltScreen: a full-screen program's screen has no commands.
func TestMarksIgnoredOnAltScreen(t *testing.T) {
	s := New(20, 5)
	s.Write([]byte("\x1b[?1049h\x1b]133;A\x07\x1b[2J\x1b[?1049l"))
	if len(s.Marks()) != 0 {
		t.Errorf("marks %+v", s.Marks())
	}
}
