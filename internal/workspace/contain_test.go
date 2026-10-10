package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestContains(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	for p, want := range map[string]bool{
		root:                                   true,
		filepath.Join(root, "src", "boot.asm"): true,
		filepath.Join(root, "new", "deep", "x.bin"): true,
		filepath.Join(root, "..notes"):              true,
		filepath.Join(root, ".."):                   false,
		filepath.Join(root, "..", "x"):              false,
		outside:                                     false,
	} {
		if got := Contains(root, p); got != want {
			t.Errorf("Contains(%q) = %v, want %v", p, got, want)
		}
	}
	link := filepath.Join(root, "build")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symbolic links unavailable: %v", err)
	}
	if Contains(root, filepath.Join(link, "boot.bin")) || !Within(root, filepath.Join(link, "boot.bin")) {
		t.Error("a path through a link leaving the workspace was contained")
	}
}
