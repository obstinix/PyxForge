package shell

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"fyne.io/fyne/v2/test"
	"github.com/obstinix/PyxForge/internal/ui/codeedit"
	"github.com/obstinix/PyxForge/internal/workspace"
)

// TestBreakpointsAreKept: a breakpoint set with Toggle Breakpoint is kept by the shell, saved
// with the workspace, and shown again in the gutter of the next window on the workspace.
func TestBreakpointsAreKept(t *testing.T) {
	root := t.TempDir()
	store := &workspace.StateStore{Dir: filepath.Join(t.TempDir(), "workspaces")}
	file := filepath.Join(root, "main.py")
	if err := os.WriteFile(file, []byte("a = 1\nb = 2\nprint(a + b)\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	open := func() (*Shell, queue) {
		q := make(queue, 1024)
		s := NewWithOptions(test.NewTempApp(t), root, Options{Editor: true, Dispatch: q.post, State: store})
		t.Cleanup(func() { s.stopEditor(); s.term.stop() })
		return s, q
	}

	s, _ := open()
	s.OpenFile(file)
	d := s.activeDoc()
	if d == nil {
		t.Fatal("main.py did not open")
	}
	if !s.available("debug.toggleBreakpoint") {
		t.Error("Toggle Breakpoint is greyed out with a file open")
	}
	d.ed.SetCursor(codeedit.Pos{Line: 2, Col: 0}, false)
	s.toggleBreakpoint()
	if len(s.breakpoints) != 1 || !slices.Equal(s.breakpoints[0].Lines, []int{3}) {
		t.Fatalf("breakpoints %+v", s.breakpoints)
	}
	if st := s.captureState(); len(st.Breakpoints) != 1 || st.Breakpoints[0].Lines[0] != 3 {
		t.Fatalf("saved breakpoints %+v", st.Breakpoints)
	}
	s.saveState()
	s.stopEditor()

	// The next window on the workspace.
	again, q2 := open()
	q2.pumpUntil(t, "the restored editor", func() bool { return again.activeDoc() != nil })
	list := again.currentBreakpoints()
	if len(list) != 1 || !slices.Equal(list[0].Lines, []int{3}) || !strings.EqualFold(filepath.Clean(list[0].File), filepath.Clean(file)) {
		t.Errorf("breakpoints after reopening: %+v", list)
	}
	if _, marked := again.activeDoc().ed.Marks()[2]; !marked {
		t.Error("the gutter does not show the restored breakpoint on line 3")
	}
}
