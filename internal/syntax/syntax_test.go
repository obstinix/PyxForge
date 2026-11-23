package syntax

import (
	"strings"
	"testing"
	"time"
)

func spanText(line string, s Span) string { return string([]rune(line)[s.Start:s.End]) }

func TestHighlightC(t *testing.T) {
	src := "#include <stdio.h>\n/* hi */\nint main(void) {\n\treturn printf(\"é%d\", 42);\n}"
	h := ForFile("main.c", "")
	if h == nil {
		t.Fatal("no highlighter for .c")
	}
	lines := strings.Split(src, "\n")
	got := h.Lines(src)
	if len(got) != len(lines) {
		t.Fatalf("%d span lines for %d lines", len(got), len(lines))
	}
	find := func(line int, text string) Kind {
		for _, s := range got[line] {
			if spanText(lines[line], s) == text {
				return s.Kind
			}
		}
		t.Errorf("line %d: no span %q in %v", line, text, got[line])
		return Plain
	}
	if k := find(1, "/* hi */"); k != Comment {
		t.Errorf("comment is %v", k)
	}
	if k := find(2, "int"); k != Type {
		t.Errorf("int is %v", k)
	}
	if k := find(3, "return"); k != Keyword {
		t.Errorf("return is %v", k)
	}
	if k := find(3, "42"); k != Number {
		t.Errorf("42 is %v", k)
	}
	// Columns count runes: the string with é ends where it does on screen.
	for _, s := range got[3] {
		if s.Kind == String && strings.Contains(spanText(lines[3], s), "\"") && !strings.HasPrefix(spanText(lines[3], s), "\"") && !strings.HasSuffix(spanText(lines[3], s), "\"") {
			t.Errorf("string span off by bytes: %q", spanText(lines[3], s))
		}
	}
}

func TestLanguagesPyxForgeOpens(t *testing.T) {
	for _, f := range []string{"a.c", "a.h", "a.cpp", "a.rs", "a.go", "a.py", "a.js", "a.ts", "a.java", "boot.asm", "Makefile", "pyxforge.toml", "a.lua", "a.sh", "README.md", "link.ld"} {
		if ForFile(f, "") == nil {
			t.Errorf("no highlighter for %s", f)
		}
	}
	if ForFile("notes.unknownext", "") != nil {
		t.Error("a highlighter for an unknown file")
	}
}

// TestHighlightSpeed: colouring a 5,000-line C file is quick enough to run after an edit.
// raceDetector is set by race_test.go when the tests run with -race.
var raceDetector bool

func TestHighlightSpeed(t *testing.T) {
	var b strings.Builder
	for i := 0; i < 1000; i++ {
		b.WriteString("static int f(int x) {\n\t// add one\n\treturn x + 1;\n}\n\n")
	}
	h := ForFile("big.c", "")
	start := time.Now()
	h.Lines(b.String())
	d := time.Since(start)
	t.Logf("5,000 lines in %v", d)
	if !raceDetector && d > 2*time.Second { // the race detector slows Chroma's regexps about 13 times
		t.Errorf("5,000 lines took %v", d)
	}
}
