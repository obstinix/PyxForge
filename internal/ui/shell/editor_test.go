package shell

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
)

// queue is a UI-thread stand-in for tests: background goroutines (language servers, builds,
// terminals) post work, the test goroutine runs it, so shell state is only touched from one
// goroutine as it is in the real app.
type queue chan func()

func (q queue) post(f func()) { q <- f }

// pumpUntil runs posted work until cond holds.
func (q queue) pumpUntil(t *testing.T, what string, cond func() bool) {
	t.Helper()
	q.pumpWithin(t, what, 15*time.Second, cond)
}

// pumpWithin is pumpUntil with its own deadline, for waits on outside programs (compilers,
// debuggers) that a loaded machine can slow down.
func (q queue) pumpWithin(t *testing.T, what string, d time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.After(d)
	for !cond() {
		select {
		case f := <-q:
			f()
		case <-time.After(20 * time.Millisecond):
		case <-deadline:
			t.Fatalf("timed out waiting for %s", what)
		}
	}
}

// newEditorShell is a shell that edits files (PyxForge's editor, D16), with a queue for work
// posted to the UI thread, in an empty workspace.
func newEditorShell(t *testing.T) (*Shell, string, queue) {
	t.Helper()
	root := t.TempDir()
	q := make(queue, 1024)
	s := NewWithOptions(test.NewTempApp(t), root, Options{Editor: true, Dispatch: q.post})
	t.Cleanup(func() { s.stopEditor(); s.term.stop() })
	return s, root, q
}

// editorFocused reports whether the document in the selected tab has keyboard focus.
func editorFocused(s *Shell) bool {
	d := s.activeDoc()
	return d != nil && s.win.Canvas().Focused() == fyne.Focusable(d.ed)
}

// TestStopLine: when a debugger stops at a source line, the editor opens the file and marks the
// line; running on clears the mark; a stop without a source line (a boot sector) marks nothing
// (#58).
func TestStopLine(t *testing.T) {
	s, root, _ := newEditorShell(t)
	src := filepath.Join(root, "hello.c")
	if err := os.WriteFile(src, []byte("#include <stdio.h>\nint main(void) {\n  puts(\"hi\");\n  return 0;\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	marked := func() int {
		if d := s.activeDoc(); d != nil {
			return d.stopLine
		}
		return 0
	}
	s.showStopLine(src, 3)
	if s.activeFile() != src || marked() != 3 {
		t.Fatalf("the editor shows %q, line %d marked", s.activeFile(), marked())
	}
	if d := s.activeDoc(); d.ed.Cur.Line != 2 {
		t.Errorf("the cursor is on line %d, not the stopped line", d.ed.Cur.Line+1)
	}
	s.clearStopLine()
	if marked() != 0 {
		t.Error("the mark stayed after running on")
	}
	s.showStopLine(src, 4)
	if marked() != 4 {
		t.Errorf("line %d marked, want 4", marked())
	}
	s.showStopLine("", 0) // stopped in code with no debug information
	if marked() != 0 {
		t.Error("a stop without a source line left a mark")
	}
}

// TestGoToLine: Go to Line moves the cursor to a line and column, counted in characters, and
// keeps it within the file; bad input is refused.
func TestGoToLine(t *testing.T) {
	s, root, _ := newEditorShell(t)
	f := filepath.Join(root, "kernel.c")
	if err := os.WriteFile(f, []byte("int a;\nint ünïcode = 1;\nint c;\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	s.OpenFile(f)
	d := s.activeDoc()
	if d == nil {
		t.Fatal("kernel.c did not open")
	}
	s.goToLineIn("2:7")
	if got := string(d.ed.Buf.Line(1)[d.ed.Cur.Col]); d.ed.Cur.Line != 1 || got != "ï" {
		t.Errorf("the cursor is at %v on %q, want the ï of ünïcode", d.ed.Cur, got)
	}
	s.goToLineIn("99")
	if last := d.ed.Buf.LineCount() - 1; d.ed.Cur.Line != last {
		t.Errorf("Go to Line 99 put the cursor on line %d of %d", d.ed.Cur.Line+1, last+1)
	}
	for _, bad := range []string{"", "0", "x", "3:0", "3:y"} {
		if _, _, err := parseLineCol(bad); err == nil {
			t.Errorf("%q was accepted", bad)
		}
	}
}
