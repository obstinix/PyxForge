package git

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/obstinix/PyxForge/internal/proc"
)

// newRemote makes an empty bare repository, with HEAD naming main, for tests to push to.
func newRemote(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "remote.git")
	for _, args := range [][]string{{"init", "--quiet", "--bare", dir}, {"-C", dir, "symbolic-ref", "HEAD", "refs/heads/main"}} {
		if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	return dir
}

// clone clones url into a new directory with a local identity, as newRepo has.
func clone(t *testing.T, url string) *Repo {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "clone")
	if out, err := exec.Command("git", "clone", "--quiet", url, dir).CombinedOutput(); err != nil {
		t.Fatalf("git clone: %v\n%s", err, out)
	}
	r, err := Open(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, kv := range [][2]string{{"user.name", "PyxForge Test"}, {"user.email", "test@example.invalid"},
		{"commit.gpgsign", "false"}, {"core.autocrlf", "false"}} {
		gitIn(t, r, "config", kv[0], kv[1])
	}
	return r
}

// problem is the kind of a RemoteError, or -1 for any other error, 0 for none.
func problem(err error) Problem {
	if err == nil {
		return ProblemUnknown
	}
	var re *RemoteError
	if !errors.As(err, &re) {
		return -1
	}
	return re.Problem
}

func status(t *testing.T, r *Repo) Status {
	t.Helper()
	st, err := r.Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return st
}

// TestFetchPullPush: a branch is published with its upstream set, a clone fetches and pulls
// what is pushed later, and when both sides have new commits the push is rejected and the
// pull refuses to merge or rebase (even with pull.rebase set), changing nothing. A pull that
// would overwrite uncommitted changes is refused too.
func TestFetchPullPush(t *testing.T) {
	isolateConfig(t)
	ctx := context.Background()
	bare := newRemote(t)
	a := newRepo(t)
	write(t, a, "boot.asm", "org 0x7c00\n")
	commitAll(t, a, "one")
	if err := a.AddRemote(ctx, "origin", bare); err != nil {
		t.Fatal(err)
	}
	if rs, err := a.Remotes(ctx); err != nil || len(rs) != 1 || rs[0].Name != "origin" || rs[0].URL != bare {
		t.Fatalf("Remotes: %+v, %v", rs, err)
	}

	if err := a.Push(ctx, ""); problem(err) != ProblemNoUpstream {
		t.Fatalf("Push without an upstream: %v (problem %d)", err, problem(err))
	}
	if err := a.Push(ctx, "origin"); err != nil {
		t.Fatalf("publish: %v", err)
	}
	if st := status(t, a); st.Upstream != "origin/main" || st.Ahead != 0 {
		t.Fatalf("after publishing: upstream %q, ahead %d", st.Upstream, st.Ahead)
	}

	b := clone(t, bare)
	write(t, a, "boot.asm", "org 0x7c00\nbits 16\n")
	commitAll(t, a, "two")
	if err := a.Push(ctx, ""); err != nil {
		t.Fatalf("Push: %v", err)
	}
	if err := b.Fetch(ctx); err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if st := status(t, b); st.Behind != 1 || read(t, b, "boot.asm") != "org 0x7c00\n" {
		t.Fatalf("after Fetch: behind %d, file %q", st.Behind, read(t, b, "boot.asm"))
	}
	if err := b.Pull(ctx); err != nil {
		t.Fatalf("Pull: %v", err)
	}
	if st := status(t, b); st.Behind != 0 || read(t, b, "boot.asm") != "org 0x7c00\nbits 16\n" {
		t.Fatalf("after Pull: behind %d, file %q", st.Behind, read(t, b, "boot.asm"))
	}

	// Both sides commit.
	write(t, a, "a.txt", "from a\n")
	commitAll(t, a, "three")
	if err := a.Push(ctx, ""); err != nil {
		t.Fatal(err)
	}
	write(t, b, "b.txt", "from b\n")
	commitAll(t, b, "local")
	if err := b.Push(ctx, ""); problem(err) != ProblemRejected {
		t.Errorf("Push behind the remote: %v (problem %d)", err, problem(err))
	}
	if err := b.Fetch(ctx); err != nil {
		t.Fatal(err)
	}
	head := gitOut(t, b, "rev-parse", "HEAD")
	gitIn(t, b, "config", "pull.rebase", "true")
	if err := b.Pull(ctx); problem(err) != ProblemDiverged {
		t.Errorf("Pull with both sides changed: %v (problem %d)", err, problem(err))
	}
	if st := status(t, b); st.Ahead != 1 || st.Behind != 1 || gitOut(t, b, "rev-parse", "HEAD") != head {
		t.Errorf("a refused pull changed the branch: ahead %d, behind %d", st.Ahead, st.Behind)
	}

	// A pull that would overwrite an uncommitted change.
	c := clone(t, bare)
	write(t, a, "boot.asm", "org 0x7c00\nbits 32\n")
	commitAll(t, a, "four")
	if err := a.Push(ctx, ""); err != nil {
		t.Fatal(err)
	}
	write(t, c, "boot.asm", "my own edit\n")
	if err := c.Pull(ctx); problem(err) != ProblemLocalChanges {
		t.Errorf("Pull over an uncommitted change: %v (problem %d)", err, problem(err))
	}
	if read(t, c, "boot.asm") != "my own edit\n" {
		t.Error("the refused pull lost the uncommitted change")
	}
}

// TestRemoteProblems: what a failure is, as the Git tab explains it.
func TestRemoteProblems(t *testing.T) {
	isolateConfig(t)
	ctx := context.Background()
	r := newRepo(t)
	write(t, r, "a.txt", "a\n")
	commitAll(t, r, "one")

	if err := r.Fetch(ctx); problem(err) != ProblemNoRemote {
		t.Errorf("Fetch without a remote: %v (problem %d)", err, problem(err))
	}
	if err := r.Push(ctx, ""); problem(err) != ProblemNoRemote {
		t.Errorf("Push without a remote: %v (problem %d)", err, problem(err))
	}
	if err := r.AddRemote(ctx, "origin", filepath.Join(t.TempDir(), "gone.git")); err != nil {
		t.Fatal(err)
	}
	if err := r.Fetch(ctx); problem(err) != ProblemNotFound {
		t.Errorf("Fetch from a missing repository: %v (problem %d)", err, problem(err))
	}
	gitIn(t, r, "remote", "set-url", "origin", "https://127.0.0.1:1/nothing.git") // nothing listens on port 1
	if err := r.Fetch(ctx); problem(err) != ProblemNetwork {
		t.Errorf("Fetch from an unreachable server: %v (problem %d)", err, problem(err))
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if err := r.Fetch(cancelled); problem(err) != ProblemCancelled {
		t.Errorf("Fetch after cancelling: %v (problem %d)", err, problem(err))
	}

	for _, bad := range [][2]string{{"", "x"}, {"origin2", " "}, {"-x", "url"}} {
		if err := r.AddRemote(ctx, bad[0], bad[1]); err == nil {
			t.Errorf("AddRemote(%q, %q) succeeded", bad[0], bad[1])
		}
	}
	if err := r.Push(ctx, "--force"); err == nil || problem(err) != -1 {
		t.Errorf("Push to a remote named like an option: %v", err)
	}
}

// TestProblemOf: the kinds read from git's messages, as Git 2.4x prints them.
func TestProblemOf(t *testing.T) {
	for msg, want := range map[string]Problem{
		"fatal: Authentication failed for 'https://github.com/o/r.git/'":                                                                                                                              ProblemAuth,
		"fatal: could not read Username for 'https://github.com': terminal prompts disabled":                                                                                                          ProblemAuth,
		"git@github.com: Permission denied (publickey).\r\nfatal: Could not read from remote repository.":                                                                                             ProblemAuth,
		"remote: Permission to o/r.git denied to someone.\nfatal: unable to access '...': The requested URL returned error: 403":                                                                      ProblemAuth,
		"Host key verification failed.\nfatal: Could not read from remote repository.":                                                                                                                ProblemHostKey,
		"remote: Repository not found.\nfatal: repository 'https://github.com/o/nothing.git/' not found":                                                                                              ProblemNotFound,
		"fatal: '../nothere.git' does not appear to be a git repository":                                                                                                                              ProblemNotFound,
		"fatal: unable to access 'https://nonexistent.invalid/x.git/': Could not resolve host: nonexistent.invalid":                                                                                   ProblemNetwork,
		"ssh: connect to host 127.0.0.1 port 1: Connection refused\nfatal: Could not read from remote repository.\n\nPlease make sure you have the correct access rights\nand the repository exists.": ProblemNetwork,
		"ssh: Could not resolve hostname example.invalid: Name or service not known":                                                                                                                  ProblemNetwork,
		"fatal: No configured push destination.":                                                                                                                                                      ProblemNoRemote,
		"fatal: The current branch topic has no upstream branch.":                                                                                                                                     ProblemNoUpstream,
		"There is no tracking information for the current branch.":                                                                                                                                    ProblemNoUpstream,
		" ! [rejected]        main -> main (non-fast-forward)\nerror: failed to push some refs":                                                                                                       ProblemRejected,
		" ! [rejected]        main -> main (fetch first)":                                                                                                                                             ProblemRejected,
		"hint: Diverging branches can't be fast-forwarded\nfatal: Not possible to fast-forward, aborting.":                                                                                            ProblemDiverged,
		"error: Your local changes to the following files would be overwritten by merge:\n\tboot.asm":                                                                                                 ProblemLocalChanges,
		"fatal: something else went wrong":                                                                                                                                                            ProblemUnknown,
	} {
		if got := problemOf(msg); got != want {
			t.Errorf("problemOf(%q) = %d, want %d", msg, got, want)
		}
	}
}

// TestRepositoryNetworkProgramsAreRefused: a repository's own configuration can name programs
// Git runs to reach a remote; for a remote on this machine, remote.<name>.uploadpack runs at
// once. Fetch, pull and push refuse them, as staging refuses the repository's filters
// (decision D14); the same setting from the user's own configuration, or an empty value, is
// not refused.
func TestRepositoryNetworkProgramsAreRefused(t *testing.T) {
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	program := "'" + filepath.ToSlash(self) + "' pass-filter" // leaves PYX_FILTER_MARK when run
	global, _ := isolateConfig(t)
	ctx := context.Background()
	bare := newRemote(t)
	r := newRepo(t)
	write(t, r, "a.txt", "a\n")
	commitAll(t, r, "one")
	if err := r.AddRemote(ctx, "origin", bare); err != nil {
		t.Fatal(err)
	}
	if err := r.Push(ctx, "origin"); err != nil {
		t.Fatal(err)
	}

	// Plain git runs the repository's upload-pack program.
	mark := filepath.Join(t.TempDir(), "ran")
	t.Setenv("PYX_FILTER_MARK", mark)
	setConfig(t, r, "", "remote.origin.uploadpack", program)
	// The stand-in leaves its mark at once, then waits on a request that never comes.
	plain, stop := context.WithTimeout(ctx, 5*time.Second)
	fetch := exec.CommandContext(plain, "git", "-C", r.Root, "fetch", "--quiet")
	fetch.WaitDelay = time.Second
	fetch.Cancel = func() error { // the whole tree: Git for Windows' cmd\git.exe starts the real git
		tree := proc.Descendants(fetch.Process.Pid)
		err := fetch.Process.Kill()
		for _, pid := range tree {
			_ = proc.Terminate(pid)
		}
		return err
	}
	_ = fetch.Run()
	stop()
	if _, err := os.Stat(mark); err != nil {
		t.Fatalf("plain git fetch did not run remote.origin.uploadpack (%v): the test proves nothing", err)
	}
	_ = os.Remove(mark)

	refused := func(what string, err error, key string) {
		t.Helper()
		var pe *ProjectProgramError
		if !errors.As(err, &pe) || pe.Key != key {
			t.Errorf("%s with the repository's %s: %v", what, key, err)
		}
		if _, err := os.Stat(mark); err == nil {
			t.Errorf("%s ran the repository's %s", what, key)
		}
	}
	// Were a program to run, it would wait like the one above: a short deadline fails fast.
	short := func() context.Context {
		c, cancel := context.WithTimeout(ctx, 20*time.Second)
		t.Cleanup(cancel)
		return c
	}
	refused("Fetch", r.Fetch(short()), "remote.origin.uploadpack")
	refused("Pull", r.Pull(short()), "remote.origin.uploadpack")
	gitIn(t, r, "config", "--local", "--unset", "remote.origin.uploadpack")

	for key, value := range map[string]string{
		"core.sshcommand":           program,
		"core.askpass":              program,
		"credential.helper":         program,
		"remote.origin.receivepack": program,
		"protocol.ext.allow":        "always",
	} {
		setConfig(t, r, "", key, value)
		refused("Push", r.Push(short(), ""), key)
		gitIn(t, r, "config", "--local", "--unset", key)
	}

	// The user's own setting, repeated in the repository, and an empty one are not refused.
	setConfig(t, r, global, "credential.helper", "store")
	setConfig(t, r, "", "credential.helper", "store")
	if err := r.Fetch(ctx); err != nil {
		t.Errorf("Fetch with the user's own credential helper repeated: %v", err)
	}
	gitIn(t, r, "config", "--local", "--unset", "credential.helper")
	setConfig(t, r, "", "credential.helper", "")
	if err := r.Fetch(ctx); err != nil {
		t.Errorf("Fetch with an empty credential.helper: %v", err)
	}
}

// TestInitAndConflicts: Init starts a repository in an empty folder; a merge that conflicts is
// reported in progress with its prepared message, the file has conflict markers, and Abort
// puts the branch and the file back.
func TestInitAndConflicts(t *testing.T) {
	isolateConfig(t)
	ctx := context.Background()
	dir := t.TempDir()
	if _, err := Open(ctx, dir); !errors.Is(err, ErrNoRepo) {
		t.Fatalf("Open before Init: %v", err)
	}
	r, err := Init(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := filepath.EvalSymlinks(r.Root); got != mustEval(t, dir) {
		t.Errorf("Init opened %q, want %q", r.Root, dir)
	}
	for _, kv := range [][2]string{{"user.name", "PyxForge Test"}, {"user.email", "test@example.invalid"},
		{"commit.gpgsign", "false"}, {"core.autocrlf", "false"}} {
		gitIn(t, r, "config", kv[0], kv[1])
	}
	gitIn(t, r, "symbolic-ref", "HEAD", "refs/heads/main")
	write(t, r, "boot.asm", "org 0x7c00\n")
	commitAll(t, r, "one")
	if op, err := r.InProgress(ctx); op != "" || err != nil {
		t.Fatalf("InProgress on a clean branch: %q, %v", op, err)
	}

	gitIn(t, r, "switch", "--quiet", "-c", "other")
	write(t, r, "boot.asm", "org 0x8000\n")
	commitAll(t, r, "other origin")
	gitIn(t, r, "switch", "--quiet", "main")
	write(t, r, "boot.asm", "org 0x7e00\n")
	commitAll(t, r, "main origin")
	if out, err := exec.Command("git", "-C", r.Root, "merge", "other").CombinedOutput(); err == nil {
		t.Fatalf("the merge did not conflict:\n%s", out)
	}

	if op, err := r.InProgress(ctx); op != "merge" || err != nil {
		t.Fatalf("InProgress during the merge: %q, %v", op, err)
	}
	if f := status(t, r).Files; len(f) != 1 || !f[0].Conflicted() {
		t.Errorf("status during the merge: %+v", f)
	}
	if has, err := HasConflictMarkers(filepath.Join(r.Root, "boot.asm")); !has || err != nil {
		t.Errorf("HasConflictMarkers: %v, %v", has, err)
	}
	if msg := r.MergeMessage(ctx); !strings.HasPrefix(msg, "Merge branch 'other'") || strings.Contains(msg, "\n#") {
		t.Errorf("MergeMessage: %q", msg)
	}
	if err := r.Abort(ctx, "merge"); err != nil {
		t.Fatal(err)
	}
	if op, _ := r.InProgress(ctx); op != "" || read(t, r, "boot.asm") != "org 0x7e00\n" || len(status(t, r).Files) != 0 {
		t.Errorf("after Abort: in progress %q, file %q, status %+v", op, read(t, r, "boot.asm"), status(t, r).Files)
	}
	if has, _ := HasConflictMarkers(filepath.Join(r.Root, "boot.asm")); has {
		t.Error("HasConflictMarkers after Abort")
	}
	if err := r.Abort(ctx, "--hard"); err == nil {
		t.Error("Abort accepted an unknown operation")
	}
}

func mustEval(t *testing.T, p string) string {
	t.Helper()
	got, err := filepath.EvalSymlinks(p)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

// TestGitLFSRoundTrip: an LFS-tracked file pushed to a remote on this machine comes back
// whole in a clone, through fetch and pull.
func TestGitLFSRoundTrip(t *testing.T) {
	if err := exec.Command("git", "lfs", "version").Run(); err != nil {
		t.Skip("git-lfs is not installed")
	}
	isolateConfig(t)
	ctx := context.Background()
	bare := newRemote(t)
	a := newRepo(t)
	gitIn(t, a, "lfs", "install", "--skip-repo")
	gitIn(t, a, "lfs", "install", "--local")
	gitIn(t, a, "lfs", "track", "*.bin")
	write(t, a, "disk.bin", "\x00\x01 version one\n")
	commitAll(t, a, "add the disk")
	if err := a.AddRemote(ctx, "origin", bare); err != nil {
		t.Fatal(err)
	}
	if err := a.Push(ctx, "origin"); err != nil {
		t.Fatalf("publish an LFS repository: %v", err)
	}
	b := clone(t, bare)
	if got := read(t, b, "disk.bin"); got != "\x00\x01 version one\n" {
		t.Fatalf("the clone has %q", got)
	}
	write(t, a, "disk.bin", "\x00\x02 version two\n")
	commitAll(t, a, "change the disk")
	if err := a.Push(ctx, ""); err != nil {
		t.Fatal(err)
	}
	if err := b.Fetch(ctx); err != nil {
		t.Fatal(err)
	}
	if err := b.Pull(ctx); err != nil {
		t.Fatalf("Pull an LFS change: %v", err)
	}
	if got := read(t, b, "disk.bin"); got != "\x00\x02 version two\n" {
		t.Errorf("after Pull the clone has %q", got)
	}
	if blob := gitOut(t, b, "cat-file", "blob", "HEAD:disk.bin"); !strings.HasPrefix(blob, "version https://git-lfs.github.com/spec/v1") {
		t.Errorf("disk.bin is not stored as an LFS pointer: %q", blob)
	}
}
