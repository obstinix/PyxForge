package shell

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2/test"
	"github.com/obstinix/PyxForge/internal/workspace"
)

// TestStartupOpensOnlyTheSavedFiles looks for the 20 to 25 tabs reported at startup (#74) across
// restarts of the real editor: its Neovim, the workspace's own ShaDa file and the saved state, as
// the app keeps them. A session that had 25 files open and closed 24 reopens the one left; a
// state without files opens none, whatever the ShaDa remembers of the 25; a file opened at
// startup adds only itself. Each tab's reason is in the Log.
func TestStartupOpensOnlyTheSavedFiles(t *testing.T) {
	if _, err := exec.LookPath("nvim"); err != nil {
		t.Skip("nvim is not installed")
	}
	a := test.NewTempApp(t)
	root := t.TempDir()
	store := &workspace.StateStore{Dir: filepath.Join(t.TempDir(), "workspaces")}
	xdg := t.TempDir()
	var env []string
	for _, d := range []string{"CONFIG", "DATA", "STATE", "CACHE"} {
		env = append(env, "XDG_"+d+"_HOME="+filepath.Join(xdg, strings.ToLower(d)))
	}
	var files []string
	for i := 0; i < 25; i++ {
		p := filepath.Join(root, fmt.Sprintf("f%02d.c", i))
		if err := os.WriteFile(p, []byte(fmt.Sprintf("int f%02d;\n", i)), 0o644); err != nil {
			t.Fatal(err)
		}
		files = append(files, p)
	}
	start := func() (*Shell, queue) {
		q := make(queue, 4096)
		s := NewWithOptions(a, root, Options{Editor: true, Dispatch: q.post, State: store,
			NvimRuntime: filepath.Join(xdg, "runtime"), NvimEnv: env})
		noToastExpiry(s)
		t.Cleanup(func() { s.stopEditor(); s.term.stop() }) // a failed test still lets go of root
		return s, q
	}
	// fileTabs are the titles of the tabs that show a file; titles all of them, for messages.
	fileTabs := func(s *Shell) []string {
		var out []string
		if s.ed == nil {
			return nil
		}
		for _, it := range s.editors.Items {
			if bt := s.ed.byTab[it]; bt != nil && bt.path != "" {
				out = append(out, it.Text)
			}
		}
		return out
	}
	titles := func(s *Shell) []string {
		var out []string
		for _, it := range s.editors.Items {
			out = append(out, it.Text)
		}
		return out
	}
	stop := func(s *Shell) {
		s.saveState()
		s.stopEditor() // Neovim writes its ShaDa as it quits
		s.term.stop()
	}
	tabLines := func(s *Shell) []string {
		var out []string
		for _, l := range s.logLines {
			if i := strings.Index(l, "Tab "); i >= 0 {
				out = append(out, l[i:])
			}
		}
		return out
	}
	// settle pumps the UI queue for a while: any tab opened late shows up.
	settle := func(q queue) {
		for end := time.Now().Add(1500 * time.Millisecond); time.Now().Before(end); {
			select {
			case f := <-q:
				f()
			case <-time.After(20 * time.Millisecond):
			}
		}
	}

	shadaNames := func() []string {
		data, _ := os.ReadFile(store.ShadaFile(root))
		var names []string
		for _, f := range files {
			if strings.Contains(string(data), filepath.Base(f)) {
				names = append(names, filepath.Base(f))
			}
		}
		return names
	}

	// The first session ends with 25 tabs: the state lists them, and the ShaDa remembers them.
	s, q := start()
	for _, f := range files {
		s.OpenFile(f)
	}
	q.pumpUntil(t, "25 tabs", func() bool { return len(fileTabs(s)) == 25 })
	stop(s)
	if got := shadaNames(); len(got) != 25 {
		t.Fatalf("the ShaDa names %d of the 25 files open at exit (%v); the test would prove nothing", len(got), got)
	}

	// Restarted: the 25, each said to be restored, and nothing else.
	s, q = start()
	q.pumpUntil(t, "the 25 restored tabs", func() bool { return len(fileTabs(s)) == 25 })
	settle(q)
	if got := titles(s); len(got) != 25 {
		t.Fatalf("after a restart, %d tabs; want the 25 open at exit:\n%s", len(got), strings.Join(tabLines(s), "\n"))
	}
	for _, l := range tabLines(s) {
		if !strings.HasSuffix(l, ": restored from the last session") {
			t.Errorf("a tab not from the saved session: %q", l)
		}
	}
	// All but one closed: that one is what the next start opens.
	s.closeAllTabs()
	q.pumpUntil(t, "no file tabs", func() bool { return len(fileTabs(s)) == 0 })
	s.OpenFile(files[7])
	q.pumpUntil(t, "one file tab", func() bool { return len(fileTabs(s)) == 1 })
	stop(s)

	s, q = start()
	q.pumpUntil(t, "the restored tab", func() bool { return len(fileTabs(s)) == 1 })
	settle(q)
	if got := titles(s); len(got) != 1 || got[0] != "f07.c" {
		t.Fatalf("after closing 24 and restarting, tabs %q; want only f07.c. Log:\n%s", got, strings.Join(tabLines(s), "\n"))
	}
	if got := tabLines(s); len(got) != 1 || got[0] != "Tab f07.c: restored from the last session" {
		t.Errorf("the Log's tabs: %q", got)
	}
	// A file opened at startup, as from Explorer: itself and nothing more.
	s.OpenFile(files[3])
	q.pumpUntil(t, "the opened file", func() bool { return len(fileTabs(s)) == 2 })
	settle(q)
	if got := titles(s); len(got) != 2 {
		t.Fatalf("after opening one more file, tabs %q; want f07.c and f03.c", got)
	}
	stop(s)

	// A saved state with no files opens none, though the ShaDa still remembers all 25: Neovim
	// doesn't bring back a buffer list from it ('shada' has no %).
	if got := shadaNames(); len(got) != 25 {
		t.Logf("the ShaDa now names %d of the files", len(got))
	}
	if err := store.Save(workspace.State{Root: root}); err != nil {
		t.Fatal(err)
	}
	s, q = start()
	if !s.startEditor() {
		t.Fatal("the editor did not start")
	}
	settle(q)
	if got := fileTabs(s); len(got) != 0 {
		t.Fatalf("a state without files opened tabs %q:\n%s", titles(s), strings.Join(tabLines(s), "\n"))
	}
	var shada string
	if err := s.ed.sess.ExecLua(`return vim.o.shada`, &shada); err != nil || strings.Contains(shada, "%") {
		t.Errorf("'shada' is %q (%v): with %%, Neovim would restore a buffer list", shada, err)
	}
	stop(s)
}
