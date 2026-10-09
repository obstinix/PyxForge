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

	// Directories first, sorted case-insensitively; .git is hidden. Quick open follows the
	// same rules, so .cache is both listed and searchable.
	if got, want := e.childUIDs(""), []string{".cache", "build", "Kernel", "lib", "boot.asm"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("root children = %v, want %v", got, want)
	}
	if !e.isDir["Kernel"] || e.isDir["boot.asm"] {
		t.Error("directory flags wrong")
	}

	files, truncated := e.Files(100)
	sort.Strings(files)
	if want := []string{".cache/x", "Kernel/main.c", "boot.asm", "build/boot.bin", "lib/link.ld"}; truncated || !reflect.DeepEqual(files, want) {
		t.Errorf("quick-open files = %v (truncated %v), want %v", files, truncated, want)
	}
	if files, truncated := e.Files(2); len(files) != 2 || !truncated {
		t.Errorf("limit: %d files, truncated %v", len(files), truncated)
	}
	if _, truncated := e.Files(5); truncated {
		t.Error("an exact fit reported truncation")
	}

	var opened string
	e.OnOpen = func(p string) { opened = p }
	e.tree.OnSelected("boot.asm")
	if opened != filepath.Join(root, "boot.asm") {
		t.Errorf("opened %q", opened)
	}
}
