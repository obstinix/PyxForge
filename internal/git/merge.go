package git

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// Conflicts (PXF-DEF-030): a merge, rebase, patch series (am), cherry-pick or revert that met a
// conflict stops half done and waits for the user. Status lists the conflicted files; these say
// which operation is waiting, offer its prepared message, and give it up.

// gitPath is where a file in the repository's Git directory is (git rev-parse --git-path).
func (r *Repo) gitPaths(ctx context.Context, names ...string) ([]string, error) {
	args := []string{"rev-parse"}
	for _, n := range names {
		args = append(args, "--git-path", n)
	}
	out, err := r.run(ctx, nil, args...)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.TrimRight(out, "\r\n"), "\n")
	if len(lines) != len(names) {
		return nil, fmt.Errorf("git rev-parse --git-path: %d paths for %d names", len(lines), len(names))
	}
	for i, l := range lines {
		p := filepath.FromSlash(strings.TrimRight(l, "\r"))
		if !filepath.IsAbs(p) {
			p = filepath.Join(r.Root, p) // relative to the working directory, which is Root
		}
		lines[i] = p
	}
	return lines, nil
}

// InProgress names the operation Git stopped in the middle of: "merge", "rebase", "am",
// "cherry-pick" or "revert"; "" when there is none.
func (r *Repo) InProgress(ctx context.Context) (string, error) {
	paths, err := r.gitPaths(ctx, "MERGE_HEAD", "rebase-merge", "rebase-apply", "CHERRY_PICK_HEAD", "REVERT_HEAD")
	if err != nil {
		return "", err
	}
	exists := func(p string) bool { _, err := os.Stat(p); return err == nil }
	switch {
	case exists(paths[0]):
		return "merge", nil
	case exists(paths[1]):
		return "rebase", nil
	case exists(filepath.Join(paths[2], "applying")):
		return "am", nil
	case exists(paths[2]):
		return "rebase", nil
	case exists(paths[3]):
		return "cherry-pick", nil
	case exists(paths[4]):
		return "revert", nil
	}
	return "", nil
}

// MergeMessage is the commit message Git prepared for the operation in progress (MERGE_MSG),
// without its comment lines; "" when there is none.
func (r *Repo) MergeMessage(ctx context.Context) string {
	paths, err := r.gitPaths(ctx, "MERGE_MSG")
	if err != nil {
		return ""
	}
	data, err := os.ReadFile(paths[0])
	if err != nil {
		return ""
	}
	var keep []string
	for _, l := range strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n") {
		if !strings.HasPrefix(l, "#") {
			keep = append(keep, l)
		}
	}
	return strings.TrimSpace(strings.Join(keep, "\n"))
}

// Abort gives up the operation in progress (InProgress) and puts the branch, the index and the
// working tree back as they were before it started. Conflicts resolved so far are lost.
func (r *Repo) Abort(ctx context.Context, op string) error {
	switch op {
	case "merge", "rebase", "am", "cherry-pick", "revert":
	default:
		return fmt.Errorf("no operation %q to abort", op)
	}
	_, err := r.write(ctx, nil, op, "--abort")
	return err
}

// HasConflictMarkers reports whether a file still has the lines Git writes around a conflict:
// one starting with "<<<<<<< " or ">>>>>>> ".
func HasConflictMarkers(path string) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	for _, line := range bytes.Split(data, []byte("\n")) {
		if bytes.HasPrefix(line, []byte("<<<<<<< ")) || bytes.HasPrefix(line, []byte(">>>>>>> ")) {
			return true, nil
		}
	}
	return false, nil
}

// Combine brings the upstream's new commits into the current branch when both have moved on
// (#66): by a merge commit, or by replaying the branch's own commits on top of the upstream
// (rebase). A conflict leaves the operation in progress for the user to resolve, and reports
// conflicted; any other failure leaves nothing in progress.
func (r *Repo) Combine(ctx context.Context, rebase bool) (conflicted bool, err error) {
	args := []string{"merge", "--no-edit", "@{upstream}"}
	if rebase {
		args = []string{"rebase", "@{upstream}"}
	}
	return r.stepOperation(ctx, args...)
}

// Continue finishes an operation the user resolved the conflicts of: it commits a merge,
// cherry-pick or revert with the message Git prepared, and goes on with a rebase or a patch
// series, which may stop at the next conflict (conflicted).
func (r *Repo) Continue(ctx context.Context, op string) (conflicted bool, err error) {
	if st, err := r.Status(ctx); err != nil {
		return false, err
	} else if n := len(slices.DeleteFunc(slices.Clone(st.Files), func(f File) bool { return !f.Conflicted() })); n > 0 {
		return false, fmt.Errorf("%d conflicted files are not resolved yet: resolve each and stage it, then continue", n)
	}
	switch op {
	case "merge":
		return r.stepOperation(ctx, "commit", "--no-edit")
	case "rebase", "am", "cherry-pick", "revert":
		return r.stepOperation(ctx, op, "--continue")
	}
	return false, fmt.Errorf("no operation %q to continue", op)
}

// stepOperation runs a command that may stop at a conflict: one that stopped with an operation
// in progress and conflicted files is not an error.
func (r *Repo) stepOperation(ctx context.Context, args ...string) (bool, error) {
	_, err := r.write(ctx, nil, args...)
	if err == nil {
		return false, nil
	}
	if op, perr := r.InProgress(ctx); perr == nil && op != "" {
		if st, serr := r.Status(ctx); serr == nil && slices.ContainsFunc(st.Files, func(f File) bool { return f.Conflicted() }) {
			return true, nil
		}
	}
	return false, err
}
