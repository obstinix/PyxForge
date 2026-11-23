package codeedit

import (
	"testing"

	"github.com/obstinix/PyxForge/internal/textbuf"
)

func model(text string) *Model { return NewModel(textbuf.New(text)) }

func typeAll(m *Model, s string) {
	for _, r := range s {
		m.Type(r)
	}
}

func TestTypingPairsAndStepsOver(t *testing.T) {
	m := model("")
	typeAll(m, "f(")
	if m.Buf.Text() != "f()" || m.Cur.Col != 2 {
		t.Fatalf("%q cursor %v", m.Buf.Text(), m.Cur)
	}
	typeAll(m, "x)")
	if m.Buf.Text() != "f(x)" || m.Cur.Col != 4 {
		t.Errorf("stepping over the closer: %q cursor %v", m.Buf.Text(), m.Cur)
	}
	m = model("dont")
	m.SetCursor(Pos{Line: 0, Col: 3}, false)
	m.Type('\'')
	if m.Buf.Text() != "don't" {
		t.Errorf("an apostrophe was paired: %q", m.Buf.Text())
	}
	m = model("word")
	m.SelectAll()
	m.Type('"')
	if m.Buf.Text() != `"word"` || m.SelectedText() != "word" {
		t.Errorf("wrapping a selection: %q, selected %q", m.Buf.Text(), m.SelectedText())
	}
	m = model("")
	typeAll(m, "(")
	m.Backspace()
	if m.Buf.Text() != "" {
		t.Errorf("backspace in an empty pair left %q", m.Buf.Text())
	}
}

func TestNewlineIndents(t *testing.T) {
	m := model("\tint f() {}")
	m.Indent = "\t"
	m.SetCursor(Pos{Line: 0, Col: 10}, false) // between { and }
	m.Newline()
	if m.Buf.Text() != "\tint f() {\n\t\t\n\t}" || m.Cur != (Pos{Line: 1, Col: 2}) {
		t.Errorf("%q cursor %v", m.Buf.Text(), m.Cur)
	}
	m = model("if x:")
	m.Indent = "    "
	m.SetCursor(Pos{Line: 0, Col: 5}, false)
	m.Newline()
	if m.Buf.Text() != "if x:\n    " {
		t.Errorf("after a colon: %q", m.Buf.Text())
	}
	m.Backspace() // back to the previous stop, not one space
	if m.Buf.Text() != "if x:\n" {
		t.Errorf("backspace in indentation: %q", m.Buf.Text())
	}
}

func TestIndentGuess(t *testing.T) {
	if m := model("a\n\tb\n\tc\n  d"); m.Indent != "\t" {
		t.Errorf("tabs: %q", m.Indent)
	}
	if m := model("a\n  b\n    c"); m.Indent != "  " {
		t.Errorf("two spaces: %q", m.Indent)
	}
}

func TestTabOutdentComment(t *testing.T) {
	m := model("a\nb\n\nc")
	m.Indent = "  "
	m.Select(Range{Start: Pos{Line: 0, Col: 0}, End: Pos{Line: 3, Col: 1}})
	m.Tab()
	if m.Buf.Text() != "  a\n  b\n\n  c" {
		t.Fatalf("indent: %q", m.Buf.Text())
	}
	m.Outdent()
	if m.Buf.Text() != "a\nb\n\nc" {
		t.Errorf("outdent: %q", m.Buf.Text())
	}
	m.LineComment = "//"
	m.ToggleComment()
	if m.Buf.Text() != "// a\n// b\n\n// c" {
		t.Errorf("comment: %q", m.Buf.Text())
	}
	m.ToggleComment()
	if m.Buf.Text() != "a\nb\n\nc" {
		t.Errorf("uncomment: %q", m.Buf.Text())
	}
	m.Undo()
	if m.Buf.Text() != "// a\n// b\n\n// c" {
		t.Errorf("one undo takes back one toggle: %q", m.Buf.Text())
	}
}

func TestMovement(t *testing.T) {
	m := model("foo.bar(baz)\n\tqux\nend")
	m.MoveCursor(WordRight, false, 1)
	if m.Cur.Col != 3 {
		t.Errorf("word right: %v", m.Cur)
	}
	m.MoveCursor(LineEnd, true, 1)
	if m.SelectedText() != ".bar(baz)" {
		t.Errorf("shift+end selected %q", m.SelectedText())
	}
	m.SetCursor(Pos{Line: 0, Col: 12}, false)
	m.MoveCursor(Down, false, 1)
	m.MoveCursor(Down, false, 1)
	m.MoveCursor(Up, false, 1)
	if m.Cur != (Pos{Line: 1, Col: 4}) { // display column 12 is past "\tqux" (4+3 columns)
		t.Errorf("down, down, up: %v", m.Cur)
	}
	m.MoveCursor(LineStart, false, 1)
	if m.Cur.Col != 1 {
		t.Errorf("home goes to the first non-blank: %v", m.Cur)
	}
	m.MoveCursor(LineStart, false, 1)
	if m.Cur.Col != 0 {
		t.Errorf("home again goes to column 0: %v", m.Cur)
	}
	if w := m.WordAt(Pos{Line: 0, Col: 5}); m.Buf.Slice(w) != "bar" {
		t.Errorf("word at: %q", m.Buf.Slice(w))
	}
}

func TestBracketsAndFolds(t *testing.T) {
	src := "int f() {\n\tif (x) {\n\t\ty();\n\t}\n}\nint g;"
	m := model(src)
	if _, mt, ok := m.MatchBracket(Pos{Line: 0, Col: 8}); !ok || mt != (Pos{Line: 4, Col: 0}) {
		t.Errorf("match of the first {: %v %v", mt, ok)
	}
	if _, mt, ok := m.MatchBracket(Pos{Line: 3, Col: 2}); !ok || mt != (Pos{Line: 1, Col: 8}) { // just after }
		t.Errorf("match of the inner }: %v %v", mt, ok)
	}
	if last, ok := m.FoldRange(0); !ok || last != 3 {
		t.Errorf("fold range of f: %d %v", last, ok)
	}
	m.ToggleFold(0)
	if got := m.VisibleLines(0, 10); len(got) != 3 || got[1] != 4 {
		t.Errorf("visible with f folded: %v", got)
	}
	m.SetCursor(Pos{Line: 2, Col: 0}, false) // moving into a fold opens it
	if m.LineHidden(2) {
		t.Error("the cursor is inside a fold")
	}
	py := model("def f():\n    a = 1\n\n    b = 2\nx = 3")
	if last, ok := py.FoldRange(0); !ok || last != 3 {
		t.Errorf("indentation fold: %d %v", last, ok)
	}
}

func TestFindReplace(t *testing.T) {
	m := model("one Two one\nthree one")
	r, ok := m.Find("one", Pos{Line: 0, Col: 1}, true, false)
	if !ok || r.Start != (Pos{Line: 0, Col: 8}) {
		t.Errorf("find: %v", r)
	}
	r, _ = m.Find("one", Pos{Line: 1, Col: 6}, true, false) // wraps
	if r.Start != (Pos{Line: 1, Col: 6}) {
		t.Errorf("find at the cursor: %v", r)
	}
	r, _ = m.Find("two", Pos{}, false, false)
	if r.Start != (Pos{Line: 0, Col: 4}) {
		t.Errorf("case-insensitive find: %v", r)
	}
	if n := m.ReplaceAll("one", "one!", true); n != 3 || m.Buf.Text() != "one! Two one!\nthree one!" {
		t.Errorf("replace all: %d %q", n, m.Buf.Text())
	}
	m.Undo()
	if m.Buf.Text() != "one Two one\nthree one" {
		t.Errorf("undo of replace all: %q", m.Buf.Text())
	}
}
