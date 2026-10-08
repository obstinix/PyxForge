package explorer

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"fyne.io/fyne/v2/test"
)

func TestListingAndQuickOpen(t *testing.T) {
	test.NewTempApp(t)
	root := t.TempDir()
	for _, f := range []string{"boot.asm", "Kernel/main.c", "lib/link.ld", ".git/HEAD", "build/boot.bin", ".cache/x"} {
		p := filepath.Join(root, filepath.FromSlash(f))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	e := New(root)

	// Directories first, sorted case-insensitively; .git is hidden. Other dot-directories are
	// listed in the tree but skipped by quick open.
	if got, want := e.childUIDs(""), []string{".cache", "build", "Kernel", "lib", "boot.asm"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("root children = %v, want %v", got, want)
	}
	if !e.isDir["Kernel"] || e.isDir["boot.asm"] {
		t.Error("directory flags wrong")
	}

	files := e.Files(100)
	sort.Strings(files)
	if want := []string{"Kernel/main.c", "boot.asm", "build/boot.bin", "lib/link.ld"}; !reflect.DeepEqual(files, want) {
		t.Errorf("quick-open files = %v, want %v", files, want)
	}
	if n := len(e.Files(2)); n != 2 {
		t.Errorf("limit ignored: %d files", n)
	}

	var opened string
	e.OnOpen = func(p string) { opened = p }
	e.tree.OnSelected("boot.asm")
	if opened != filepath.Join(root, "boot.asm") {
		t.Errorf("opened %q", opened)
	}
}
