package shell

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/test"

	"github.com/obstinix/PyxForge/internal/ui/codeedit"
)

// newNativeShell is a shell whose editor is PyxForge's own (D16), with a queue for work
// posted to the UI thread.
func newNativeShell(t *testing.T, files map[string]string) (*Shell, string, queue) {
	t.Helper()
	root := t.TempDir()
	for name, text := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	q := make(queue, 1024)
	s := NewWithOptions(test.NewTempApp(t), root, Options{Editor: true, Engine: engineNative, Dispatch: q.post})
	noToastExpiry(s)
	t.Cleanup(s.Shutdown)
	return s, root, q
}

// TestNativeEditorInTheShell edits a file through the shell's own paths: open it from the
// workspace, type, save with the Save command, find, go to a line, set a breakpoint, see it
// kept when a line is added above, and close a modified tab, which asks first.
func TestNativeEditorInTheShell(t *testing.T) {
	s, root, q := newNativeShell(t, map[string]string{"notes.txt": "alpha\nbeta\ngamma beta\n"})
	path := filepath.Join(root, "notes.txt")
	if got := s.statusEditor.text.Text; got != noFileOpen {
		t.Errorf("status bar before a file opens: %q", got)
	}
	s.OpenFile(path)
	d := s.activeDoc()
	if d == nil || d.path != path || s.ed != nil {
		t.Fatalf("not open in the native editor (doc %v, neovim host %v)", d, s.ed)
	}
	if d.item.Text != "notes.txt" || d.ed.Buf.Modified() {
		t.Fatalf("a file just opened is titled %q, modified %v", d.item.Text, d.ed.Buf.Modified())
	}
	// Changed on disk with no unsaved changes: selecting the tab reloads it, unmodified.
	os.WriteFile(path, []byte("alpha\nbeta\ngamma beta\n"), 0o644)
	d.mtime = d.mtime.Add(-time.Second)
	s.nat.selected(d.item, false)
	if d.ed.Buf.Modified() || d.item.Text != "notes.txt" {
		t.Errorf("a reload left the tab %q, modified %v", d.item.Text, d.ed.Buf.Modified())
	}
	s.OpenFile(path) // again: the same tab
	if n := len(s.nat.docs); n != 1 {
		t.Fatalf("%d tabs after opening one file twice", n)
	}
	d.ed.SetCursor(codeedit.Pos{Line: 0, Col: 5}, false)
	test.Type(d.ed, "!")
	if d.item.Text != "notes.txt ●" {
		t.Errorf("modified tab is titled %q", d.item.Text)
	}
	if !s.Commands().Run("file.save") {
		t.Fatal("file.save did not run")
	}
	disk, _ := os.ReadFile(path)
	if string(disk) != "alpha!\nbeta\ngamma beta\n" || d.item.Text != "notes.txt" {
		t.Errorf("after Save: disk %q, tab %q", disk, d.item.Text)
	}

	// Find: two matches of "beta", the second selected after Next.
	s.find.open(false)
	s.find.find.SetText("beta")
	s.find.search(false, true)
	if s.find.count.Text != "1 of 2" || d.ed.SelectedText() != "beta" {
		t.Errorf("find: %q, selected %q", s.find.count.Text, d.ed.SelectedText())
	}
	s.find.search(false, false)
	if s.find.count.Text != "2 of 2" || d.ed.Cur.Line != 2 {
		t.Errorf("find next: %q at %v", s.find.count.Text, d.ed.Cur)
	}
	s.find.repl.SetText("BETA")
	s.find.replaceAll()
	if !strings.Contains(d.ed.Buf.Text(), "gamma BETA") || s.find.count.Text != "Replaced 2" {
		t.Errorf("replace all: %q, %q", d.ed.Buf.Text(), s.find.count.Text)
	}
	s.find.close()

	s.goToLineIn("2:3")
	if d.ed.Cur != (codeedit.Pos{Line: 1, Col: 2}) {
		t.Errorf("go to 2:3 put the cursor at %v", d.ed.Cur)
	}
	s.toggleBreakpoint()
	if bp := s.currentBreakpoints(); len(bp) != 1 || bp[0].Lines[0] != 2 {
		t.Fatalf("breakpoints %+v", bp)
	}
	d.ed.SetCursor(codeedit.Pos{Line: 0, Col: 0}, false)
	d.ed.TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn}) // a line above: the breakpoint moves down
	if bp := s.currentBreakpoints(); len(bp) != 1 || bp[0].Lines[0] != 3 {
		t.Errorf("after a line was added above: %+v", s.currentBreakpoints())
	}

	// The editor's own keys: F9 clears the breakpoint on the cursor's line, Ctrl+/ comments.
	d.ed.SetCursor(codeedit.Pos{Line: 2, Col: 0}, false)
	d.ed.TypedKey(&fyne.KeyEvent{Name: fyne.KeyF9})
	if bp := s.currentBreakpoints(); len(bp) != 0 {
		t.Errorf("F9 left %+v", bp)
	}
	d.ed.LineComment = "#"
	d.ed.TypedShortcut(&desktop.CustomShortcut{KeyName: fyne.KeySlash, Modifier: fyne.KeyModifierShortcutDefault})
	if got := string(d.ed.Buf.Line(2)); got != "# BETA" { // Replace All made it BETA
		t.Errorf("Ctrl+/ made line 3 %q", got)
	}

	s.closeEditor() // modified: asks first
	if len(s.nat.docs) != 1 {
		t.Fatal("a modified tab closed without asking")
	}
	tapInDialog(t, s, "Don't Save")
	q.pumpUntil(t, "the tab to close", func() bool { return len(s.nat.docs) == 0 })
	if got := s.statusEditor.text.Text; got != noFileOpen {
		t.Errorf("status bar after the last tab closed: %q", got)
	}
	if disk, _ := os.ReadFile(path); strings.Contains(string(disk), "BETA") {
		t.Error("Don't Save saved")
	}
}

// TestNativeEditorLanguageServer: a C file opened in the native editor starts clangd, whose
// error shows in the editor and in Problems; it runs where clangd is installed.
func TestNativeEditorLanguageServer(t *testing.T) {
	if _, err := exec.LookPath("clangd"); err != nil {
		t.Skip("clangd is not installed")
	}
	// The call is on a line of its own: clangd resolves nothing inside an expression that
	// holds an undeclared name.
	src := "static int twice(int x) { return 2 * x; }\nint main(void) {\n\tint y = twice(1);\n\treturn missing + y;\n}\n"
	s, root, q := newNativeShell(t, map[string]string{"main.c": src})
	s.OpenFile(filepath.Join(root, "main.c"))
	d := s.activeDoc()
	q.pumpWithin(t, "clangd's diagnostics", 90*time.Second, func() bool { return len(d.lspDiags) > 0 })
	if got := d.lspDiags[0]; got.Line != 3 || got.Severity != 1 {
		t.Errorf("diagnostic %+v", got)
	}
	if len(d.ed.Diagnostics()) == 0 {
		t.Error("the editor marks no problem")
	}
	found := false
	for _, p := range s.problems {
		found = found || strings.Contains(p.d.Message, "missing")
	}
	if !found {
		t.Errorf("Problems lists %+v", s.problems)
	}
	if !strings.Contains(s.statusEditor.text.Text, "clangd") {
		t.Errorf("status bar: %q", s.statusEditor.text.Text)
	}
	// F12 on the call goes to the definition; Ctrl+K describes it; F2's rename renames it.
	d.ed.SetCursor(codeedit.Pos{Line: 2, Col: 11}, false) // in "twice"
	d.ed.TypedKey(&fyne.KeyEvent{Name: fyne.KeyF12})
	q.pumpWithin(t, "go to definition", 30*time.Second, func() bool { return d.ed.Cur.Line == 0 })
	d.ed.TypedShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyK, Modifier: fyne.KeyModifierShortcutDefault})
	q.pumpWithin(t, "hover", 30*time.Second, func() bool {
		return strings.Contains(strings.Join(rendered(d.ed), " "), "twice")
	})
	d.ed.SetCursor(codeedit.Pos{Line: 2, Col: 11}, false)
	s.nat.rename(d, "doubled") // not "double": a C keyword, which clangd refuses
	q.pumpWithin(t, "rename", 30*time.Second, func() bool { return logged(s, "Renamed") || logged(s, "Not renamed") })
	if n := strings.Count(d.ed.Buf.Text(), "doubled"); n != 2 {
		t.Errorf("rename made %d changes: %q; log %q", n, d.ed.Buf.Text(), s.logLines[max(len(s.logLines)-3, 0):])
	}
	if strings.Contains(d.ed.Buf.Text(), "twice") {
		t.Errorf("rename left %q", d.ed.Buf.Text())
	}

	// Fix it: the error goes.
	d.ed.SetCursor(codeedit.Pos{Line: 3, Col: 8}, false)
	d.ed.MoveCursor(codeedit.WordRight, true, 1)
	d.ed.Paste("0")
	d.ed.OnEdit()
	q.pumpWithin(t, "the error to clear", 60*time.Second, func() bool { return len(d.lspDiags) == 0 })
}

// rendered is the text an editor draws, its information box included.
func rendered(e *codeedit.Editor) []string {
	var out []string
	for _, o := range test.WidgetRenderer(e).Objects() {
		if tx, ok := o.(*canvas.Text); ok {
			out = append(out, tx.Text)
		}
	}
	return out
}
