package git

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestWorktrees: a worktree is added on a new branch and on an existing one, listed with its
// branch, refused while it has uncommitted changes, removed when clean, and a folder deleted by
// hand is pruned (#68).
func TestWorktrees(t *testing.T) {
	isolateConfig(t)
	ctx := context.Background()
	r := newRepo(t)
	write(t, r, "boot.asm", "org 0x7c00\n")
	commitAll(t, r, "one")
	gitIn(t, r, "branch", "existing")
	base := t.TempDir()
	feature, existing := filepath.Join(base, "feature work"), filepath.Join(base, "existing")
	if err := r.AddWorktree(ctx, feature, "feature", true); err != nil {
		t.Fatal(err)
	}
	if err := r.AddWorktree(ctx, existing, "existing", false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(feature, "boot.asm")); err != nil {
		t.Errorf("the new worktree has no files: %v", err)
	}
	list, err := r.Worktrees(ctx)
	if err != nil {
		t.Fatal(err)
	}
	byBranch := map[string]Worktree{} // Git lists the others by path
	for _, w := range list {
		byBranch[w.Branch] = w
	}
	if len(list) != 3 || !list[0].Main || list[0].Branch != "main" || byBranch["feature"].Head == "" || byBranch["existing"].Main {
		t.Fatalf("worktrees: %+v", list)
	}
	if !sameDir(byBranch["feature"].Path, feature) {
		t.Errorf("listed at %q, made at %q", byBranch["feature"].Path, feature)
	}

	for _, bad := range []struct{ path, branch string }{{filepath.Join(base, "x"), ""}, {filepath.Join(base, "y"), "-f"}, {"relative", "z"}} {
		if err := r.AddWorktree(ctx, bad.path, bad.branch, true); err == nil {
			t.Errorf("AddWorktree(%q, %q) was accepted", bad.path, bad.branch)
		}
	}
	if err := r.AddWorktree(ctx, filepath.Join(base, "again"), "existing", false); err == nil {
		t.Error("a branch checked out in another worktree was checked out again")
	}

	if err := os.WriteFile(filepath.Join(feature, "boot.asm"), []byte("changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := r.RemoveWorktree(ctx, feature); err == nil || !strings.Contains(err.Error(), "modified") {
		t.Errorf("a worktree with changes was removed: %v", err)
	}
	if err := r.RemoveWorktree(ctx, existing); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(existing); !os.IsNotExist(err) {
		t.Errorf("the removed worktree's folder is still there: %v", err)
	}

	if err := os.RemoveAll(feature); err != nil {
		t.Fatal(err)
	}
	if list, _ := r.Worktrees(ctx); len(list) != 2 || !list[1].Prunable || list[1].Branch != "feature" {
		t.Errorf("a deleted folder is not prunable: %+v", list)
	}
	if err := r.PruneWorktrees(ctx); err != nil {
		t.Fatal(err)
	}
	if list, _ := r.Worktrees(ctx); len(list) != 1 {
		t.Errorf("after prune: %+v", list)
	}
}

func sameDir(a, b string) bool {
	ra, _ := filepath.EvalSymlinks(a)
	rb, _ := filepath.EvalSymlinks(b)
	return strings.EqualFold(filepath.Clean(ra), filepath.Clean(rb))
}
