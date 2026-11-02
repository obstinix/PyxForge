package shell

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/test"
	"github.com/obstinix/PyxForge/internal/toolchain"
	"github.com/obstinix/PyxForge/internal/ui/theme"
)

// queue is a UI-thread stand-in for tests: Neovim's goroutines post work, the test goroutine
// runs it, so shell state is only touched from one goroutine as it is in the real app.
type queue chan func()

func (q queue) post(f func()) { q <- f }

// pumpUntil runs posted work until cond holds.
func (q queue) pumpUntil(t *testing.T, what string, cond func() bool) {
	t.Helper()
	q.pumpWithin(t, what, 15*time.Second, cond)
}

// pumpWithin is pumpUntil with its own deadline, for waits on outside programs (compilers,
// debuggers) that a loaded machine can slow down.
func (q queue) pumpWithin(t *testing.T, what string, d time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.After(d)
	for !cond() {
		select {
		case f := <-q:
			f()
		case <-time.After(20 * time.Millisecond):
		case <-deadline:
			t.Fatalf("timed out waiting for %s", what)
		}
	}
}

func newEditorShell(t *testing.T) (*Shell, string, queue) {
	t.Helper()
	if _, err := exec.LookPath("nvim"); err != nil {
		t.Skip("nvim is not installed")
	}
	a := test.NewTempApp(t)
	root := t.TempDir()
	q := make(queue, 1024)
	xdg := t.TempDir()
	var env []string
	for _, d := range []string{"CONFIG", "DATA", "STATE", "CACHE"} {
		env = append(env, "XDG_"+d+"_HOME="+filepath.Join(xdg, strings.ToLower(d)))
	}
	s := NewWithOptions(a, root, Options{Editor: true, Dispatch: q.post, NvimRuntime: filepath.Join(xdg, "runtime"), NvimEnv: env})
	t.Cleanup(func() { s.stopEditor(); s.term.stop() })
	return s, root, q
}

// TestEditorTabsFollowNeovimBuffers opens files from the shell into the embedded Neovim and
// checks the round trip: buffers become tabs, edits mark them unsaved, Save writes the file,
// closing a modified tab asks first, and diagnostics fill the Problems panel.
func TestEditorTabsFollowNeovimBuffers(t *testing.T) {
	s, root, q := newEditorShell(t)
	boot := filepath.Join(root, "boot.asm")
	kernel := filepath.Join(root, "kernel.c")
	for p, body := range map[string]string{boot: "org 0x7c00\n", kernel: "int main(void) { return 0; }\n"} {
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	s.OpenFile(boot)
	q.pumpUntil(t, "a tab for boot.asm", func() bool { return len(s.editors.Items) == 1 && s.editors.Items[0].Text == "boot.asm" })
	if !s.editorPane.Visible() || s.ed.bufs[s.ed.current] == nil {
		t.Fatal("the editor pane is not showing the buffer")
	}
	s.OpenFile(kernel)
	q.pumpUntil(t, "a second tab", func() bool { return len(s.editors.Items) == 2 && s.editors.Selected().Text == "kernel.c" })
	s.OpenFile(boot) // already open: selects its tab, no new buffer
	q.pumpUntil(t, "boot.asm selected again", func() bool { return s.editors.Selected().Text == "boot.asm" })
	if len(s.editors.Items) != 2 {
		t.Fatalf("%d tabs after reopening a file", len(s.editors.Items))
	}

	// Typing through the view edits the current buffer in Neovim.
	s.ed.view.TypedRune('O')
	for _, r := range "; stage 1" {
		s.ed.view.TypedRune(r)
	}
	s.ed.sess.Input("<Esc>")
	q.pumpUntil(t, "the unsaved marker", func() bool { return s.editors.Selected().Text == "boot.asm ●" })
	// Neovim back in Normal mode, not the status from before typing: the check below reads the
	// live mode.
	q.pumpUntil(t, "the mode in the status bar", func() bool {
		return strings.HasPrefix(s.ed.sess.Grid().Mode, "normal") && strings.HasPrefix(s.statusEditor.text.Text, "NORMAL · boot.asm ●")
	})
	// A toolchain check that finishes after Neovim attached leaves the editor's status alone.
	s.applyTools([]toolchain.Status{{Tool: toolchain.Tool{ID: "nvim", Label: "Neovim"}, Path: "nvim", Version: "0.12.6"}}, false)
	if got := s.statusEditor.text.Text; !strings.HasPrefix(got, "NORMAL · boot.asm") {
		t.Errorf("a late toolchain check replaced the editor's status with %q", got)
	}
	// The cursor's line and column follow it: after Esc it is on the last character typed, the
	// "1" of "; stage 1" (column 9), on the new first line; then it moves down a line.
	q.pumpUntil(t, "the cursor in the status bar", func() bool { return strings.Contains(s.statusEditor.text.Text, "Ln 1, Col 9") })
	s.ed.sess.Input("j$")
	q.pumpUntil(t, "the moved cursor", func() bool { return strings.Contains(s.statusEditor.text.Text, "Ln 2, Col ") })

	// Ctrl+S, typed in the editor, saves (a Neovim mapping in PyxForge's configuration).
	s.ed.view.TypedShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyS, Modifier: fyne.KeyModifierControl})
	q.pumpUntil(t, "the saved title", func() bool { return s.editors.Selected().Text == "boot.asm" })
	data, _ := os.ReadFile(boot)
	if !strings.HasPrefix(string(data), "; stage 1\norg 0x7c00") {
		t.Fatalf("file on disk: %q", data)
	}

	// Closing a tab with unsaved changes asks; Don't Save drops them and the tab goes.
	s.ed.sess.Input("ggdd")
	q.pumpUntil(t, "modified again", func() bool { return s.editors.Selected().Text == "boot.asm ●" })
	s.closeEditor()
	if s.lastDialog == nil {
		t.Fatal("closing an unsaved tab did not ask")
	}
	s.lastDialog.Hide()
	bt := s.ed.byTab[s.editors.Selected()]
	s.ed.sess.CloseBuffer(bt.buf, false, true) // what Don't Save runs
	q.pumpUntil(t, "the tab to close", func() bool { return len(s.editors.Items) == 1 })
	if data, _ := os.ReadFile(boot); !strings.HasPrefix(string(data), "; stage 1") {
		t.Errorf("Don't Save wrote the file: %q", data)
	}

	// Diagnostics from any Neovim source reach the Problems panel, with a count on the tab.
	if err := s.ed.sess.ExecLua(`vim.diagnostic.set(vim.api.nvim_create_namespace("t"), 0,
  {{ lnum = 0, col = 4, severity = 1, message = "expected ';'", source = "clangd" }})`, nil); err != nil {
		t.Fatal(err)
	}
	q.pumpUntil(t, "a problem", func() bool { return len(s.problems) == 1 })
	if s.problemsTab.Text != "Problems (1)" || s.problems[0].d.Message != "expected ';'" || !s.problemList.Visible() {
		t.Errorf("problems = %+v, tab %q", s.problems, s.problemsTab.Text)
	}

	// The editor takes the PyxForge theme's colours, and follows a theme change.
	normalBg := func() string {
		var bg int
		_ = s.ed.sess.ExecLua(`return vim.api.nvim_get_hl(0, { name = "Normal" }).bg`, &bg)
		return fmt.Sprintf("#%06x", bg)
	}
	want := func() string { b := theme.Current().Surface.Base; return fmt.Sprintf("#%02x%02x%02x", b.R, b.G, b.B) }
	q.pumpUntil(t, "the editor background to match the theme", func() bool { return normalBg() == want() })
	s.Commands().Run("prefs.theme.ink-paper")
	q.pumpUntil(t, "the editor background to follow Ink & Paper", func() bool { return normalBg() == want() })
	var bgOpt string
	_ = s.ed.sess.ExecLua(`return vim.o.background`, &bgOpt)
	if bgOpt != "light" {
		t.Errorf("'background' = %q under a light theme", bgOpt)
	}

	// Ctrl+Shift chords from the editor run shell commands.
	explorerOn := s.bench.explorerOn
	for _, sc := range s.shortcuts {
		if sc.ShortcutName() == chord("E").ShortcutName() {
			s.ed.view.TypedShortcut(sc)
		}
	}
	if s.bench.explorerOn == explorerOn {
		t.Error("Ctrl+Shift+E typed in the editor did not toggle the explorer")
	}
}

// TestEditorUnavailable keeps files openable without Neovim: they get a placeholder that says
// why, and the user is told once.
func TestEditorUnavailable(t *testing.T) {
	a := test.NewTempApp(t)
	root := t.TempDir()
	p := filepath.Join(root, "boot.asm")
	if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	s := NewWithOptions(a, root, Options{Editor: true, NvimPath: filepath.Join(root, "no-nvim"), Dispatch: func(f func()) { f() }})
	noToastExpiry(s)
	s.OpenFile(p)
	if len(s.editors.Items) != 1 || s.notes.Count() != 1 || s.ed == nil || s.ed.err == nil {
		t.Fatalf("tabs %d, notifications %d, editor %+v", len(s.editors.Items), s.notes.Count(), s.ed)
	}
}

// TestTabOrigins: the Log says why each new tab opened: asked for in PyxForge, restored from
// the last session, or entered by Neovim itself (PXF-DEF-001, where tabs were reported to
// appear by themselves at start-up).
func TestTabOrigins(t *testing.T) {
	s, root, q := newEditorShell(t)
	write := func(name string) string {
		p := filepath.Join(root, name)
		if err := os.WriteFile(p, []byte(name+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	a, b, c, d := write("a.asm"), write("b.asm"), write("c.asm"), write("d.asm")
	tabs := func(n int) { q.pumpUntil(t, fmt.Sprint(n, " tabs"), func() bool { return len(s.ed.bufs) == n }) }
	s.OpenFile(a)
	tabs(1)
	s.openFiles([]string{b, c}, "")
	tabs(3)
	if err := s.ed.sess.Command("edit " + d); err != nil {
		t.Fatal(err)
	}
	tabs(4)
	for _, want := range []string{"Tab a.asm: opened in PyxForge", "Tab b.asm: restored from the last session",
		"Tab c.asm: restored from the last session", "Tab d.asm: entered in Neovim"} {
		if !logged(s, want) {
			t.Errorf("the Log does not say %q:\n%s", want, strings.Join(s.logLines, "\n"))
		}
	}
}

// TestStopLine: when a debugger stops at a source line, the editor opens the file and marks the
// line; running on clears the mark; a stop without a source line (a boot sector) marks nothing
// (#58).
func TestStopLine(t *testing.T) {
	s, root, q := newEditorShell(t)
	src := filepath.Join(root, "hello.c")
	if err := os.WriteFile(src, []byte("#include <stdio.h>\nint main(void) {\n  puts(\"hi\");\n  return 0;\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	marked := func() string {
		var file string
		var line int
		var out []any
		if err := s.ed.sess.ExecLua(`return { require("pyxforge.stopline").current() }`, &out); err != nil {
			t.Fatal(err)
		}
		if len(out) == 2 {
			file, _ = out[0].(string)
			switch n := out[1].(type) {
			case int64:
				line = int(n)
			case uint64:
				line = int(n)
			}
		}
		if file == "" {
			return ""
		}
		return fmt.Sprintf("%s:%d", filepath.Base(file), line)
	}
	s.showStopLine(src, 3)
	q.pumpUntil(t, "hello.c open and line 3 marked", func() bool { return s.ed != nil && len(s.ed.bufs) == 1 && marked() == "hello.c:3" })
	if s.activeFile() != src {
		t.Errorf("the editor shows %q", s.activeFile())
	}
	s.clearStopLine()
	q.pumpUntil(t, "the mark cleared", func() bool { return marked() == "" })
	s.showStopLine(src, 4)
	q.pumpUntil(t, "line 4 marked", func() bool { return marked() == "hello.c:4" })
	s.showStopLine("", 0) // stopped in code with no debug information
	q.pumpUntil(t, "no mark", func() bool { return marked() == "" })
}
