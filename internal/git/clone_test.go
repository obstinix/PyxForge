package git

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCloneLocalRepository(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	file := filepath.Join(r.Root, "hello.txt")
	if err := os.WriteFile(file, []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := r.run(ctx, nil, "add", "hello.txt"); err != nil {
		t.Fatal(err)
	}
	if _, err := r.run(ctx, nil, "commit", "-m", "first"); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(t.TempDir(), "cloned repo")
	cloned, err := Clone(ctx, r.Root, dest)
	if err != nil {
		t.Fatal(err)
	}
	if b, err := os.ReadFile(filepath.Join(cloned.Root, "hello.txt")); err != nil || strings.ReplaceAll(string(b), "\r\n", "\n") != "hello\n" {
		t.Fatalf("clone content: %q %v", b, err)
	}
	if _, err := Clone(ctx, r.Root, dest); err == nil {
		t.Fatal("clone accepted existing destination")
	}
	for _, remote := range []string{"", "--upload-pack=bad", "ext::bad"} {
		if _, err := Clone(ctx, remote, filepath.Join(t.TempDir(), "repo")); err == nil {
			t.Errorf("accepted %q", remote)
		}
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := Clone(cancelled, r.Root, filepath.Join(t.TempDir(), "repo")); err == nil {
		t.Fatal("cancelled clone succeeded")
	}
}
