package git

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// Branch is a local branch.
type Branch struct {
	Name     string
	Current  bool
	Upstream string // "origin/main", or "" when it tracks nothing
	Head     string // short hash of its last commit
	Subject  string // its last commit's subject
}

// Branches lists the local branches, read with for-each-ref's NUL-separated format rather
// than from `git branch`'s display.
func (r *Repo) Branches(ctx context.Context) ([]Branch, error) {
	out, err := r.run(ctx, nil, "for-each-ref", "--sort=refname",
		"--format=%(refname:short)%00%(HEAD)%00%(upstream:short)%00%(objectname:short)%00%(contents:subject)", "refs/heads")
	if err != nil {
		return nil, err
	}
	var list []Branch
	for line := range strings.SplitSeq(strings.TrimRight(out, "\n"), "\n") {
		f := strings.Split(line, "\x00")
		if len(f) < 5 || f[0] == "" {
			continue
		}
		list = append(list, Branch{Name: f[0], Current: f[1] == "*", Upstream: f[2], Head: f[3], Subject: f[4]})
	}
	return list, nil
}

// CheckBranchName reports why name cannot be a branch name, using Git's own rules.
func (r *Repo) CheckBranchName(ctx context.Context, name string) error {
	if name == "" || strings.HasPrefix(name, "-") {
		return fmt.Errorf("%q is not a valid branch name", name)
	}
	if _, err := r.run(ctx, nil, "check-ref-format", "--branch", name); err != nil {
		return fmt.Errorf("%q is not a valid branch name", name)
	}
	return nil
}

// CreateBranch makes a branch at HEAD, and switches to it with switchTo.
func (r *Repo) CreateBranch(ctx context.Context, name string, switchTo bool) error {
	if err := r.CheckBranchName(ctx, name); err != nil {
		return err
	}
	if switchTo {
		_, err := r.run(ctx, nil, "switch", "--create", name)
		return err
	}
	_, err := r.run(ctx, nil, "branch", name)
	return err
}

// Switch checks out a branch. Git refuses, and nothing changes, when local changes would be
// overwritten; the error carries its explanation and the files involved.
func (r *Repo) Switch(ctx context.Context, name string) error {
	if err := r.CheckBranchName(ctx, name); err != nil {
		return err
	}
	_, err := r.run(ctx, nil, "switch", "--no-guess", name)
	return err
}

// ErrNotMerged means a branch has commits no other branch has; deleting it needs force.
var ErrNotMerged = errors.New("the branch is not fully merged")

// DeleteBranch removes a local branch. Without force Git refuses a branch whose commits
// would be lost, reported as ErrNotMerged.
func (r *Repo) DeleteBranch(ctx context.Context, name string, force bool) error {
	if err := r.CheckBranchName(ctx, name); err != nil {
		return err
	}
	flag := "--delete"
	if force {
		flag = "-D"
	}
	_, err := r.run(ctx, nil, "branch", flag, name)
	if err != nil && strings.Contains(err.Error(), "not fully merged") {
		return fmt.Errorf("%w: %s", ErrNotMerged, name)
	}
	return err
}

// Stash is one entry of the stash list.
type Stash struct {
	Ref     string // "stash@{0}"
	Message string // "On main: work in progress"
	Date    string // ISO 8601
}

var stashRef = regexp.MustCompile(`^stash@\{[0-9]+\}$`)

// Stashes lists the stash, newest first.
func (r *Repo) Stashes(ctx context.Context) ([]Stash, error) {
	out, err := r.run(ctx, nil, "stash", "list", "--format=%gd%x00%gs%x00%cI")
	if err != nil {
		return nil, err
	}
	var list []Stash
	for line := range strings.SplitSeq(strings.TrimRight(out, "\n"), "\n") {
		f := strings.Split(line, "\x00")
		if len(f) == 3 && stashRef.MatchString(f[0]) {
			list = append(list, Stash{Ref: f[0], Message: f[1], Date: f[2]})
		}
	}
	return list, nil
}

// ErrNothingToStash means the working tree has no changes to save.
var ErrNothingToStash = errors.New("no local changes to stash")

// StashPush saves the working tree's changes (untracked files too with untracked) and
// cleans it.
func (r *Repo) StashPush(ctx context.Context, message string, untracked bool) error {
	before, err := r.Stashes(ctx)
	if err != nil {
		return err
	}
	args := []string{"stash", "push"}
	if untracked {
		args = append(args, "--include-untracked")
	}
	if message = strings.TrimSpace(message); message != "" {
		args = append(args, "--message", message)
	}
	if _, err := r.run(ctx, nil, args...); err != nil {
		return err
	}
	after, err := r.Stashes(ctx)
	if err == nil && len(after) == len(before) {
		return ErrNothingToStash // git says "No local changes to save" and succeeds
	}
	return err
}

// StashApply restores a stash entry; with pop it is also removed, unless applying conflicts,
// in which case Git keeps it and the error explains.
func (r *Repo) StashApply(ctx context.Context, ref string, pop bool) error {
	if !stashRef.MatchString(ref) {
		return fmt.Errorf("%q is not a stash entry", ref)
	}
	op := "apply"
	if pop {
		op = "pop"
	}
	_, err := r.run(ctx, nil, "stash", op, ref)
	return err
}

// StashDrop deletes a stash entry.
func (r *Repo) StashDrop(ctx context.Context, ref string) error {
	if !stashRef.MatchString(ref) {
		return fmt.Errorf("%q is not a stash entry", ref)
	}
	_, err := r.run(ctx, nil, "stash", "drop", ref)
	return err
}
