package trash

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestFreedesktopTrash: a file and a folder go to files/ with a .trashinfo naming where they
// came from; a second file of the same name gets a name of its own.
func TestFreedesktopTrash(t *testing.T) {
	trash := filepath.Join(t.TempDir(), "Trash")
	work := t.TempDir()
	write := func(rel string) string {
		p := filepath.Join(work, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(rel), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	a := write("boot.asm")
	if err := toTrash(trash, a); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(a); !os.IsNotExist(err) {
		t.Error("the file is still in place")
	}
	if b, _ := os.ReadFile(filepath.Join(trash, "files", "boot.asm")); string(b) != "boot.asm" {
		t.Errorf("trashed file holds %q", b)
	}
	info, _ := os.ReadFile(filepath.Join(trash, "info", "boot.asm.trashinfo"))
	if !strings.HasPrefix(string(info), "[Trash Info]\nPath=") || !strings.Contains(string(info), "boot.asm") || !strings.Contains(string(info), "DeletionDate=") {
		t.Errorf(".trashinfo:\n%s", info)
	}

	again := write("boot.asm")
	if err := toTrash(trash, again); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(trash, "files", "boot.asm.2")); err != nil {
		t.Errorf("the second file of the same name: %v", err)
	}

	write("src/kernel/main.c")
	if err := toTrash(trash, filepath.Join(work, "src")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(trash, "files", "src", "kernel", "main.c")); err != nil {
		t.Errorf("the folder was not trashed whole: %v", err)
	}
	if err := Move(filepath.Join(work, "missing")); err == nil {
		t.Error("a missing file was accepted")
	}
}

// TestMove moves a file to this system's trash for real. It is opt-in, as it leaves a file in
// the Recycle Bin or Trash: PYXFORGE_TEST_TRASH=1 go test ./internal/trash
func TestMove(t *testing.T) {
	if os.Getenv("PYXFORGE_TEST_TRASH") != "1" {
		t.Skip("set PYXFORGE_TEST_TRASH=1 to move a file to this system's trash")
	}
	p := filepath.Join(t.TempDir(), "pyxforge-trash-test.txt")
	if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Move(p); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(p); !os.IsNotExist(err) {
		t.Error("the file is still in place")
	}
}
