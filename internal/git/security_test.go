package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// The test binary doubles as a filesystem monitor hook: with PYX_FSMONITOR_MARK set it writes
// that file and exits, as a malicious core.fsmonitor program in a cloned repository could.
func TestMain(m *testing.M) {
	if mark := os.Getenv("PYX_FSMONITOR_MARK"); mark != "" && len(os.Args) > 1 && os.Args[1] != "-test.paniconexit0" {
		_ = os.WriteFile(mark, []byte("ran"), 0o644)
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// TestRepositoryConfigCannotRunCode: a repository's own core.fsmonitor setting would make
// `git status` run a program; PyxForge's Git calls must not start it.
func TestRepositoryConfigCannotRunCode(t *testing.T) {
	r := newRepo(t)
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	mark := filepath.Join(t.TempDir(), "hook-ran")
	t.Setenv("PYX_FSMONITOR_MARK", mark)
	if out, err := exec.Command("git", "-C", r.Root, "config", "core.fsmonitor", filepath.ToSlash(self)).CombinedOutput(); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	write(t, r, "boot.asm", "org 0x7c00\n")

	// Plain git does run it: the check below is meaningful.
	_ = exec.Command("git", "-C", r.Root, "status").Run()
	if _, err := os.Stat(mark); err != nil {
		t.Skipf("this git does not run core.fsmonitor programs (%v); nothing to guard against", err)
	}
	if err := os.Remove(mark); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	if _, err := r.Status(ctx); err != nil {
		t.Fatal(err)
	}
	if err := r.Stage(ctx, "."); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Branches(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(mark); err == nil {
		t.Error("PyxForge's Git calls ran the repository's fsmonitor program")
	}
}
