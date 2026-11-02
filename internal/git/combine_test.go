package git

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// diverged makes a clone whose branch and upstream both have a new commit since they last met:
// the remote changed `remote` to theirs, the clone changed `local` to ours.
func diverged(t *testing.T, remote, theirs, local, ours string) *Repo {
	t.Helper()
	isolateConfig(t)
	ctx := context.Background()
	bare := newRemote(t)
	a := clone(t, bare)
	write(t, a, "boot.asm", "org 0x7c00\n")
	commitAll(t, a, "one")
	if err := a.Push(ctx, "origin"); err != nil {
		t.Fatal(err)
	}
	b := clone(t, bare)
	write(t, a, remote, theirs)
	commitAll(t, a, "theirs")
	if err := a.Push(ctx, ""); err != nil {
		t.Fatal(err)
	}
	write(t, b, local, ours)
	commitAll(t, b, "ours")
	if err := b.Fetch(ctx); err != nil {
		t.Fatal(err)
	}
	if st := status(t, b); st.Ahead != 1 || st.Behind != 1 {
		t.Fatalf("not diverged: ahead %d, behind %d", st.Ahead, st.Behind)
	}
	return b
}

// TestCombineByMerge: diverged branches without conflicts merge in one step, with Git's own
// message and no editor (#66).
func TestCombineByMerge(t *testing.T) {
	ctx := context.Background()
	b := diverged(t, "a.txt", "from the remote\n", "b.txt", "from here\n")
	conflicted, err := b.Combine(ctx, false)
	if err != nil || conflicted {
		t.Fatalf("Combine(merge) = %v, %v", conflicted, err)
	}
	if st := status(t, b); st.Ahead != 2 || st.Behind != 0 {
		t.Errorf("after the merge: ahead %d, behind %d", st.Ahead, st.Behind)
	}
	if read(t, b, "a.txt") != "from the remote\n" || read(t, b, "b.txt") != "from here\n" {
		t.Error("the merge lost a side")
	}
	if msg := gitOut(t, b, "log", "-1", "--format=%s"); !strings.HasPrefix(msg, "Merge ") {
		t.Errorf("merge commit message %q", msg)
	}
}

// TestCombineByRebaseWithAConflict: a rebase that meets a conflict stops in progress; once the
// file is resolved and staged, Continue finishes it, and the branch's commit sits on top of the
// upstream's (#66).
func TestCombineByRebaseWithAConflict(t *testing.T) {
	ctx := context.Background()
	b := diverged(t, "boot.asm", "org 0x7c00\nbits 16\n", "boot.asm", "org 0x7c00\nbits 32\n")
	conflicted, err := b.Combine(ctx, true)
	if err != nil || !conflicted {
		t.Fatalf("Combine(rebase) = %v, %v; want a conflict", conflicted, err)
	}
	if op, _ := b.InProgress(ctx); op != "rebase" {
		t.Fatalf("in progress: %q", op)
	}
	// Continuing before resolving is refused, and the rebase stays.
	if _, err := b.Continue(ctx, "rebase"); err == nil {
		t.Error("Continue with the conflict unresolved succeeded")
	}
	if err := os.WriteFile(filepath.Join(b.Root, "boot.asm"), []byte("org 0x7c00\nbits 16\nbits 32\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitIn(t, b, "add", "boot.asm")
	conflicted, err = b.Continue(ctx, "rebase")
	if err != nil || conflicted {
		t.Fatalf("Continue = %v, %v", conflicted, err)
	}
	if op, _ := b.InProgress(ctx); op != "" {
		t.Errorf("still in progress: %q", op)
	}
	if st := status(t, b); st.Ahead != 1 || st.Behind != 0 {
		t.Errorf("after the rebase: ahead %d, behind %d", st.Ahead, st.Behind)
	}
	if got := gitOut(t, b, "log", "--format=%s", "-3"); strings.Join(strings.Fields(got), " ") != "ours theirs one" {
		t.Errorf("history after the rebase:\n%s", got)
	}
}

// TestContinueAMerge: a merge stopped at a conflict is committed by Continue with Git's
// prepared message once resolved.
func TestContinueAMerge(t *testing.T) {
	ctx := context.Background()
	b := diverged(t, "boot.asm", "theirs\n", "boot.asm", "ours\n")
	if conflicted, err := b.Combine(ctx, false); err != nil || !conflicted {
		t.Fatalf("Combine(merge) = %v, %v; want a conflict", conflicted, err)
	}
	if err := os.WriteFile(filepath.Join(b.Root, "boot.asm"), []byte("both\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitIn(t, b, "add", "boot.asm")
	if conflicted, err := b.Continue(ctx, "merge"); err != nil || conflicted {
		t.Fatalf("Continue = %v, %v", conflicted, err)
	}
	if op, _ := b.InProgress(ctx); op != "" {
		t.Errorf("still in progress: %q", op)
	}
	if msg := gitOut(t, b, "log", "-1", "--format=%s"); !strings.HasPrefix(msg, "Merge ") {
		t.Errorf("merge commit message %q", msg)
	}
	if _, err := b.Continue(ctx, "nothing"); err == nil {
		t.Error("an unknown operation was continued")
	}
}
