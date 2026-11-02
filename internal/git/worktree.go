package git

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
)

// Worktrees (#68): one repository checked out in several folders at once, each on its own
// branch, so a second line of work needs no stash or second clone.

// Worktree is a folder the repository is checked out in.
type Worktree struct {
	Path     string // absolute
	Branch   string // short name; "" when HEAD is detached
	Head     string // the commit checked out
	Main     bool   // the repository's own working tree, which cannot be removed
	Locked   bool
	Prunable bool // its folder is gone; Prune forgets it
}

// Worktrees lists the repository's worktrees, its own first.
func (r *Repo) Worktrees(ctx context.Context) ([]Worktree, error) {
	out, err := r.run(ctx, nil, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, err
	}
	var list []Worktree
	var cur *Worktree
	for _, l := range strings.Split(strings.ReplaceAll(out, "\r\n", "\n"), "\n") {
		key, val, _ := strings.Cut(l, " ")
		switch key {
		case "worktree":
			list = append(list, Worktree{Path: filepath.FromSlash(val), Main: len(list) == 0})
			cur = &list[len(list)-1]
		case "HEAD":
			if cur != nil {
				cur.Head = val
			}
		case "branch":
			if cur != nil {
				cur.Branch = strings.TrimPrefix(val, "refs/heads/")
			}
		case "locked":
			if cur != nil {
				cur.Locked = true
			}
		case "prunable":
			if cur != nil {
				cur.Prunable = true
			}
		}
	}
	return list, nil
}

// AddWorktree checks out branch in a new folder at path: an existing branch, or with create a
// new one made from HEAD.
func (r *Repo) AddWorktree(ctx context.Context, path, branch string, create bool) error {
	switch {
	case strings.TrimSpace(branch) == "":
		return errors.New("name the branch to check out")
	case strings.HasPrefix(branch, "-"):
		return errors.New("a branch's name cannot start with -")
	case !filepath.IsAbs(path):
		return errors.New("the worktree's folder must be a full path")
	}
	args := []string{"worktree", "add", "--quiet"}
	if create {
		args = append(args, "-b", branch, "--", path)
	} else {
		args = append(args, "--", path, branch)
	}
	_, err := r.write(ctx, nil, args...)
	return err
}

// RemoveWorktree deletes a worktree's folder and forgets it. Git refuses while it has changes
// not committed; PyxForge never forces that.
func (r *Repo) RemoveWorktree(ctx context.Context, path string) error {
	_, err := r.write(ctx, nil, "worktree", "remove", "--", path)
	return err
}

// PruneWorktrees forgets worktrees whose folders were deleted by hand.
func (r *Repo) PruneWorktrees(ctx context.Context) error {
	_, err := r.write(ctx, nil, "worktree", "prune")
	return err
}
