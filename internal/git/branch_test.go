package git

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func commitAll(t *testing.T, r *Repo, msg string) {
	t.Helper()
	ctx := context.Background()
	if err := r.Stage(ctx, "."); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Commit(ctx, msg); err != nil {
		t.Fatal(err)
	}
}

func read(t *testing.T, r *Repo, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(r.Root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func names(bs []Branch) (out []string, current string) {
	for _, b := range bs {
		out = append(out, b.Name)
		if b.Current {
			current = b.Name
		}
	}
	return out, current
}

func TestBranches(t *testing.T) {
	ctx := context.Background()
	r := newRepo(t)
	write(t, r, "boot.asm", "org 0x7c00\n")
	commitAll(t, r, "first")

	if err := r.CreateBranch(ctx, "feature/vga", true); err != nil {
		t.Fatal(err)
	}
	if err := r.CreateBranch(ctx, "spare", false); err != nil {
		t.Fatal(err)
	}
	bs, err := r.Branches(ctx)
	got, cur := names(bs)
	if err != nil || strings.Join(got, ",") != "feature/vga,main,spare" || cur != "feature/vga" {
		t.Fatalf("branches %v, current %q, %v", got, cur, err)
	}
	if bs[0].Head == "" || bs[0].Subject != "first" {
		t.Errorf("branch details: %+v", bs[0])
	}
	for _, bad := range []string{"", "-delete", "has space", "a..b", "end."} {
		if err := r.CreateBranch(ctx, bad, false); err == nil {
			t.Errorf("CreateBranch(%q) succeeded", bad)
		}
	}
	if err := r.CreateBranch(ctx, "spare", false); err == nil {
		t.Error("an existing branch was created again")
	}

	// A commit on feature/vga, then a dirty change that would be overwritten by main: Git
	// refuses the switch and nothing changes.
	write(t, r, "boot.asm", "org 0x7c00\nbits 16\n")
	commitAll(t, r, "16-bit")
	write(t, r, "boot.asm", "org 0x7c00\nbits 16\n; edited\n")
	err = r.Switch(ctx, "main")
	if err == nil || !strings.Contains(err.Error(), "would be overwritten") || !strings.Contains(err.Error(), "boot.asm") {
		t.Errorf("switch over a conflicting change: %v", err)
	}
	if bs, _ := r.Branches(ctx); !bs[0].Current || read(t, r, "boot.asm") != "org 0x7c00\nbits 16\n; edited\n" {
		t.Error("a refused switch changed the branch or the file")
	}

	// A change that does not conflict travels with the switch.
	write(t, r, "boot.asm", "org 0x7c00\nbits 16\n")
	write(t, r, "notes.md", "todo\n")
	if err := r.Switch(ctx, "main"); err != nil {
		t.Fatal(err)
	}
	if read(t, r, "boot.asm") != "org 0x7c00\n" || read(t, r, "notes.md") != "todo\n" {
		t.Error("the working tree does not match main after the switch")
	}
	if err := r.Switch(ctx, "nonesuch"); err == nil {
		t.Error("switched to a branch that does not exist")
	}

	// feature/vga has a commit main lacks: deleting needs force; spare is merged.
	if err := r.DeleteBranch(ctx, "feature/vga", false); !errors.Is(err, ErrNotMerged) {
		t.Errorf("delete unmerged: %v", err)
	}
	if err := r.DeleteBranch(ctx, "feature/vga", true); err != nil {
		t.Error(err)
	}
	if err := r.DeleteBranch(ctx, "spare", false); err != nil {
		t.Error(err)
	}
	if err := r.DeleteBranch(ctx, "main", true); err == nil {
		t.Error("deleted the current branch")
	}
	if bs, _ := r.Branches(ctx); len(bs) != 1 {
		t.Errorf("branches left: %+v", bs)
	}
}

func TestStash(t *testing.T) {
	ctx := context.Background()
	r := newRepo(t)
	write(t, r, "boot.asm", "org 0x7c00\n")
	commitAll(t, r, "first")

	if err := r.StashPush(ctx, "", false); !errors.Is(err, ErrNothingToStash) {
		t.Errorf("stash of a clean tree: %v", err)
	}
	write(t, r, "boot.asm", "org 0x7c00\nbits 16\n")
	write(t, r, "new.asm", "nop\n")
	if err := r.StashPush(ctx, "wip 16-bit", true); err != nil {
		t.Fatal(err)
	}
	if read(t, r, "boot.asm") != "org 0x7c00\n" {
		t.Error("the stash did not clean the tree")
	}
	if _, err := os.Stat(filepath.Join(r.Root, "new.asm")); err == nil {
		t.Error("the untracked file was not stashed")
	}
	list, err := r.Stashes(ctx)
	if err != nil || len(list) != 1 || list[0].Ref != "stash@{0}" || !strings.HasSuffix(list[0].Message, "wip 16-bit") || list[0].Date == "" {
		t.Fatalf("stash list %+v, %v", list, err)
	}

	// Apply keeps the entry; pop over a conflicting change fails and keeps it too.
	if err := r.StashApply(ctx, "stash@{0}", false); err != nil {
		t.Fatal(err)
	}
	if read(t, r, "boot.asm") != "org 0x7c00\nbits 16\n" {
		t.Error("apply did not restore the change")
	}
	write(t, r, "boot.asm", "org 0x7c00\n; another edit\n")
	if err := r.Stage(ctx, "boot.asm"); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Commit(ctx, "another"); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(r.Root, "new.asm")); err != nil {
		t.Fatal(err)
	}
	if err := r.StashApply(ctx, "stash@{0}", true); err == nil {
		t.Error("a conflicting pop succeeded")
	}
	if list, _ := r.Stashes(ctx); len(list) != 1 {
		t.Errorf("a failed pop dropped the entry: %+v", list)
	}

	if err := r.StashDrop(ctx, "stash@{0}"); err != nil {
		t.Fatal(err)
	}
	if list, _ := r.Stashes(ctx); len(list) != 0 {
		t.Errorf("after drop: %+v", list)
	}
	for _, bad := range []string{"stash@{9}", "HEAD", "--all"} {
		if err := r.StashDrop(ctx, bad); err == nil {
			t.Errorf("StashDrop(%q) succeeded", bad)
		}
	}
}
