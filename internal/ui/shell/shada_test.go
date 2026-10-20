package shell

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"fyne.io/fyne/v2/test"
	"github.com/obstinix/PyxForge/internal/workspace"
)

// TestEditorHistoryIsPerWorkspace: the editor's Neovim keeps its history in the workspace's own
// ShaDa file, so its jumplist and old files hold this project's files only (PXF-DEF-001).
func TestEditorHistoryIsPerWorkspace(t *testing.T) {
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
	q := make(queue, 1024)
	s := NewWithOptions(a, root, Options{Editor: true, Dispatch: q.post, NvimRuntime: filepath.Join(xdg, "runtime"), NvimEnv: env, State: store})
	t.Cleanup(func() { s.stopEditor(); s.term.stop() })
	f := filepath.Join(root, "a.txt")
	if err := os.WriteFile(f, []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	s.OpenFile(f)
	q.pumpUntil(t, "the editor", func() bool { return s.ed != nil && s.ed.sess != nil && len(s.editors.Items) == 1 })
	var got string
	if err := s.ed.sess.ExecLua(`return vim.o.shadafile`, &got); err != nil {
		t.Fatal(err)
	}
	if want := store.ShadaFile(root); !strings.EqualFold(filepath.Clean(got), filepath.Clean(want)) {
		t.Errorf("the editor's ShaDa file is %q, want the workspace's %q", got, want)
	}
}
