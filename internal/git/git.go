// Package git reads a working tree's status and runs the local Git operations the desktop
// app offers: stage, unstage, commit, and the committed text of a file for diffs. It drives
// the git command line, never over the network.
package git

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Repo is a Git working tree.
type Repo struct {
	Root string // the top level of the working tree
	Git  string // the git executable; empty means "git" on PATH
}

// ErrNoRepo means the folder is not inside a Git working tree.
var ErrNoRepo = errors.New("not a Git repository")

// Open finds the working tree that dir belongs to.
func Open(ctx context.Context, dir string) (*Repo, error) {
	r := &Repo{Root: dir}
	out, err := r.run(ctx, nil, "rev-parse", "--show-toplevel")
	if err != nil {
		if strings.Contains(err.Error(), "not a git repository") {
			return nil, ErrNoRepo
		}
		return nil, err
	}
	r.Root = filepath.Clean(strings.TrimSpace(out))
	return r, nil
}

// File is one changed path. Index and Work are porcelain status letters: M modified,
// A added, D deleted, R renamed, C copied, T type changed, U unmerged, ? untracked, . none.
type File struct {
	Path     string // slash-separated, relative to Root
	OrigPath string // the source of a rename or copy
	Index    byte   // staged change
	Work     byte   // change in the working tree not yet staged
}

// Staged reports whether the file has a change in the index.
func (f File) Staged() bool { return f.Index != '.' && f.Index != '?' && f.Index != 0 }

// Unstaged reports whether the working tree differs from the index (untracked included).
func (f File) Unstaged() bool { return f.Work != '.' && f.Work != 0 }

// Untracked reports whether Git does not track the file yet.
func (f File) Untracked() bool { return f.Index == '?' }

// Conflicted reports an unmerged path.
func (f File) Conflicted() bool { return f.Index == 'U' || f.Work == 'U' }

// Status is the branch and the changed files.
type Status struct {
	Branch   string // "" when HEAD is detached
	Head     string // the commit HEAD names; "" before the first commit
	Upstream string
	Ahead    int
	Behind   int
	Files    []File
}

// Status reads `git status --porcelain=v2`.
func (r *Repo) Status(ctx context.Context) (Status, error) {
	out, err := r.run(ctx, nil, "status", "--porcelain=v2", "--branch", "-z", "--untracked-files=all")
	if err != nil {
		return Status{}, err
	}
	return parseStatus(out), nil
}

func parseStatus(out string) Status {
	var st Status
	recs := strings.Split(out, "\x00")
	for i := 0; i < len(recs); i++ {
		rec := recs[i]
		switch {
		case strings.HasPrefix(rec, "# branch.oid "):
			if oid := strings.TrimPrefix(rec, "# branch.oid "); oid != "(initial)" {
				st.Head = oid
			}
		case strings.HasPrefix(rec, "# branch.head "):
			if h := strings.TrimPrefix(rec, "# branch.head "); h != "(detached)" {
				st.Branch = h
			}
		case strings.HasPrefix(rec, "# branch.upstream "):
			st.Upstream = strings.TrimPrefix(rec, "# branch.upstream ")
		case strings.HasPrefix(rec, "# branch.ab "):
			var a, b int
			if _, err := fmt.Sscanf(strings.TrimPrefix(rec, "# branch.ab "), "+%d -%d", &a, &b); err == nil {
				st.Ahead, st.Behind = a, b
			}
		case strings.HasPrefix(rec, "1 "): // 1 XY sub mH mI mW hH hI path
			f := strings.SplitN(rec, " ", 9)
			if len(f) == 9 {
				st.Files = append(st.Files, File{Path: f[8], Index: f[1][0], Work: f[1][1]})
			}
		case strings.HasPrefix(rec, "2 "): // 2 XY sub mH mI mW hH hI Xscore path, then origPath
			f := strings.SplitN(rec, " ", 10)
			if len(f) == 10 {
				file := File{Path: f[9], Index: f[1][0], Work: f[1][1]}
				if i+1 < len(recs) {
					i++
					file.OrigPath = recs[i]
				}
				st.Files = append(st.Files, file)
			}
		case strings.HasPrefix(rec, "u "): // u XY sub m1 m2 m3 mW h1 h2 h3 path
			f := strings.SplitN(rec, " ", 11)
			if len(f) == 11 {
				st.Files = append(st.Files, File{Path: f[10], Index: 'U', Work: 'U'})
			}
		case strings.HasPrefix(rec, "? "):
			st.Files = append(st.Files, File{Path: rec[2:], Index: '?', Work: '?'})
		}
	}
	return st
}

// Stage adds paths (relative to Root) to the index, deletions included.
func (r *Repo) Stage(ctx context.Context, paths ...string) error {
	_, err := r.run(ctx, nil, append([]string{"add", "--all", "--"}, paths...)...)
	return err
}

// Unstage removes paths' staged changes, keeping the working tree as it is.
func (r *Repo) Unstage(ctx context.Context, paths ...string) error {
	if _, err := r.run(ctx, nil, "rev-parse", "--verify", "--quiet", "HEAD"); err != nil {
		// Before the first commit there is nothing to restore from: drop the entries.
		_, err := r.run(ctx, nil, append([]string{"rm", "--cached", "--quiet", "-r", "--"}, paths...)...)
		return err
	}
	_, err := r.run(ctx, nil, append([]string{"restore", "--staged", "--"}, paths...)...)
	return err
}

// Commit records the index with message and returns the new commit's short hash.
func (r *Repo) Commit(ctx context.Context, message string) (string, error) {
	if strings.TrimSpace(message) == "" {
		return "", errors.New("write a commit message first")
	}
	if _, err := r.run(ctx, strings.NewReader(message), "commit", "--file=-", "--cleanup=strip"); err != nil {
		return "", err
	}
	out, err := r.run(ctx, nil, "rev-parse", "--short", "HEAD")
	return strings.TrimSpace(out), err
}

// Show returns a file's text at a revision ("HEAD", or ":" for the index). A path the
// revision does not have gives empty text and no error, so new files diff against nothing.
func (r *Repo) Show(ctx context.Context, rev, path string) (string, error) {
	spec := rev + ":" + path
	if rev == ":" {
		spec = ":" + path
	}
	out, err := r.run(ctx, nil, "show", spec)
	if err != nil && (strings.Contains(err.Error(), "does not exist") || strings.Contains(err.Error(), "exists on disk, but not in") ||
		strings.Contains(err.Error(), "invalid object name")) {
		return "", nil
	}
	return out, err
}

// Log returns the last n commits as "hash subject" lines.
func (r *Repo) Log(ctx context.Context, n int) ([]string, error) {
	out, err := r.run(ctx, nil, "log", "-n", strconv.Itoa(n), "--format=%h %s")
	if err != nil {
		if strings.Contains(err.Error(), "does not have any commits") {
			return nil, nil
		}
		return nil, err
	}
	return strings.Split(strings.TrimRight(out, "\n"), "\n"), nil
}

type reader interface{ Read([]byte) (int, error) }

// run runs git in Root with prompts and pagers off, and returns its standard output. A
// failure carries git's own message.
func (r *Repo) run(ctx context.Context, stdin reader, args ...string) (string, error) {
	exe := r.Git
	if exe == "" {
		exe = "git"
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	// core.fsmonitor names a program Git runs during status: a repository's own config could
	// make merely opening it run code, so PyxForge never lets Git start one.
	cmd := exec.CommandContext(ctx, exe, append([]string{"-C", r.Root, "-c", "core.quotepath=off", "-c", "color.ui=false",
		"-c", "core.fsmonitor=false"}, args...)...)
	cmd.Env = append(cmd.Environ(), "GIT_TERMINAL_PROMPT=0", "GIT_PAGER=cat", "LC_ALL=C", "GIT_OPTIONAL_LOCKS=0")
	if stdin != nil {
		cmd.Stdin = stdin
	}
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return "", errors.New("git is not installed or not on PATH")
		}
		msg := strings.TrimSpace(errb.String())
		if msg == "" {
			msg = err.Error()
		}
		return out.String(), fmt.Errorf("git %s: %s", args[0], msg)
	}
	return out.String(), nil
}
