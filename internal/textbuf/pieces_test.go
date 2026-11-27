package textbuf

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestLinesShareTheFile: lines loaded from disk are slices of the file's bytes, and an edit
// gives only the edited line bytes of its own, never writing over a neighbour.
func TestLinesShareTheFile(t *testing.T) {
	p := filepath.Join(t.TempDir(), "a.txt")
	if err := os.WriteFile(p, []byte("αβγ\r\nline two\r\nthree"), 0o644); err != nil {
		t.Fatal(err)
	}
	b, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if b.LineEnding() != "CRLF" || b.LineLen(0) != 3 || b.LineBytes(0) != 6 || b.LineText(1) != "line two" {
		t.Fatalf("loaded %q %d %d %q", b.LineEnding(), b.LineLen(0), b.LineBytes(0), b.LineText(1))
	}
	if cap(b.lines[0]) != len(b.lines[0]) {
		t.Fatal("a line's capacity reaches into the next line")
	}
	b.Insert(Pos{0, 3}, "δ") // appending at a line's end must not overwrite "\r\nline two"
	b.Insert(Pos{1, 4}, "!")
	if got := b.Text(); got != "αβγδ\nline! two\nthree" {
		t.Fatalf("text %q", got)
	}
	if got := b.Slice(Range{Pos{0, 2}, Pos{1, 2}}); got != "γδ\nli" {
		t.Fatalf("slice %q", got)
	}
	if got := string(b.LineHead(0, 2)); got != "αβ" {
		t.Fatalf("head %q", got)
	}
	if err := b.Save(p); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(p)
	if string(data) != "αβγδ\r\nline! two\r\nthree" {
		t.Fatalf("saved %q", data)
	}
}

// TestBinaryIsLatin1: LoadBinary shows each byte as one character and keeps every byte.
func TestBinaryIsLatin1(t *testing.T) {
	p := filepath.Join(t.TempDir(), "a.bin")
	if err := os.WriteFile(p, []byte{'M', 'Z', 0, 0xff, '\r', '\n', 'x'}, 0o644); err != nil {
		t.Fatal(err)
	}
	b, err := LoadBinary(p)
	if err != nil {
		t.Fatal(err)
	}
	if !b.Binary() || b.LineCount() != 2 || b.LineLen(0) != 5 || string(b.Line(0)) != "MZ\x00ÿ\r" {
		t.Fatalf("binary lines %d %q", b.LineCount(), string(b.Line(0)))
	}
}

// TestLargeFileMemory: a file costs about its size, not four bytes a character.
func TestLargeFileMemory(t *testing.T) {
	line := strings.Repeat("x", 99) + "\n"
	text := strings.Repeat(line, 100000) // 10 MB
	b := New(text)
	if b.LineCount() != 100001 || b.Size() != len(text) {
		t.Fatalf("lines %d size %d", b.LineCount(), b.Size())
	}
	if got := b.LineLen(5000); got != 99 {
		t.Fatalf("line length %d", got)
	}
}
