package git

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
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
