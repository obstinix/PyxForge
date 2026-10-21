package shell

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"fyne.io/fyne/v2/test"
	"github.com/obstinix/PyxForge/internal/workspace"
)

// TestBreakpointsAreKept: a breakpoint set with Toggle Breakpoint is reported to the shell,
// saved with the workspace, and shown again by the editor of the next window on the workspace.
func TestBreakpointsAreKept(t *testing.T) {
	if _, err := exec.LookPath("nvim"); err != nil {
		t.Skip("nvim is not installed")
	}
	root := t.TempDir()
	store := &workspace.StateStore{Dir: filepath.Join(t.TempDir(), "workspaces")}
	file := filepath.Join(root, "main.py")
	if err := os.WriteFile(file, []byte("a = 1\nb = 2\nprint(a + b)\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	xdg := t.TempDir()
	var env []string
	for _, d := range []string{"CONFIG", "DATA", "STATE", "CACHE"} {
		env = append(env, "XDG_"+d+"_HOME="+filepath.Join(xdg, strings.ToLower(d)))
	}
	open := func() (*Shell, queue) {
		q := make(queue, 1024)
		s := NewWithOptions(test.NewTempApp(t), root, Options{Editor: true, Dispatch: q.post,
			NvimRuntime: filepath.Join(xdg, "runtime"), NvimEnv: env, State: store})
		t.Cleanup(func() { s.stopEditor(); s.term.stop() })
		return s, q
	}

	s, q := open()
	s.OpenFile(file)
	q.pumpUntil(t, "the editor", func() bool { bt := s.activeBuffer(); return bt != nil && bt.path != "" })
	if !s.available("debug.toggleBreakpoint") {
		t.Error("Toggle Breakpoint is greyed out with a file open")
	}
	s.ed.sess.Input("3G") // queued in Neovim; Toggle Breakpoint is not, so wait for the cursor
	q.pumpUntil(t, "the cursor on line 3", func() bool {
		var line int
		return s.ed.sess.ExecLua(`return vim.api.nvim_win_get_cursor(0)[1]`, &line) == nil && line == 3
	})
	s.toggleBreakpoint()
	q.pumpUntil(t, "the breakpoint reported", func() bool {
		return len(s.breakpoints) == 1 && slices.Equal(s.breakpoints[0].Lines, []int{3})
	})
	if st := s.captureState(); len(st.Breakpoints) != 1 || st.Breakpoints[0].Lines[0] != 3 {
		t.Fatalf("saved breakpoints %+v", st.Breakpoints)
	}
	s.saveState()
	s.stopEditor()

	// The next window on the workspace.
	again, q2 := open()
	q2.pumpUntil(t, "the restored editor", func() bool { return again.ed != nil && again.ed.sess != nil && len(again.editors.Items) == 1 })
	q2.pumpUntil(t, "the breakpoint shown again", func() bool {
		list, err := again.ed.sess.Breakpoints()
		return err == nil && len(list) == 1 && slices.Equal(list[0].Lines, []int{3}) &&
			strings.EqualFold(filepath.Clean(list[0].File), filepath.Clean(file))
	})
}
