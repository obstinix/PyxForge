package git

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// isolateConfig points Git's global and system configuration at empty files of the test's
// own, so the machine's real configuration plays no part, and returns their paths.
func isolateConfig(t *testing.T) (global, system string) {
	t.Helper()
	dir := t.TempDir()
	global, system = filepath.Join(dir, "global.gitconfig"), filepath.Join(dir, "system.gitconfig")
	for _, p := range []string{global, system} {
		if err := os.WriteFile(p, nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("GIT_CONFIG_GLOBAL", global)
	t.Setenv("GIT_CONFIG_SYSTEM", system)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "")
	return global, system
}

// gitOut runs plain git in r and returns its output, failing the test on an error.
func gitOut(t *testing.T, r *Repo, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", r.Root}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}

// setConfig writes key = value into a configuration file, or into the repository's own
// configuration when file is "".
func setConfig(t *testing.T, r *Repo, file, key, value string) {
	t.Helper()
	if file == "" {
		gitIn(t, r, "config", "--local", key, value)
		return
	}
	gitIn(t, r, "config", "--file", file, key, value)
}

// TestRepeatedUserFiltersAreNotRefused: decision D14 refuses staging and committing when the
// repository's own configuration names a program Git would run. A repository setting that
// repeats, key and value, one from the user's global or system configuration names a program
// the user already runs everywhere (`git lfs install --local` writes exactly that), so it must
// not be refused; a repository setting that differs from the user's, or that the user does not
// have, still is.
func TestRepeatedUserFiltersAreNotRefused(t *testing.T) {
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	filter := "'" + filepath.ToSlash(self) + "' pass-filter"
	type where struct{ global, system, local string } // the clean filter each one sets
	for _, tc := range []struct {
		name    string
		set     where
		refused bool
	}{
		{name: "ordinary repository"},
		{name: "filter in the global configuration", set: where{global: filter}},
		{name: "repository repeats the global filter", set: where{global: filter, local: filter}},
		{name: "repository repeats the system filter", set: where{system: filter, local: filter}},
		{name: "repository changes the user's filter", set: where{global: filter, local: filter + " changed"}, refused: true},
		{name: "repository names its own filter", set: where{local: filter}, refused: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			global, system := isolateConfig(t)
			mark := filepath.Join(t.TempDir(), "filter-ran")
			t.Setenv("PYX_FILTER_MARK", mark)
			r := newRepo(t)
			ctx := context.Background()
			uses := tc.set != where{}
			for file, value := range map[string]string{global: tc.set.global, system: tc.set.system, "": tc.set.local} {
				if value != "" {
					setConfig(t, r, file, "filter.pass.clean", value)
					setConfig(t, r, file, "filter.pass.smudge", value)
				}
			}
			if uses {
				write(t, r, ".gitattributes", "*.txt filter=pass\n")
			}
			write(t, r, "notes.txt", "kept byte for byte\r\n")

			err := r.Stage(ctx, ".")
			if tc.refused {
				var refused *ProjectProgramError
				if !errors.As(err, &refused) || refused.Key != "filter.pass.clean" && refused.Key != "filter.pass.smudge" {
					t.Fatalf("Stage: %v, want a ProjectProgramError for filter.pass", err)
				}
				if !strings.Contains(err.Error(), refused.Key) || !strings.Contains(err.Error(), refused.Value) {
					t.Errorf("the refusal does not name the setting: %v", err)
				}
				if tc.set.global != "" && (refused.User != tc.set.global || !strings.Contains(err.Error(), "your own Git configuration sets "+tc.set.global)) {
					t.Errorf("the refusal does not say what the user's own configuration sets: %v", err)
				}
				if _, err := os.Stat(mark); err == nil {
					t.Error("the refused filter ran")
				}
				if staged := gitOut(t, r, "diff", "--cached", "--name-only"); staged != "" {
					t.Errorf("a refused Stage staged %q", staged)
				}
				return
			}
			if err != nil {
				t.Fatalf("Stage: %v", err)
			}
			hash, err := r.Commit(ctx, "first")
			if err != nil {
				t.Fatalf("Commit: %v", err)
			}
			// Check the result with plain git, not through the code under test.
			if head := strings.TrimSpace(gitOut(t, r, "rev-parse", "HEAD")); hash == "" || !strings.HasPrefix(head, hash) {
				t.Errorf("Commit returned %q; HEAD is %s", hash, head)
			}
			if got := gitOut(t, r, "cat-file", "blob", "HEAD:notes.txt"); got != "kept byte for byte\r\n" {
				t.Errorf("committed content %q", got)
			}
			if st := gitOut(t, r, "status", "--porcelain"); st != "" {
				t.Errorf("working tree not clean after the commit:\n%s", st)
			}
			if _, err := os.Stat(mark); uses && err != nil {
				t.Error("the user's filter did not run")
			}
		})
	}
}

// TestGitLFSRepositoryStagesAndCommits: a Git LFS repository, with LFS installed for the user
// only and then also in the repository itself (`git lfs install --local`, which writes the same
// filter.lfs.* settings into .git/config), stages and commits through PyxForge, and the commit
// holds LFS pointers.
func TestGitLFSRepositoryStagesAndCommits(t *testing.T) {
	if err := exec.Command("git", "lfs", "version").Run(); err != nil {
		t.Skip("git-lfs is not installed")
	}
	isolateConfig(t)
	r := newRepo(t)
	ctx := context.Background()
	gitIn(t, r, "lfs", "install", "--skip-repo") // the user's global configuration
	gitIn(t, r, "lfs", "track", "*.bin")

	commit := func(name, body string) {
		t.Helper()
		write(t, r, name, body)
		if err := r.Stage(ctx, ".gitattributes", name); err != nil {
			t.Fatalf("Stage %s: %v", name, err)
		}
		if _, err := r.Commit(ctx, "add "+name); err != nil {
			t.Fatalf("Commit %s: %v", name, err)
		}
		if blob := gitOut(t, r, "cat-file", "blob", "HEAD:"+name); !strings.HasPrefix(blob, "version https://git-lfs.github.com/spec/v1") {
			t.Errorf("%s was committed as %q, not as an LFS pointer", name, blob)
		}
		if st := gitOut(t, r, "status", "--porcelain"); st != "" {
			t.Errorf("working tree not clean after committing %s:\n%s", name, st)
		}
	}
	commit("global.bin", "\x00\x01 stored through the user's LFS filter\n")

	gitIn(t, r, "lfs", "install", "--local")
	scopes := gitOut(t, r, "config", "--show-scope", "--get-regexp", `^filter\.lfs\.`)
	if !strings.Contains(scopes, "local\tfilter.lfs.clean") || !strings.Contains(scopes, "global\tfilter.lfs.clean") {
		t.Fatalf("git lfs install --local did not repeat the global settings in .git/config:\n%s", scopes)
	}
	commit("local.bin", "\x00\x02 stored with LFS also set in .git/config\n")
}
