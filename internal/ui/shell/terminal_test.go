package shell

import (
	"slices"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"github.com/obstinix/PyxForge/internal/neovim"
	"github.com/obstinix/PyxForge/internal/proc"
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
	s.showDockTab(slices.Index(s.dock.Items, tm.tab))
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

	// A shell exits: the session reports its status; Restart gives it a new shell.
	typeLine("exit 3")
	q.pumpUntil(t, "the exit report", func() bool { return first.done })
	if first.status != 3 || tm.body.Objects[0] == tm.view || !strings.HasSuffix(tm.pick.Selected, "(exited)") {
		t.Fatalf("after exit: status %d, picker %q", first.status, tm.pick.Selected)
	}
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
