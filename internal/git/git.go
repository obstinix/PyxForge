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
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/obstinix/PyxForge/internal/proc"
	"github.com/obstinix/PyxForge/internal/toolchain"
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

// Status reads `git status --porcelain=v2`. A submodule is listed when it is at another
// commit than the one recorded; Git is not sent into its working tree, where it would run that
// repository's own configuration.
func (r *Repo) Status(ctx context.Context) (Status, error) {
	out, err := r.run(ctx, nil, "status", "--porcelain=v2", "--branch", "-z", "--untracked-files=all", "--ignore-submodules=dirty")
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
	_, err := r.write(ctx, nil, append([]string{"add", "--all", "--"}, paths...)...)
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
	if _, err := r.write(ctx, strings.NewReader(message), "commit", "--file=-", "--cleanup=strip"); err != nil {
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

// projectProgramKeys are the settings naming a program Git would run during PyxForge's
// operations: content filters (status and add run clean, checkout runs smudge), merge drivers
// (stash apply) and signing programs (commit). core.fsmonitor is always off. Git prints keys
// with the section and the name in lower case.
const projectProgramKeys = `^(filter\..+\.(clean|smudge|process)|merge\..+\.driver|gpg\.(.+\.)?program|gpg\.ssh\.defaultkeycommand)$`

// ProjectProgramError means the repository's own configuration names a program that the
// operation would make Git run. PyxForge runs nothing a project names (decision D14), so the
// operation is left to a terminal, where the user runs Git knowingly.
type ProjectProgramError struct {
	Key, Value string
	User       string // the value the user's global or system configuration gives Key, if any
}

func (e *ProjectProgramError) Error() string {
	user := ""
	if e.User != "" {
		user = fmt.Sprintf(" (your own Git configuration sets %s)", e.User)
	}
	return fmt.Sprintf("this repository's own Git configuration sets %s = %s%s, a program Git would run here. "+
		"PyxForge does not run programs a project names: do this from a terminal, or move the setting to your global Git configuration",
		e.Key, e.Value, user)
}

// projectPrograms lists the projectProgramKeys settings that come from the repository itself
// (.git/config, the worktree's config and the files they include), not from the user's
// global or the system configuration, where Git LFS, for one, sets its filter. A repository
// setting that repeats one of those, key and value, names a program the user already runs
// everywhere and is not listed: `git lfs install --local` writes the same filter.lfs.*
// settings into .git/config that the system or global configuration has.
func (r *Repo) projectPrograms(ctx context.Context) ([]*ProjectProgramError, error) {
	out, err := r.exec(ctx, nil, nil, "config", "--show-scope", "--null", "--get-regexp", projectProgramKeys)
	if err != nil {
		if ee := (*exec.ExitError)(nil); errors.As(err, &ee) && ee.ExitCode() == 1 && out == "" {
			return nil, nil // no such settings
		}
		return nil, err
	}
	var repo [][2]string
	user := map[[2]string]bool{}
	userValue := map[string]string{} // each key's last, effective, value from the user's files
	f := strings.Split(out, "\x00")
	for i := 0; i+1 < len(f); i += 2 {
		key, value, _ := strings.Cut(f[i+1], "\n")
		if scope := f[i]; scope == "local" || scope == "worktree" {
			repo = append(repo, [2]string{key, value})
		} else {
			user[[2]string{key, value}] = true
			userValue[key] = value
		}
	}
	var list []*ProjectProgramError
	for _, p := range repo {
		if !user[p] {
			list = append(list, &ProjectProgramError{Key: p[0], Value: p[1], User: userValue[p[0]]})
		}
	}
	return list, nil
}

// safety returns the -c options that keep Git from running the project's own filters while it
// reads, or an error when it cannot, or, for an operation that changes files (write), when
// the project names any program the operation could run: running it would execute project
// code, and skipping a filter would store or check out the wrong content.
func (r *Repo) safety(ctx context.Context, write bool) ([]string, error) {
	progs, err := r.projectPrograms(ctx)
	if err != nil {
		return nil, err
	}
	var opts []string
	for _, p := range progs {
		if write {
			return nil, p
		}
		name, ok := strings.CutPrefix(p.Key, "filter.")
		if !ok {
			continue // merge drivers and signing programs do not run while reading
		}
		name = name[:strings.LastIndexByte(name, '.')]
		if strings.ContainsAny(name, "=\n") { // cannot be written as -c key=value
			return nil, p
		}
		opts = append(opts, "-c", "filter."+name+".clean=", "-c", "filter."+name+".smudge=",
			"-c", "filter."+name+".process=", "-c", "filter."+name+".required=false")
	}
	return opts, nil
}

// run runs a git command that reads, or changes only refs and the index from what is
// committed; write runs one that converts files between the working tree and the repository.
func (r *Repo) run(ctx context.Context, stdin reader, args ...string) (string, error) {
	return r.guarded(ctx, false, stdin, args...)
}

func (r *Repo) write(ctx context.Context, stdin reader, args ...string) (string, error) {
	return r.guarded(ctx, true, stdin, args...)
}

func (r *Repo) guarded(ctx context.Context, write bool, stdin reader, args ...string) (string, error) {
	opts, err := r.safety(ctx, write)
	if err != nil {
		return "", r.explain(args, err)
	}
	out, err := r.exec(ctx, stdin, opts, args...)
	return out, r.explain(args, err)
}

// explain turns a failure into git's own message.
func (r *Repo) explain(args []string, err error) error {
	var ee *gitError
	switch {
	case err == nil:
		return nil
	case errors.Is(err, exec.ErrNotFound):
		return errors.New("git is not installed or not on PATH")
	case errors.As(err, &ee):
		return fmt.Errorf("git %s: %s", args[0], ee.msg)
	}
	return err
}

// gitError is a git command that failed, with what it printed.
type gitError struct {
	msg string
	err error
}

func (e *gitError) Error() string { return e.msg }
func (e *gitError) Unwrap() error { return e.err }

// exec runs git in Root with prompts and pagers off, and returns its standard output.
func (r *Repo) exec(ctx context.Context, stdin reader, opts []string, args ...string) (string, error) {
	exe := r.Git
	if exe == "" {
		exe = "git"
		if p, err := toolchain.LookPath("git"); err == nil {
			exe = p // a path set in Doctor, or Git's install folder
		}
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	// core.fsmonitor names a program Git runs during status: a repository's own config could
	// make merely opening it run code, so PyxForge never lets Git start one. Nor does Git go
	// into submodules, whose own configuration safety does not read.
	base := []string{"-C", r.Root, "-c", "core.quotepath=off", "-c", "color.ui=false", "-c", "core.fsmonitor=false",
		"-c", "submodule.recurse=false"}
	cmd := exec.CommandContext(ctx, exe, slices.Concat(base, opts, args)...)
	proc.NoWindow(cmd)
	cmd.Env = append(cmd.Environ(), "GIT_TERMINAL_PROMPT=0", "GIT_PAGER=cat", "LC_ALL=C", "GIT_OPTIONAL_LOCKS=0")
	if stdin != nil {
		cmd.Stdin = stdin
	}
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return "", err
		}
		msg := strings.TrimSpace(errb.String())
		if msg == "" {
			msg = err.Error()
		}
		return out.String(), &gitError{msg: msg, err: err}
	}
	return out.String(), nil
}
