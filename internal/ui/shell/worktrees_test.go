package shell

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"fyne.io/fyne/v2/test"

	"github.com/obstinix/PyxForge/internal/git"
)

// TestGitWorktrees: a worktree is created on a new branch from the Git tab, listed in the
// palette beside this window's, and removed after asking (#68).
func TestGitWorktrees(t *testing.T) {
	a := test.NewTempApp(t)
	root := filepath.Join(t.TempDir(), "kernel")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	gitInit(t, root)
	if err := os.WriteFile(filepath.Join(root, "boot.asm"), []byte("org 0x7c00\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, root, "add", ".")
	gitRun(t, root, "commit", "--quiet", "-m", "first")

	q := make(queue, 1024)
	s := NewWithOptions(a, root, Options{Dispatch: q.post})
	g := s.gitp
	g.refresh()
	q.pumpUntil(t, "the repository", func() bool { return g.repo != nil && g.loaded && !g.running })

	// The form suggests a folder beside the repository, named after it and the branch.
	g.createWorktree()
	if s.lastDialog == nil {
		t.Fatal("Create Worktree did not ask")
	}
	s.lastDialog.Hide()
	path := filepath.Join(filepath.Dir(root), "kernel-feature")
	g.addWorktree(path, "feature", true)
	q.pumpUntil(t, "the worktree", func() bool { return !g.running && findButton(s.win.Canvas().Overlays().Top(), "Open Here") != nil })
	tapInDialog(t, s, "Cancel")
	if _, err := os.Stat(filepath.Join(path, "boot.asm")); err != nil {
		t.Fatalf("the worktree has no files: %v", err)
	}

	titles := func() []string {
		var out []string
		for _, it := range s.palette.Results() {
			out = append(out, it.Title+" | "+it.Detail)
		}
		return out
	}
	g.worktrees()
	q.pumpUntil(t, "the list", func() bool { return s.palette.Visible() && len(s.palette.Results()) == 3 })
	got := titles()
	if !strings.HasPrefix(got[0], "Create Worktree…") || !slices.ContainsFunc(got, func(l string) bool { return strings.HasPrefix(l, "feature | ") }) ||
		!slices.ContainsFunc(got, func(l string) bool { return strings.HasSuffix(l, "· this window") }) {
		t.Errorf("the list: %q", got)
	}
	s.palette.Hide()

	list, err := g.repo.Worktrees(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	i := slices.IndexFunc(list, func(w git.Worktree) bool { return w.Branch == "feature" })
	g.worktreeChosen(list[i])
	if findButton(s.win.Canvas().Overlays().Top(), "Open Here") == nil {
		t.Fatal("no Open Here")
	}
	tapInDialog(t, s, "Remove…")
	tapInDialog(t, s, "Remove")
	q.pumpUntil(t, "the removal", func() bool { return !g.running && logged(s, "Removed the worktree kernel-feature") })
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("the worktree's folder is still there: %v", err)
	}
}
