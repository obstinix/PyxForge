package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGitBranch(t *testing.T) {
	write := func(p, s string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	repo := t.TempDir()
	write(filepath.Join(repo, ".git", "HEAD"), "ref: refs/heads/v3\n")
	if b, ok := GitBranch(repo); !ok || b != "v3" {
		t.Errorf("branch = %q, %v", b, ok)
	}

	detached := t.TempDir()
	write(filepath.Join(detached, ".git", "HEAD"), "9d273f5aa0bb1c2d3e4f5a6b7c8d9e0f1a2b3c4d\n")
	if b, ok := GitBranch(detached); !ok || b != "9d273f5" {
		t.Errorf("detached = %q, %v", b, ok)
	}

	// A linked worktree: .git is a file naming the real git directory.
	worktree := t.TempDir()
	gitdir := filepath.Join(t.TempDir(), "worktrees", "ref")
	write(filepath.Join(gitdir, "HEAD"), "ref: refs/heads/main\n")
	write(filepath.Join(worktree, ".git"), "gitdir: "+gitdir+"\n")
	if b, ok := GitBranch(worktree); !ok || b != "main" {
		t.Errorf("worktree = %q, %v", b, ok)
	}

	if _, ok := GitBranch(t.TempDir()); ok {
		t.Error("plain folder reported a branch")
	}
}

func TestDiscover(t *testing.T) {
	write := func(p, s string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	repo := t.TempDir()
	write(filepath.Join(repo, ".git", "HEAD"), "ref: refs/heads/main\n")
	write(filepath.Join(repo, "os", ProjectFile), "[project]\nname = \"pyxos\"\n")
	deep := filepath.Join(repo, "os", "boot", "stage1")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}

	w, err := Discover(deep)
	if err != nil {
		t.Fatal(err)
	}
	if w.Dir != deep || w.Root != filepath.Join(repo, "os") || w.ProjectFile != filepath.Join(repo, "os", ProjectFile) {
		t.Errorf("project not found from a subfolder: %+v", w)
	}
	if w.GitRoot != repo || w.Branch != "main" {
		t.Errorf("git checkout not found: %+v", w)
	}

	plain := t.TempDir()
	w, err = Discover(plain)
	if err != nil || w.Root != plain || w.ProjectFile != "" {
		t.Errorf("a folder without a project file: %+v, %v", w, err)
	}

	if _, err := Discover(filepath.Join(plain, "missing")); err == nil {
		t.Error("a missing folder was accepted")
	}
	file := filepath.Join(plain, "boot.asm")
	write(file, "")
	if _, err := Discover(file); err == nil {
		t.Error("a file was accepted as a workspace")
	}
}
