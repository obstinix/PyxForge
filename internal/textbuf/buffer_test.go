package textbuf

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEditsAndChanges(t *testing.T) {
	b := New("int main() {\n}\n")
	var got []Change
	b.OnChange = func(c Change) { got = append(got, c) }
	end := b.Insert(Pos{0, 12}, "\n\treturn 0;")
	if end != (Pos{1, 10}) {
		t.Errorf("insert ended at %v", end)
	}
	if want := "int main() {\n\treturn 0;\n}\n"; b.Text() != want {
		t.Fatalf("text %q", b.Text())
	}
	b.Delete(Range{Pos{1, 1}, Pos{1, 8}}) // "return "... leaves "\t0;"
	if string(b.Line(1)) != "\t0;" {
		t.Errorf("line 1 is %q", string(b.Line(1)))
	}
	if len(got) != 2 || got[0].Range != (Range{Pos{0, 12}, Pos{0, 12}}) || got[1].Text != "" {
		t.Errorf("changes %+v", got)
	}
	if s := b.Slice(Range{Pos{0, 4}, Pos{1, 2}}); s != "main() {\n\t0" {
		t.Errorf("slice %q", s)
	}
	if b.Clamp(Pos{9, 9}) != b.End() || b.Clamp(Pos{0, 99}) != (Pos{0, 12}) {
		t.Error("clamp")
	}
}

func TestUndoRedoGroups(t *testing.T) {
	b := New("")
	p := Pos{}
	for _, r := range "abc" { // one run of typing
		p = b.Insert(p, string(r))
	}
	b.Seal()
	b.Insert(p, " def")
	if b.Text() != "abc def" {
		t.Fatal(b.Text())
	}
	if at, ok := b.Undo(); !ok || b.Text() != "abc" || at != (Pos{0, 3}) {
		t.Fatalf("first undo: %q at %v", b.Text(), at)
	}
	if _, ok := b.Undo(); !ok || b.Text() != "" {
		t.Fatalf("second undo took back the typing as one: %q", b.Text())
	}
	if _, ok := b.Undo(); ok {
		t.Error("undo with nothing left")
	}
	b.Redo()
	b.Redo()
	if b.Text() != "abc def" {
		t.Errorf("redo: %q", b.Text())
	}
	b.Undo()
	b.Insert(Pos{0, 0}, "x") // a new edit drops what could be redone
	if _, ok := b.Redo(); ok {
		t.Error("redo after a new edit")
	}
	// Multi-line replace and its undo.
	b = New("one\ntwo\nthree")
	b.Replace(Range{Pos{0, 1}, Pos{2, 2}}, "X\nY")
	if b.Text() != "oX\nYree" {
		t.Fatalf("replace: %q", b.Text())
	}
	b.Undo()
	if b.Text() != "one\ntwo\nthree" {
		t.Errorf("undo of a multi-line replace: %q", b.Text())
	}
}

func TestModifiedFollowsUndo(t *testing.T) {
	b := New("x")
	b.Insert(Pos{0, 1}, "y")
	if !b.Modified() {
		t.Fatal("not modified after an edit")
	}
	b.Undo()
	if b.Modified() {
		t.Error("modified after undoing back to the loaded text")
	}
	b.Redo()
	if !b.Modified() {
		t.Error("not modified after redo")
	}
}

func TestLoadSaveKeepsLineEndingsAndBOM(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "a.c")
	orig := "\xEF\xBB\xBFint x;\r\nint y;\r\n"
	if err := os.WriteFile(p, []byte(orig), 0o644); err != nil {
		t.Fatal(err)
	}
	b, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if b.Text() != "int x;\nint y;\n" {
		t.Fatalf("loaded %q", b.Text())
	}
	b.Insert(Pos{1, 6}, " // y")
	if err := b.Save(p); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(p)
	if string(got) != "\xEF\xBB\xBFint x;\r\nint y; // y\r\n" {
		t.Errorf("saved %q", got)
	}
	if b.Modified() {
		t.Error("modified after save")
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Errorf("save left %d files behind", len(entries))
	}
	bin := filepath.Join(dir, "b.bin")
	os.WriteFile(bin, []byte{0x7f, 'E', 'L', 'F', 0, 1}, 0o644)
	if _, err := Load(bin); err == nil {
		t.Error("a binary file loaded as text")
	}
}
