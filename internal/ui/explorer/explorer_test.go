package explorer

import (
	"fmt"
	"image/color"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
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

func TestEmptyRootNeverListsOrWatchesCWD(t *testing.T) {
	test.NewTempApp(t)
	dir := t.TempDir()
	t.Chdir(dir)
	if err := os.WriteFile(filepath.Join(dir, "unrelated.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	e := New("")
	defer e.Close()
	if got := e.childUIDs(""); len(got) != 0 {
		t.Fatalf("listed cwd: %v", got)
	}
	if got, _ := e.Files(100); len(got) != 0 {
		t.Fatalf("walked cwd: %v", got)
	}
	if err := e.Watch(func(f func()) { f() }); err != nil || e.watch != nil {
		t.Fatal("empty explorer started watcher")
	}
	if _, err := e.Create("unexpected.txt", false); err == nil {
		t.Fatal("created a file without a workspace")
	}
	if e.Reveal(filepath.Join(dir, "unrelated.txt")) {
		t.Fatal("revealed cwd file")
	}
	e.Reload()
	if len(e.children) != 0 {
		t.Fatal("reload listed cwd")
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

	// A folder that links outside the workspace is refused.
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "escape")); err == nil {
		if _, err := e.Create("escape/evil.c", false); err == nil || !strings.Contains(err.Error(), "links outside") {
			t.Errorf("Create through a link: %v", err)
		}
		if _, err := os.Stat(filepath.Join(outside, "evil.c")); err == nil {
			t.Error("a file was written outside the workspace")
		}
	} else {
		t.Logf("symbolic links unavailable here (%v); link check not exercised", err)
	}
	if _, err := e.Create("..notes/a.txt", false); err != nil {
		t.Errorf("a name starting with two dots was refused: %v", err)
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

// TestOpenPreviewAndActiveRow: a click or Enter opens a file for editing, Space previews it,
// opening the same row twice opens it twice (Fyne reports no tap on a selected row, so none
// stays selected), and the file shown in the editor is the row drawn selected.
func TestOpenPreviewAndActiveRow(t *testing.T) {
	test.NewTempApp(t)
	root := t.TempDir()
	for _, f := range []string{"a.txt", "b.txt"} {
		if err := os.WriteFile(filepath.Join(root, f), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	e := New(root)
	w := test.NewTempWindow(t, e.Widget())
	w.Resize(fyne.NewSize(300, 200))
	var opened, previewed []string
	e.OnOpen = func(p string) { opened = append(opened, filepath.Base(p)) }
	e.OnPreview = func(p string) { previewed = append(previewed, filepath.Base(p)) }

	e.tree.OnSelected("a.txt") // what a tap does
	e.tree.OnSelected("a.txt") // the same row again
	if fmt.Sprint(opened) != "[a.txt a.txt]" {
		t.Fatalf("two taps on a.txt opened %v", opened)
	}

	w.Canvas().Focus(e.tree)                            // highlights the first row
	e.tree.TypedKey(&fyne.KeyEvent{Name: fyne.KeyDown}) // then the second
	e.tree.TypedKey(&fyne.KeyEvent{Name: fyne.KeySpace})
	e.tree.TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})
	if fmt.Sprint(previewed) != "[b.txt]" || fmt.Sprint(opened) != "[a.txt a.txt b.txt]" {
		t.Fatalf("Space previewed %v; Enter opened %v", previewed, opened)
	}
	e.OnPreview = nil // without a preview handler Space opens
	e.tree.TypedKey(&fyne.KeyEvent{Name: fyne.KeySpace})
	if len(opened) != 4 {
		t.Errorf("Space without OnPreview opened %v", opened)
	}

	// The active file's row carries the selection wash; no other row does.
	wash := func(id string) color.Color {
		row := e.create(false)
		e.update(id, false, row)
		return row.(*fyne.Container).Objects[0].(*canvas.Rectangle).FillColor
	}
	e.SetActive(filepath.Join(root, "b.txt"))
	if e.Active() != "b.txt" || wash("b.txt") == color.Color(color.Transparent) || wash("a.txt") != color.Color(color.Transparent) {
		t.Errorf("active %q: b washed %v, a washed %v", e.Active(), wash("b.txt"), wash("a.txt"))
	}
	e.SetActive(filepath.Join(t.TempDir(), "elsewhere.txt"))
	if e.Active() != "" {
		t.Errorf("a file outside the workspace became the active row %q", e.Active())
	}
}
