package shell

import (
	"slices"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/test"
	fyneTheme "fyne.io/fyne/v2/theme"
	"github.com/obstinix/PyxForge/internal/neovim"
	"github.com/obstinix/PyxForge/internal/proc"
	"github.com/obstinix/PyxForge/internal/ui/theme"
)

func gridCount(g neovim.Snapshot, want string) int {
	n := 0
	rows, _ := g.Size()
	for r := range rows {
		n += strings.Count(g.Line(r), want)
	}
	return n
}

// shellPid reads the process ID of a terminal session's shell from the panel's Neovim.
func shellPid(t *testing.T, tm *terminalHost, ts *termSession) int {
	t.Helper()
	var pid int
	if err := tm.sess.ExecLua(`return vim.fn.jobpid(vim.b[...].terminal_job_id)`, &pid, ts.buf); err != nil {
		t.Fatal(err)
	}
	return pid
}

// TestTerminalSessions opens the Terminal tab, runs commands in two sessions whose output stays
// apart, reports a shell's exit status, restarts it, renames and closes sessions, and checks
// that a closed session's shell process ends.
func TestTerminalSessions(t *testing.T) {
	s, _, q := newEditorShell(t)
	tm := s.term
	t.Cleanup(func() {
		if t.Failed() && tm.sess != nil {
			g := tm.sess.Grid()
			rows, _ := g.Size()
			for r := range rows {
				if l := strings.TrimSpace(g.Line(r)); l != "" {
					t.Logf("grid %2d| %s", r, l)
				}
			}
		}
	})
	if tm.sess != nil {
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
	if len(tm.sessions) != 1 || tm.current.name != "Shell 1" || tm.body.Objects[0] != tm.view || s.active != regionPanel {
		t.Fatalf("first session: %d sessions, active region %v", len(tm.sessions), s.active)
	}
	first := tm.current
	typeLine("echo first-session")
	q.pumpUntil(t, "the first session's output", func() bool { return gridCount(tm.sess.Grid(), "first-session") >= 2 })

	// A second session has its own output, and the first keeps running while hidden.
	tm.newSession()
	second := tm.current
	if len(tm.sessions) != 2 || second.name != "Shell 2" || second.buf == first.buf {
		t.Fatalf("second session: %+v", tm.sessions)
	}
	typeLine("echo second-session")
	q.pumpUntil(t, "the second session's output", func() bool { return gridCount(tm.sess.Grid(), "second-session") >= 2 })
	if gridCount(tm.sess.Grid(), "first-session") != 0 {
		t.Error("the second session shows the first one's output")
	}
	tm.cycle(1)
	q.pumpUntil(t, "the first session again", func() bool {
		return tm.current == first && gridCount(tm.sess.Grid(), "first-session") >= 2
	})
	if !proc.Alive(shellPid(t, tm, second)) {
		t.Error("the hidden session's shell stopped")
	}

	// A shell exits: the session reports its status, and what it showed stays on screen, as a
	// read-only transcript under a line saying how it ended (PXF-DEF-015); Restart gives it a
	// new shell.
	typeLine("exit 3")
	q.pumpUntil(t, "the exit report", func() bool { return first.done })
	if first.status != 3 || tm.body.Objects[0] == tm.view || !strings.HasSuffix(tm.pick.Selected, "(exited)") {
		t.Fatalf("after exit: status %d, picker %q", first.status, tm.pick.Selected)
	}
	if !slices.Contains(texts(tm.body), "The shell exited with status 3.") || findButton(tm.body, "Restart") == nil {
		t.Errorf("the exit line says %q", texts(tm.body))
	}
	q.pumpUntil(t, "the transcript", func() bool {
		var on bool
		_ = tm.sess.ExecLua(`return vim.b[...].pyxforge_transcript == true`, &on, first.buf)
		return on && gridCount(tm.sess.Grid(), "first-session") >= 1
	})
	oldBuf := first.buf
	tm.restart()
	if first.done || first.buf == oldBuf || tm.body.Objects[0] != tm.view || first.name != "Shell 1" {
		t.Fatalf("restart: %+v", first)
	}
	typeLine("echo restarted")
	q.pumpUntil(t, "the restarted shell", func() bool { return gridCount(tm.sess.Grid(), "restarted") >= 2 })

	// Rename keeps names unique.
	tm.rename(first, "build watch")
	tm.rename(second, "build watch")
	if first.name != "build watch" || second.name != "Shell 2" {
		t.Errorf("names %q, %q", first.name, second.name)
	}

	// Closing a session ends its shell and selects another.
	pid := shellPid(t, tm, second)
	tm.selectSession(second)
	tm.closeCurrent()
	if len(tm.sessions) != 1 || tm.current != first {
		t.Fatalf("after close: %d sessions, current %+v", len(tm.sessions), tm.current)
	}
	q.pumpUntil(t, "the closed shell to end", func() bool { return !proc.Alive(pid) })

	// Stop ends Neovim and with it the remaining shells.
	pid = shellPid(t, tm, first)
	sess := tm.sess
	tm.stop()
	q.pumpUntil(t, "Neovim to exit", sess.Exited)
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

// TestSessionPickerShowsWholeNames: the session picker takes the bar's free width, so a Run or
// Debug session's name is read whole even in a panel at the editor's narrowest, the bar never
// needs more than that panel, and hovering the picker shows the selected name (#28).
func TestSessionPickerShowsWholeNames(t *testing.T) {
	s, _ := newTestShell(t)
	tm := s.term
	s.tips.Delay = 0 // show at once
	w := test.NewWindow(container.NewStack(tm.frame, s.tips.Layer()))
	defer w.Close()
	w.Resize(fyne.NewSize(minEditorW, 200))
	th := fyne.CurrentApp().Settings().Theme()
	pad := th.Size(fyneTheme.SizeNameInnerPadding)
	icon := th.Size(fyneTheme.SizeNameInlineIcon)
	for _, name := range []string{"Debug count.c", "Run hello.c (exited)"} {
		tm.pick.Options = []string{"Shell 1", name}
		tm.pick.Selected = name
		tm.pick.Refresh()
		tm.bar.Refresh()
		text := fyne.MeasureText(name, th.Size(fyneTheme.SizeNameText), fyne.TextStyle{}).Width
		if room := tm.pick.Size().Width - 4*pad - icon; room < text {
			t.Errorf("%q needs %.0f, the picker has %.0f in a panel %d wide", name, text, room, minEditorW)
		}
		tm.pick.MouseIn(&desktop.MouseEvent{})
		if got := s.tips.Text(); got != name {
			t.Errorf("hovering the picker shows %q, want %q", got, name)
		}
		tm.pick.MouseOut()
		if got := s.tips.Text(); got != "" {
			t.Errorf("the tooltip %q stayed after the pointer left", got)
		}
	}
	if need := tm.bar.MinSize().Width; need > minEditorW {
		t.Errorf("the bar needs %.0f, more than the panel's %d", need, minEditorW)
	}
}
