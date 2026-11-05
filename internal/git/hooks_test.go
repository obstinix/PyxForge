package git

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestRepositoryHooksPathNeedsTrust: a repository whose own configuration points
// core.hooksPath at a folder of its files runs those hooks from PyxForge only once allowed,
// which the shell does for a trusted workspace (#30). The setting repeated in the user's own
// configuration is the user's, and .git/hooks keep running as with git (D14).
func TestRepositoryHooksPathNeedsTrust(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(t.TempDir(), "gitconfig")) // never the user's own
	marker := filepath.Join(r.Root, "hook-ran")
	hook := "#!/bin/sh\necho ran > hook-ran\n"
	write := func(rel, body string) {
		t.Helper()
		p := filepath.Join(r.Root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	gitDo := func(args ...string) {
		t.Helper()
		if out, err := exec.Command("git", append([]string{"-C", r.Root}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	commit := func(file string) error {
		t.Helper()
		write(file, file+"\n")
		gitDo("add", file)
		_, err := r.Commit(ctx, "add "+file)
		return err
	}
	write("hooks/pre-commit", hook)
	gitDo("config", "core.hooksPath", "hooks")

	allowed := false
	r.AllowHooksPath = func() bool { return allowed }
	err := commit("a.txt")
	var hp *HooksPathError
	if !errors.As(err, &hp) || hp.Path != "hooks" {
		t.Fatalf("commit with the project's hooks not allowed: %v", err)
	}
	if _, err := os.Stat(marker); err == nil {
		t.Fatal("the project's hook ran although it wasn't allowed")
	}
	if log, _ := r.Log(ctx, 5); len(log) != 0 {
		t.Fatalf("a commit was made: %v", log)
	}

	allowed = true
	if err := commit("a.txt"); err != nil {
		t.Fatalf("commit once allowed: %v", err)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatal("the allowed hook did not run")
	}

	// The same setting in the user's own configuration is theirs: not refused.
	allowed = false
	if err := os.WriteFile(os.Getenv("GIT_CONFIG_GLOBAL"), []byte("[core]\n\thooksPath = hooks\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := commit("b.txt"); err != nil {
		t.Fatalf("commit with core.hooksPath repeated in the user's configuration: %v", err)
	}

	// No core.hooksPath of the project's: .git/hooks runs, allowed or not.
	if err := os.WriteFile(os.Getenv("GIT_CONFIG_GLOBAL"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	gitDo("config", "--unset", "core.hooksPath")
	os.Remove(marker)
	write(".git/hooks/pre-commit", hook)
	if err := commit("c.txt"); err != nil {
		t.Fatalf("commit with only .git/hooks: %v", err)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatal(".git/hooks' pre-commit did not run")
	}
}
