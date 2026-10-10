package shell

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"fyne.io/fyne/v2/test"
)

// gitInit makes root a repository with a local identity, so commits work on any machine.
func gitInit(t *testing.T, root string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	for _, args := range [][]string{{"init", "--quiet"}, {"config", "user.name", "PyxForge Test"},
		{"config", "user.email", "test@example.invalid"}, {"config", "commit.gpgsign", "false"}, {"config", "core.autocrlf", "false"},
		{"symbolic-ref", "HEAD", "refs/heads/main"}} {
		if out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
}

func headings(g *gitPanel) []string {
	var out []string
	for _, r := range g.rows {
		if r.heading != "" {
			out = append(out, r.heading)
		}
	}
	return out
}

func TestGitPanel(t *testing.T) {
	a := test.NewTempApp(t)
	root := t.TempDir()
	q := make(queue, 1024)
	s := NewWithOptions(a, root, Options{Dispatch: q.post})
	g := s.gitp
	gitTab := slices.Index(s.dock.Items, g.tab)

	// Not a repository yet: the tab says how to start one.
	s.showDockTab(gitTab)
	q.pumpUntil(t, "the first status", func() bool { return !g.running })
	if g.loaded || s.statusBranch.text.Text != "no repository" {
		t.Fatalf("outside a repository: loaded %v, status %q", g.loaded, s.statusBranch.text.Text)
	}

	gitInit(t, root)
	if err := os.WriteFile(filepath.Join(root, "boot.asm"), []byte("org 0x7c00\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	g.refresh()
	q.pumpUntil(t, "the untracked file", func() bool { return g.loaded && !g.running })
	if got := headings(g); !slices.Equal(got, []string{"Untracked (1)"}) {
		t.Fatalf("groups %v", got)
	}
	if !g.commit.Disabled() || s.statusBranch.text.Text != "main · 1 changed" || !strings.Contains(g.branch.Text, "no commits yet") {
		t.Errorf("commit enabled %v, status %q, branch %q", !g.commit.Disabled(), s.statusBranch.text.Text, g.branch.Text)
	}

	// Stage it with the row's button, then commit.
	g.apply("stage", false, []string{"boot.asm"})
	q.pumpUntil(t, "the staged file", func() bool { return !g.running && slices.Equal(headings(g), []string{"Staged (1)"}) })
	if g.commit.Disabled() || g.commit.Text != "Commit 1 file" {
		t.Errorf("commit button %q, disabled %v", g.commit.Text, g.commit.Disabled())
	}
	g.message.SetText("Add the boot sector")
	g.doCommit()
	q.pumpUntil(t, "a clean tree", func() bool {
		return !g.running && slices.Equal(headings(g), []string{"No changes"}) && len(g.history.Objects) == 1
	})
	if g.message.Text != "" || s.statusBranch.text.Text != "main" {
		t.Errorf("after commit: message %q, status %q", g.message.Text, s.statusBranch.text.Text)
	}

	// An edit shows under Changes; Stage All and Unstage All move it back and forth.
	if err := os.WriteFile(filepath.Join(root, "boot.asm"), []byte("org 0x7c00\nbits 16\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	g.refresh()
	q.pumpUntil(t, "the change", func() bool { return !g.running && slices.Equal(headings(g), []string{"Changes (1)"}) })
	g.apply("stage", true, nil)
	q.pumpUntil(t, "all staged", func() bool { return !g.running && slices.Equal(headings(g), []string{"Staged (1)"}) })
	g.apply("unstage", true, nil)
	q.pumpUntil(t, "all unstaged", func() bool { return !g.running && slices.Equal(headings(g), []string{"Changes (1)"}) })
}

// TestGitDiffOpensInTheEditor checks the diff against HEAD with a real Neovim.
func TestGitDiffOpensInTheEditor(t *testing.T) {
	s, root, q := newEditorShell(t)
	gitInit(t, root)
	boot := filepath.Join(root, "boot.asm")
	if err := os.WriteFile(boot, []byte("org 0x7c00\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"add", "."}, {"commit", "--quiet", "-m", "first"}} {
		if out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	if err := os.WriteFile(boot, []byte("org 0x7c00\nbits 16\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	g := s.gitp
	t.Cleanup(func() {
		if t.Failed() {
			t.Logf("log:\n%s", strings.Join(s.logLines, "\n"))
		}
	})
	g.refresh()
	q.pumpUntil(t, "the change", func() bool { return g.loaded && !g.running && len(g.rows) == 2 })
	g.list.Select(1)
	q.pumpUntil(t, "the file's tab", func() bool { return len(s.editors.Items) == 1 && s.editors.Selected().Text == "boot.asm" })
	var wins int
	q.pumpUntil(t, "two windows in diff mode", func() bool {
		_ = s.ed.sess.ExecLua(`local n = 0
for _, w in ipairs(vim.api.nvim_list_wins()) do if vim.wo[w].diff then n = n + 1 end end
return n`, &wins)
		return wins == 2
	})
}

func gitRun(t *testing.T, root string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// TestGitBranchesAndStashes goes through the branch and stash workflow of the Git tab.
func TestGitBranchesAndStashes(t *testing.T) {
	a := test.NewTempApp(t)
	root := t.TempDir()
	gitInit(t, root)
	boot := filepath.Join(root, "boot.asm")
	if err := os.WriteFile(boot, []byte("org 0x7c00\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, root, "add", ".")
	gitRun(t, root, "commit", "--quiet", "-m", "first")
	q := make(queue, 1024)
	s := NewWithOptions(a, root, Options{Dispatch: q.post})
	g := s.gitp
	g.refresh()
	q.pumpUntil(t, "the status", func() bool { return g.loaded && !g.running })
	if g.branchBtn.text.Text != "main ▾" {
		t.Errorf("branch button %q", g.branchBtn.text.Text)
	}

	// Create and switch to a branch; the status bar follows.
	g.createBranchNamed("feature/vga", true)
	q.pumpUntil(t, "the new branch", func() bool { return !g.running && g.status.Branch == "feature/vga" })
	if !strings.HasPrefix(s.statusBranch.text.Text, "feature/vga") {
		t.Errorf("status bar %q", s.statusBranch.text.Text)
	}
	g.createBranchNamed("bad name", false)
	q.pumpUntil(t, "the refusal", func() bool {
		return !g.running && slices.ContainsFunc(s.logLines, func(l string) bool { return strings.Contains(l, "not a valid branch name") })
	})

	// A commit on the branch, then a dirty tree: switching asks first.
	if err := os.WriteFile(boot, []byte("org 0x7c00\nbits 16\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, root, "commit", "--quiet", "-am", "16-bit")
	if err := os.WriteFile(boot, []byte("org 0x7c00\nbits 16\n; wip\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	g.refresh()
	q.pumpUntil(t, "the dirty status", func() bool { return !g.running && len(g.status.Files) == 1 })
	s.lastDialog = nil
	g.switchBranch("main")
	if s.lastDialog == nil {
		t.Fatal("switching with uncommitted changes did not ask")
	}
	s.lastDialog.Hide()

	// Switch alone is refused by Git (the change conflicts) and nothing moves.
	g.doSwitch("main", false)
	q.pumpUntil(t, "the refused switch", func() bool {
		return !g.running && slices.ContainsFunc(s.logLines, func(l string) bool { return strings.Contains(l, "would be overwritten") })
	})
	if g.status.Branch != "feature/vga" {
		t.Errorf("a refused switch moved to %q", g.status.Branch)
	}

	// Stash and Switch saves the change, then switches.
	g.doSwitch("main", true)
	q.pumpUntil(t, "main with a stash", func() bool { return !g.running && g.status.Branch == "main" && len(g.stashes) == 1 })
	if data, _ := os.ReadFile(boot); string(data) != "org 0x7c00\n" {
		t.Errorf("main's file: %q", data)
	}
	if len(g.stashBox.Objects) != 2 { // the title and one entry
		t.Errorf("stash list shows %d rows", len(g.stashBox.Objects))
	}

	// Back on the branch, pop restores the change and removes the entry.
	g.doSwitch("feature/vga", false)
	q.pumpUntil(t, "the branch again", func() bool { return !g.running && g.status.Branch == "feature/vga" })
	g.applyStash("stash@{0}", true)
	q.pumpUntil(t, "the popped change", func() bool { return !g.running && len(g.stashes) == 0 && len(g.status.Files) == 1 })

	// Stash again, then drop it after the confirmation.
	g.stashNamed("throwaway", true)
	q.pumpUntil(t, "the stash", func() bool { return !g.running && len(g.stashes) == 1 })
	s.lastDialog = nil
	g.dropStash(g.stashes[0])
	if s.lastDialog == nil {
		t.Fatal("drop did not ask")
	}
	s.lastDialog.Hide()
	if got := gitRun(t, root, "stash", "list"); got == "" {
		t.Fatal("the stash was dropped before confirming")
	}

	// Deleting an unmerged branch asks a second time.
	g.doSwitch("main", false)
	q.pumpUntil(t, "main", func() bool { return !g.running && g.status.Branch == "main" })
	s.lastDialog = nil
	g.deleteBranchNamed("feature/vga", false)
	q.pumpUntil(t, "the not-merged question", func() bool { return !g.running && s.lastDialog != nil })
	s.lastDialog.Hide()
	g.deleteBranchNamed("feature/vga", true)
	q.pumpUntil(t, "the deletion", func() bool { return !g.running && !strings.Contains(gitRun(t, root, "branch"), "feature/vga") })
}
