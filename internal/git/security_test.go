package git

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
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

func gitIn(t *testing.T, r *Repo, args ...string) {
	t.Helper()
	if out, err := exec.Command("git", append([]string{"-C", r.Root}, args...)...).CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// TestRepositoryFiltersDoNotRun: a repository's own config can name a clean filter, which
// plain `git status` runs whenever a file's timestamps differ from the index, as they do in a
// project unpacked from an archive with its .git folder. Opening and reading must not start
// it; staging, committing and switching, which would need it, are refused with the reason. A
// filter from the user's own configuration (Git LFS sets one) still applies.
func TestRepositoryFiltersDoNotRun(t *testing.T) {
	r := newRepo(t)
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	write(t, r, "boot.asm", "org 0x7c00\n")
	gitIn(t, r, "add", ".")
	gitIn(t, r, "commit", "--quiet", "-m", "first")
	write(t, r, ".gitattributes", "*.asm filter=evil\n")
	gitIn(t, r, "config", "filter.evil.clean", "'"+filepath.ToSlash(self)+"' clean")
	mark := filepath.Join(t.TempDir(), "filter-ran")
	t.Setenv("PYX_FSMONITOR_MARK", mark)
	touch := func() {
		t.Helper()
		when := time.Now().Add(-time.Hour * time.Duration(1+len(t.Name())))
		if err := os.Chtimes(filepath.Join(r.Root, "boot.asm"), when, when); err != nil {
			t.Fatal(err)
		}
	}

	// Plain git does run it: the checks below are meaningful.
	touch()
	_ = exec.Command("git", "-C", r.Root, "status").Run()
	if _, err := os.Stat(mark); err != nil {
		t.Skipf("this git did not run the clean filter (%v); nothing to guard against", err)
	}
	if err := os.Remove(mark); err != nil {
		t.Fatal(err)
	}

	touch()
	if st, err := r.Status(ctx); err != nil || len(st.Files) != 1 || st.Files[0].Path != ".gitattributes" {
		t.Errorf("Status = %+v, %v; want only .gitattributes untracked", st, err)
	}
	if _, err := r.Log(ctx, 5); err != nil {
		t.Error(err)
	}
	if _, err := r.Show(ctx, "HEAD", "boot.asm"); err != nil {
		t.Error(err)
	}
	if _, err := r.Branches(ctx); err != nil {
		t.Error(err)
	}
	var refused *ProjectProgramError
	if err := r.Stage(ctx, "boot.asm"); !errors.As(err, &refused) || refused.Key != "filter.evil.clean" {
		t.Errorf("Stage: %v, want a ProjectProgramError for filter.evil.clean", err)
	}
	if _, err := r.Commit(ctx, "x"); !errors.As(err, &refused) {
		t.Errorf("Commit: %v", err)
	}
	if err := r.CreateBranch(ctx, "side", true); !errors.As(err, &refused) {
		t.Errorf("CreateBranch and switch: %v", err)
	}
	if _, err := os.Stat(mark); err == nil {
		t.Error("PyxForge's Git calls ran the repository's clean filter")
	}

	// The same filter in the user's global configuration is the user's choice and runs.
	gitIn(t, r, "config", "--unset", "filter.evil.clean")
	global := filepath.Join(t.TempDir(), "gitconfig")
	if err := os.WriteFile(global, []byte("[filter \"evil\"]\n\tclean = '"+filepath.ToSlash(self)+"' clean\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", global)
	touch()
	if _, err := r.Status(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(mark); err != nil {
		t.Error("a filter from the global configuration was not run")
	}
}

// TestSubmoduleFiltersDoNotRun: plain `git status` also checks each submodule's working tree,
// running that repository's own filters; PyxForge's Status does not go there.
func TestSubmoduleFiltersDoNotRun(t *testing.T) {
	sub, top := newRepo(t), newRepo(t)
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	write(t, sub, "s.asm", "hlt\n")
	write(t, sub, ".gitattributes", "*.asm filter=evil\n")
	gitIn(t, sub, "add", ".")
	gitIn(t, sub, "commit", "--quiet", "-m", "sub")
	gitIn(t, top, "-c", "protocol.file.allow=always", "submodule", "add", "--quiet", sub.Root, "sub")
	gitIn(t, top, "commit", "--quiet", "-m", "top")
	inner := &Repo{Root: filepath.Join(top.Root, "sub")}
	gitIn(t, inner, "config", "filter.evil.clean", "'"+filepath.ToSlash(self)+"' clean")
	mark := filepath.Join(t.TempDir(), "filter-ran")
	t.Setenv("PYX_FSMONITOR_MARK", mark)
	touch := func(hours int) {
		when := time.Now().Add(-time.Duration(hours) * time.Hour)
		if err := os.Chtimes(filepath.Join(inner.Root, "s.asm"), when, when); err != nil {
			t.Fatal(err)
		}
	}
	touch(30)
	_ = exec.Command("git", "-C", top.Root, "status").Run()
	if _, err := os.Stat(mark); err != nil {
		t.Skipf("this git did not run the submodule's filter (%v); nothing to guard against", err)
	}
	if err := os.Remove(mark); err != nil {
		t.Fatal(err)
	}
	touch(40)
	if _, err := top.Status(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(mark); err == nil {
		t.Error("PyxForge's Status ran the submodule's clean filter")
	}
}
