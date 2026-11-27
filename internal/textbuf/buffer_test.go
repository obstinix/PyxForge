package textbuf

import (
	"bytes"
	"errors"
	"fmt"
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

// TestUTF16RoundTrip: UTF-16 with a byte-order mark, either byte order, opens as text and is
// written back byte for byte, line endings included; UTF-16 with a lone surrogate is refused.
func TestUTF16RoundTrip(t *testing.T) {
	dir := t.TempDir()
	for name, data := range map[string][]byte{
		"le.txt": {0xFF, 0xFE, 'h', 0, 0xE9, 0, '\r', 0, '\n', 0, 'x', 0},
		"be.txt": {0xFE, 0xFF, 0, 'h', 0, 0xE9, 0, '\n', 0, 'x'},
	} {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, data, 0o644); err != nil {
			t.Fatal(err)
		}
		b, err := Load(p)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if got := b.Text(); got != "hé\nx" {
			t.Errorf("%s reads as %q", name, got)
		}
		if err := b.Save(p); err != nil {
			t.Fatal(err)
		}
		if got, _ := os.ReadFile(p); !bytes.Equal(got, data) {
			t.Errorf("%s saved as % x, was % x", name, got, data)
		}
	}
	bad := filepath.Join(dir, "bad.txt")
	if err := os.WriteFile(bad, []byte{0xFF, 0xFE, 0x00, 0xD8, 'a', 0}, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(bad); !errors.Is(err, ErrNotText) {
		t.Errorf("a lone surrogate loaded: %v", err)
	}
}

// TestBinaryIsReadOnly: a binary file shown as text takes no edit, no undo and no save.
func TestBinaryIsReadOnly(t *testing.T) {
	p := filepath.Join(t.TempDir(), "image.bin")
	data := []byte{0x00, 0xFF, 'A', '\n', 'B'}
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatal(err)
	}
	b, err := LoadBinary(p)
	if err != nil {
		t.Fatal(err)
	}
	if b.LineCount() != 2 || string(b.Line(0)) != "\x00ÿA" {
		t.Fatalf("lines %q", b.Text())
	}
	b.Insert(Pos{Line: 0, Col: 0}, "x")
	if _, ok := b.Undo(); ok || b.Modified() || b.Text() != "\x00ÿA\nB" {
		t.Errorf("a read-only buffer changed: %q", b.Text())
	}
	if err := b.Save(p); !errors.Is(err, ErrReadOnly) {
		t.Errorf("Save: %v", err)
	}
	if got, _ := os.ReadFile(p); !bytes.Equal(got, data) {
		t.Error("the file changed")
	}
}

// TestEncodingAndLineEndingNames: the names the status bar shows match what Save will write.
func TestEncodingAndLineEndingNames(t *testing.T) {
	dir := t.TempDir()
	cases := []struct {
		data      []byte
		enc, ends string
	}{
		{[]byte("a\nb\n"), "UTF-8", "LF"},
		{[]byte("a\r\nb\r\n"), "UTF-8", "CRLF"},
		{[]byte("\xEF\xBB\xBFa\r\n"), "UTF-8 with BOM", "CRLF"},
		{[]byte{0xFF, 0xFE, 'a', 0, '\n', 0}, "UTF-16 LE", "LF"},
		{[]byte{0xFE, 0xFF, 0, 'a', 0, '\r', 0, '\n'}, "UTF-16 BE", "CRLF"},
	}
	for i, c := range cases {
		p := filepath.Join(dir, fmt.Sprintf("f%d.txt", i))
		if err := os.WriteFile(p, c.data, 0o644); err != nil {
			t.Fatal(err)
		}
		b, err := Load(p)
		if err != nil {
			t.Fatalf("%q: %v", c.data, err)
		}
		if b.EncodingName() != c.enc || b.LineEnding() != c.ends {
			t.Errorf("%q: %s %s, want %s %s", c.data, b.EncodingName(), b.LineEnding(), c.enc, c.ends)
		}
	}
	if b := New("new file"); b.EncodingName() != "UTF-8" || b.LineEnding() != "LF" {
		t.Errorf("a new buffer: %s %s", b.EncodingName(), b.LineEnding())
	}
}
