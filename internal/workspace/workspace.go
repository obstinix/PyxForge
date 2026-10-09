// Package workspace finds the project a folder belongs to: the nearest pyxforge.toml above it
// and the Git checkout it is in. It reads files only; it never runs Git.
package workspace

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// ProjectFile is the name of a PyxForge project's configuration file.
const ProjectFile = "pyxforge.toml"

// Workspace is what Discover found for a folder.
type Workspace struct {
	Dir         string // the folder asked about, absolute
	Root        string // the project root: the folder holding pyxforge.toml, else Dir
	ProjectFile string // absolute path of pyxforge.toml, or "" when there is none
	GitRoot     string // the checkout's top folder, or "" outside Git
	Branch      string // checked-out branch, or a short commit for a detached HEAD
}

// Discover resolves dir to an absolute path and searches it and its parents for a project
// file and a Git checkout.
func Discover(dir string) (Workspace, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return Workspace{}, err
	}
	fi, err := os.Stat(abs)
	if errors.Is(err, fs.ErrNotExist) {
		return Workspace{}, fmt.Errorf("%s: no such folder", abs)
	}
	if err != nil {
		return Workspace{}, err
	}
	if !fi.IsDir() {
		return Workspace{}, fmt.Errorf("%s: not a folder", abs)
	}
	w := Workspace{Dir: abs, Root: abs}
	for d := abs; ; d = filepath.Dir(d) {
		if w.ProjectFile == "" {
			if p := filepath.Join(d, ProjectFile); isFile(p) {
				w.ProjectFile, w.Root = p, d
			}
		}
		if w.GitRoot == "" {
			if _, err := os.Stat(filepath.Join(d, ".git")); err == nil {
				w.GitRoot = d
				w.Branch, _ = GitBranch(d)
			}
		}
		if (w.ProjectFile != "" && w.GitRoot != "") || filepath.Dir(d) == d {
			return w, nil
		}
	}
}

func isFile(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.Mode().IsRegular()
}

// GitBranch reads the checked-out branch from root's .git without running git. ok is false
// when root is not a repository. A detached HEAD returns its short hash.
func GitBranch(root string) (string, bool) {
	gitPath := filepath.Join(root, ".git")
	if fi, err := os.Stat(gitPath); err == nil && !fi.IsDir() {
		// A worktree or submodule: .git is a file pointing at the real git directory.
		b, err := os.ReadFile(gitPath)
		if err != nil {
			return "", false
		}
		dir := strings.TrimSpace(strings.TrimPrefix(string(b), "gitdir:"))
		if !filepath.IsAbs(dir) {
			dir = filepath.Join(root, dir)
		}
		gitPath = dir
	}
	head, err := os.ReadFile(filepath.Join(gitPath, "HEAD"))
	if err != nil {
		return "", false
	}
	h := strings.TrimSpace(string(head))
	if ref, ok := strings.CutPrefix(h, "ref: refs/heads/"); ok {
		return ref, true
	}
	if len(h) >= 7 {
		return h[:7], true
	}
	return "", false
}
