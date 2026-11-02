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
	var progress []string
	cloned, err := CloneWithProgress(ctx, r.Root, dest, func(line string) { progress = append(progress, line) })
	if err != nil {
		t.Fatal(err)
	}
	if len(progress) == 0 {
		t.Fatal("clone did not report progress")
	}
	if b, err := os.ReadFile(filepath.Join(cloned.Root, "hello.txt")); err != nil || strings.ReplaceAll(string(b), "\r\n", "\n") != "hello\n" {
		t.Fatalf("clone content: %q %v", b, err)
	}
	if _, err := Clone(ctx, r.Root, dest); err == nil {
		t.Fatal("clone accepted existing destination")
	}
	for _, remote := range []string{"", "--upload-pack=bad", "ext::bad"} {
		dest := filepath.Join(t.TempDir(), "repo")
		if _, err := Clone(ctx, remote, dest); err == nil {
			t.Errorf("accepted %q", remote)
		}
		if _, err := os.Lstat(dest); !os.IsNotExist(err) {
			t.Errorf("%q left a destination behind: %v", remote, err)
		}
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	partial := filepath.Join(t.TempDir(), "repo")
	if _, err := Clone(cancelled, r.Root, partial); err == nil {
		t.Fatal("cancelled clone succeeded")
	}
	if _, err := os.Lstat(partial); !os.IsNotExist(err) {
		t.Fatalf("cancelled clone left a partial destination: %v", err)
	}
}
