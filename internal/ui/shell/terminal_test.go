package shell

import (
	"github.com/obstinix/PyxForge/internal/pty"
	"slices"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/test"
	"github.com/obstinix/PyxForge/internal/proc"
	"github.com/obstinix/PyxForge/internal/ui/termview"
	"github.com/obstinix/PyxForge/internal/ui/theme"
)

// rendersCursor reports whether the terminal view draws its cursor: a cell-sized rectangle at
// the cursor's cell.
func rendersCursor(tm *terminalHost) bool {
	v := tm.view.Screen().View(0)
	cell := tm.view.CellSize()
	at := fyne.NewPos(termview.Inset+termview.Gutter+float32(v.CursorX)*cell.Width, termview.Inset+float32(v.CursorY)*cell.Height)
	for _, o := range test.WidgetRenderer(tm.view).Objects() {
		if r, ok := o.(*canvas.Rectangle); ok && r.Position() == at && r.Size() == cell {
			return true
		}
	}
	return false
}

// screenCount is how often want shows in a session's screen and scrollback.
func screenCount(ts *termSession, want string) int {
	if ts == nil || ts.t == nil {
		return 0
	}
	return strings.Count(ts.t.Screen.String(), want)
}

// TestTerminalSessions opens the Terminal tab, runs commands in two sessions whose output stays
// apart, reports a shell's exit status, restarts it, renames and closes sessions, and checks
// that a closed session's shell process ends.
func TestTerminalSessions(t *testing.T) {
	s, _, q := newEditorShell(t)
	tm := s.term
	t.Cleanup(func() {
		if t.Failed() && tm.current != nil {
			t.Logf("screen:\n%s", tm.current.t.Screen.String())
		}
	})
	if len(tm.sessions) != 0 {
		t.Fatal("the terminal started before its tab opened")
	}
	typeLine := func(line string) {
		for _, r := range line {
			tm.view.TypedRune(r)
		}
		tm.view.TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})
	}

	// Opening the tab starts the first session.
	s.showDockItem(tm.tab)
	if len(tm.sessions) != 1 || tm.current.name != "Shell 1" || tm.body.Objects[0] != tm.pane().box || s.active != regionPanel {
		t.Fatalf("first session: %d sessions, active region %v", len(tm.sessions), s.active)
	}
	first := tm.current
	typeLine("echo first-session")
	q.pumpUntil(t, "the first session's output", func() bool { return screenCount(first, "first-session") >= 2 })
	// The prompt may still be arriving after the output: wait for the drawn cursor to settle
	// on the screen's cursor rather than comparing one frame with a later screen.
	q.pumpUntil(t, "a running session's cursor", func() bool {
		tm.view.Refresh()
		return rendersCursor(tm)
	})

	// A second session has its own output, and the first keeps running while hidden.
	tm.newSession()
	second := tm.current
	if len(tm.sessions) != 2 || second.name != "Shell 2" || second.t == first.t || tm.view.Screen() != second.t.Screen {
		t.Fatalf("second session: %+v", tm.sessions)
	}
	typeLine("echo second-session")
	q.pumpUntil(t, "the second session's output", func() bool { return screenCount(second, "second-session") >= 2 })
	if screenCount(second, "first-session") != 0 || screenCount(first, "second-session") != 0 {
		t.Error("the sessions' output mixed")
	}
	tm.cycle(1)
	if tm.current != first || tm.view.Screen() != first.t.Screen {
		t.Fatal("cycle did not show the first session again")
	}
	if !proc.Alive(second.t.Pid()) {
		t.Error("the hidden session's shell stopped")
	}

	// A shell exits: the session reports its status, and what it showed stays on screen under
	// a line saying how it ended (PXF-DEF-015); Restart gives it a new shell.
	typeLine("exit 3")
	q.pumpUntil(t, "the exit report", func() bool { return first.done })
	if first.status != 3 || len(tm.pane().box.Objects) != 2 || !strings.HasPrefix(sessionState(first), "exit 3") {
		t.Fatalf("after exit: status %d, rail says %q", first.status, sessionState(first))
	}
	if !slices.Contains(texts(tm.body), "The shell exited with status 3.") || findButton(tm.body, "Restart") == nil {
		t.Errorf("the exit line says %q", texts(tm.body))
	}
	if rendersCursor(tm) {
		t.Error("an ended session still draws a cursor")
	}
	if screenCount(first, "first-session") < 1 || tm.view.Screen() != first.t.Screen {
		t.Error("the ended session's output is gone")
	}
	old := first.t
	tm.restart()
	if first.done || first.t == old || len(tm.pane().box.Objects) != 1 || first.name != "Shell 1" {
		t.Fatalf("restart: %+v", first)
	}
	typeLine("echo restarted")
	q.pumpUntil(t, "the restarted shell", func() bool { return screenCount(first, "restarted") >= 2 })

	// Rename keeps names unique.
	tm.rename(first, "build watch")
	tm.rename(second, "build watch")
	if first.name != "build watch" || second.name != "Shell 2" {
		t.Errorf("names %q, %q", first.name, second.name)
	}

	// Closing a session ends its shell and selects another.
	pid := second.t.Pid()
	tm.selectSession(second)
	tm.closeCurrent()
	if len(tm.sessions) != 1 || tm.current != first {
		t.Fatalf("after close: %d sessions, current %+v", len(tm.sessions), tm.current)
	}
	q.pumpUntil(t, "the closed shell to end", func() bool { return !proc.Alive(pid) })

	// Stop ends the remaining shells.
	pid = first.t.Pid()
	tm.stop()
	q.pumpUntil(t, "the last shell to end", func() bool { return !proc.Alive(pid) })
}

// TestExitStripFitsANarrowPanel: the longest exit status wraps, and Restart stays whole and
// inside a panel at the editor's narrowest (PXF-DEF-006).
func TestExitStripFitsANarrowPanel(t *testing.T) {
	test.NewTempApp(t)
	strip := exitStrip(exitDescription("The shell", int(int32(-1073741510)))+".", true, func() {})
	w := test.NewWindow(strip)
	defer w.Close()
	w.Resize(fyne.NewSize(minEditorW, 200))
	b := findButton(strip, "Restart")
	if b == nil {
		t.Fatal("no Restart button")
	}
	pos := fyne.CurrentApp().Driver().AbsolutePositionForObject(b)
	if right := pos.X + b.Size().Width; right > w.Canvas().Size().Width+0.5 || b.Size().Width < b.MinSize().Width {
		t.Errorf("Restart ends at %.0f of %.0f, %.0f wide of %.0f", right, w.Canvas().Size().Width, b.Size().Width, b.MinSize().Width)
	}
	if strip.MinSize().Width > minEditorW {
		t.Errorf("the strip needs %.0f, more than the panel's %d", strip.MinSize().Width, minEditorW)
	}
}

// TestEditorEmptyStateFitsANarrowEditor: the editor's empty state stays inside an editor at its
// narrowest; the note wraps instead of running past both edges (PXF-DEF-006).
func TestEditorEmptyStateFitsANarrowEditor(t *testing.T) {
	s, _ := newTestShell(t)
	w := test.NewWindow(s.editorEmpty)
	defer w.Close()
	w.Resize(fyne.NewSize(minEditorW, minEditorH*3))
	d := fyne.CurrentApp().Driver()
	at, size := d.AbsolutePositionForObject(s.emptyNote), s.emptyNote.Size()
	if at.X < 0 || at.X+size.Width > float32(minEditorW)+0.5 {
		t.Errorf("the note spans %.0f to %.0f in an editor %d wide", at.X, at.X+size.Width, minEditorW)
	}
	if size.Height <= s.emptyNote.MinSize().Height/2 || s.editorEmpty.MinSize().Width > minEditorW {
		t.Errorf("the empty state needs %.0f wide; the note is %.0f high", s.editorEmpty.MinSize().Width, size.Height)
	}
	// The note's text starts at the title's left edge (#51): a label insets its text by Space2.
	if title := d.AbsolutePositionForObject(s.emptyTitle).X; title != at.X+theme.Space2 {
		t.Errorf("the title starts at %.1f and the note's text at %.1f", title, at.X+theme.Space2)
	}
}

// TestTerminalUsesTheChosenShell: a new session runs the shell chosen in Settings > Terminal >
// Shell (D24), and with no choice the preferred one found on this machine.
func TestTerminalUsesTheChosenShell(t *testing.T) {
	s, _, q := newEditorShell(t)
	shells := pty.Shells()
	if got := s.terminalShell(); got.Argv[0] != shells[0].Argv[0] {
		t.Errorf("with no choice: %v, want %v", got.Argv, shells[0].Argv)
	}
	last := shells[len(shells)-1]
	s.app.Preferences().SetString(prefTerminalShell, last.Argv[0])
	tm := s.term
	s.showDockItem(tm.tab)
	q.pumpUntil(t, "a session", func() bool { return tm.current != nil && tm.current.t != nil })
	if got := tm.current.t.Argv; len(got) == 0 || got[0] != last.Argv[0] {
		t.Errorf("the session runs %v, want %s", got, last.Argv[0])
	}
}
