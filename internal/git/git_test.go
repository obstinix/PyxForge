package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestParseStatus(t *testing.T) {
	out := strings.Join([]string{
		"# branch.oid 1a2b3c",
		"# branch.head main",
		"# branch.upstream origin/main",
		"# branch.ab +2 -1",
		"1 M. N... 100644 100644 100644 aaa bbb boot/boot.asm",
		"1 .D N... 100644 100644 000000 aaa aaa old file.c",
		"2 R. N... 100644 100644 100644 aaa aaa R100 kernel/main.c",
		"kernel/kmain.c",
		"u UU N... 100644 100644 100644 100644 a b c link.ld",
		"? notes.md",
		"",
	}, "\x00")
	got := parseStatus(out)
	want := Status{Branch: "main", Head: "1a2b3c", Upstream: "origin/main", Ahead: 2, Behind: 1, Files: []File{
		{Path: "boot/boot.asm", Index: 'M', Work: '.'},
		{Path: "old file.c", Index: '.', Work: 'D'},
		{Path: "kernel/main.c", OrigPath: "kernel/kmain.c", Index: 'R', Work: '.'},
		{Path: "link.ld", Index: 'U', Work: 'U'},
		{Path: "notes.md", Index: '?', Work: '?'},
	}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("parseStatus:\n got %+v\nwant %+v", got, want)
	}
	f := want.Files
	if !f[0].Staged() || f[0].Unstaged() || f[1].Staged() || !f[1].Unstaged() || !f[3].Conflicted() || !f[4].Untracked() || f[4].Staged() {
		t.Error("File predicates disagree with the status letters")
	}
	if detached := parseStatus("# branch.oid abc\x00# branch.head (detached)\x00"); detached.Branch != "" || detached.Head != "abc" {
		t.Errorf("detached: %+v", detached)
	}
}

// newRepo makes a repository with a local identity, so commits work on any machine.
func newRepo(t *testing.T) *Repo {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	dir := t.TempDir()
	for _, args := range [][]string{
		{"init", "--quiet"},
		{"config", "user.name", "PyxForge Test"},
		{"config", "user.email", "test@example.invalid"},
		{"config", "commit.gpgsign", "false"},
		{"config", "core.autocrlf", "false"}, // byte-exact checkouts whatever the global config
		{"symbolic-ref", "HEAD", "refs/heads/main"},
	} {
		if out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	r, err := Open(context.Background(), filepath.Join(dir))
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func write(t *testing.T, r *Repo, rel, body string) {
	t.Helper()
	p := filepath.Join(r.Root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func files(t *testing.T, r *Repo) map[string]string {
	t.Helper()
	st, err := r.Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for _, f := range st.Files {
		out[f.Path] = string([]byte{f.Index, f.Work})
	}
	return out
}

func TestRepository(t *testing.T) {
	ctx := context.Background()
	r := newRepo(t)
	if _, err := Open(ctx, t.TempDir()); err != ErrNoRepo {
		t.Errorf("Open outside a repository: %v", err)
	}
	sub := filepath.Join(r.Root, "boot")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if inner, err := Open(ctx, sub); err != nil || !strings.EqualFold(inner.Root, r.Root) {
		t.Errorf("Open from a subfolder: %v, %v", inner, err)
	}

	write(t, r, "boot/boot.asm", "org 0x7c00\n")
	st, err := r.Status(ctx)
	if err != nil || st.Branch != "main" || st.Head != "" || len(st.Files) != 1 || !st.Files[0].Untracked() {
		t.Fatalf("new repository: %+v, %v", st, err)
	}

	// Stage and unstage before the first commit.
	if err := r.Stage(ctx, "boot/boot.asm"); err != nil {
		t.Fatal(err)
	}
	if got := files(t, r)["boot/boot.asm"]; got != "A." {
		t.Errorf("staged: %q", got)
	}
	if err := r.Unstage(ctx, "boot/boot.asm"); err != nil {
		t.Fatal(err)
	}
	if got := files(t, r)["boot/boot.asm"]; got != "??" {
		t.Errorf("unstaged before the first commit: %q", got)
	}

	// Commit, then change, rename and add files.
	if _, err := r.Commit(ctx, "  \n"); err == nil {
		t.Error("an empty message was accepted")
	}
	if err := r.Stage(ctx, "."); err != nil {
		t.Fatal(err)
	}
	hash, err := r.Commit(ctx, "Add the boot sector\n\nFirst commit.")
	if err != nil || hash == "" {
		t.Fatalf("Commit: %q, %v", hash, err)
	}
	write(t, r, "boot/boot.asm", "org 0x7c00\nbits 16\n")
	write(t, r, "kernel/main.c", "int main(void) { return 0; }\n")
	if got := files(t, r); !reflect.DeepEqual(got, map[string]string{"boot/boot.asm": ".M", "kernel/main.c": "??"}) {
		t.Errorf("after edits: %v", got)
	}
	if err := r.Stage(ctx, "boot/boot.asm"); err != nil {
		t.Fatal(err)
	}
	if err := r.Unstage(ctx, "boot/boot.asm"); err != nil {
		t.Fatal(err)
	}
	if got := files(t, r)["boot/boot.asm"]; got != ".M" {
		t.Errorf("unstaged after a commit: %q", got)
	}

	// Show gives the committed text, and nothing for a file HEAD does not have.
	if text, err := r.Show(ctx, "HEAD", "boot/boot.asm"); err != nil || text != "org 0x7c00\n" {
		t.Errorf("Show HEAD: %q, %v", text, err)
	}
	if text, err := r.Show(ctx, "HEAD", "kernel/main.c"); err != nil || text != "" {
		t.Errorf("Show of a new file: %q, %v", text, err)
	}
	if log, err := r.Log(ctx, 5); err != nil || len(log) != 1 || !strings.HasSuffix(log[0], " Add the boot sector") {
		t.Errorf("Log: %q, %v", log, err)
	}
}
