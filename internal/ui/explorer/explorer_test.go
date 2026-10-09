package explorer

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"testing"
	"time"

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

func TestWatchCreateExpandReveal(t *testing.T) {
	test.NewTempApp(t)
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "src", "kernel"), 0o755); err != nil {
		t.Fatal(err)
	}
	e := New(root)
	posted := make(chan func(), 16)
	if err := e.Watch(func(f func()) { posted <- f }); err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	e.childUIDs("")
	e.childUIDs("src")

	// A file written by another program appears once the change settles.
	if err := os.WriteFile(filepath.Join(root, "src", "boot.asm"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	select {
	case f := <-posted:
		f()
	case <-time.After(5 * time.Second):
		t.Fatal("no refresh after a file was created on disk")
	}
	if got := e.childUIDs("src"); !slices.Contains(got, "src/boot.asm") {
		t.Errorf("src lists %v", got)
	}

	// Create makes files and folders, parents included, and refuses bad names.
	p, err := e.Create("src/drivers/vga.c", false)
	if err != nil || p != filepath.Join(root, "src", "drivers", "vga.c") {
		t.Fatalf("Create = %q, %v", p, err)
	}
	if !slices.Contains(e.childUIDs("src"), "src/drivers") {
		t.Error("the new folder is not listed")
	}
	if _, err := e.Create("docs", true); err != nil {
		t.Error(err)
	}
	for _, bad := range []string{"", "src/boot.asm", "../escape.c"} {
		if _, err := e.Create(bad, false); err == nil {
			t.Errorf("Create(%q) succeeded", bad)
		}
	}

	// Expanded and Expand round-trip the open folders; Reveal opens the path without opening the file.
	e.Expand([]string{"src/kernel", "src", "gone"})
	if got, want := e.Expanded(), []string{"src", "src/kernel"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Expanded = %v, want %v", got, want)
	}
	opened := ""
	e.OnOpen = func(p string) { opened = p }
	if !e.Reveal(filepath.Join(root, "src", "drivers", "vga.c")) || opened != "" || !e.tree.IsBranchOpen("src/drivers") {
		t.Errorf("Reveal: opened %q, drivers open %v", opened, e.tree.IsBranchOpen("src/drivers"))
	}
	if e.Reveal(filepath.Join(t.TempDir(), "x.c")) {
		t.Error("revealed a file outside the workspace")
	}
}
