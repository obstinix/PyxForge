package shell

import (
	"slices"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"github.com/obstinix/PyxForge/internal/neovim"
)

func gridCount(g neovim.Snapshot, want string) int {
	n := 0
	rows, _ := g.Size()
	for r := range rows {
		n += strings.Count(g.Line(r), want)
	}
	return n
}

// TestTerminalPanelLifecycle opens the Terminal tab, runs a command through the view, and
// checks that an exiting shell is reported with its status and can be restarted.
func TestTerminalPanelLifecycle(t *testing.T) {
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

	s.showDockTab(slices.Index(s.dock.Items, tm.tab))
	if tm.sess == nil || tm.body.Objects[0] != tm.view {
		t.Fatal("opening the Terminal tab did not start a shell in the panel")
	}
	if s.active != regionPanel {
		t.Error("the panel is not the active region")
	}

	typeLine := func(line string) {
		for _, r := range line {
			tm.view.TypedRune(r)
		}
		tm.view.TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})
	}
	typeLine("echo pyx-term-ok")
	q.pumpUntil(t, "the command's output", func() bool { return gridCount(tm.sess.Grid(), "pyx-term-ok") >= 2 })

	// The shell exits: the panel says so, with the status, and offers Restart.
	typeLine("exit 3")
	q.pumpUntil(t, "the exit report", func() bool { return tm.done })
	if tm.body.Objects[0] == tm.view {
		t.Fatal("the panel still shows the finished terminal")
	}
	if !slices.ContainsFunc(s.logLines, func(l string) bool { return strings.Contains(l, "status 3") }) {
		t.Errorf("exit status not logged: %q", s.logLines)
	}

	// Restart opens a new shell in the same Neovim.
	first := tm.buf
	tm.restart()
	if tm.done || tm.buf == first || tm.body.Objects[0] != tm.view {
		t.Fatalf("restart did not start a new shell (buffer %d → %d, done %v)", first, tm.buf, tm.done)
	}
	typeLine("echo second-shell")
	q.pumpUntil(t, "the new shell's output", func() bool { return gridCount(tm.sess.Grid(), "second-shell") >= 2 })

	// Stopping ends the process; a later exit event is not mistaken for the shell's.
	sess := tm.sess
	tm.stop()
	q.pumpUntil(t, "Neovim to exit", sess.Exited)
}
