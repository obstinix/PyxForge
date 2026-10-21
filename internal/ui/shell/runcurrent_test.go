package shell

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"github.com/obstinix/PyxForge/internal/runfile"
	"github.com/obstinix/PyxForge/internal/toolchain"
)

// TestRunCurrentFile runs the file in the editor with Run Current File, in a project with no
// pyxforge.toml: a Python script runs in a Terminal session of its own and reads what is
// typed; a C file is compiled in the Build panel, into PyxForge's cache and not the project,
// then run; a compile error goes to Problems and nothing runs; Restart runs the program again.
func TestRunCurrentFile(t *testing.T) {
	s, root, q := newEditorShell(t)
	tm := s.term
	t.Cleanup(func() {
		if t.Failed() && tm.sess != nil {
			g := tm.sess.Grid()
			for r := range g.Rows {
				if l := strings.TrimSpace(g.Line(r)); l != "" {
					t.Logf("terminal %2d| %s", r, l)
				}
			}
		}
	})
	write := func(name, body string) string {
		t.Helper()
		p := filepath.Join(root, name)
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	runIt := func(path string) {
		t.Helper()
		s.OpenFile(path)
		q.pumpUntil(t, "the editor on "+filepath.Base(path), func() bool {
			bt := s.activeBuffer()
			return bt != nil && bt.path != "" && strings.EqualFold(filepath.Clean(bt.path), filepath.Clean(path))
		})
		s.ed.view.TypedShortcut(chord(fyne.KeyF9)) // Ctrl+Shift+F9 in the editor
	}
	ran := func(name string) *termSession {
		for _, ts := range tm.sessions {
			if ts.name == "Run "+name {
				return ts
			}
		}
		return nil
	}

	if _, err := runfile.For(filepath.Join(root, "x.py"), t.TempDir(), toolchain.LookPath); err == nil {
		py := write("greet.py", "name = input('name? ')\nprint('hello', name)\n")
		runIt(py)
		q.pumpUntil(t, "the Python session", func() bool { return ran("greet.py") != nil && tm.sess != nil })
		q.pumpUntil(t, "the prompt", func() bool { return gridCount(tm.sess.Grid(), "name?") > 0 })
		for _, r := range "PyxForge" {
			tm.view.TypedRune(r)
		}
		tm.view.TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})
		q.pumpUntil(t, "the script's exit", func() bool { ts := ran("greet.py"); return ts.done && ts.status == 0 })
		q.pumpUntil(t, "its answer", func() bool { return gridCount(tm.sess.Grid(), "hello PyxForge") > 0 })
	} else {
		t.Log("Python part skipped: ", err)
	}

	if _, err := toolchain.LookPath("gcc"); err != nil {
		if _, err := toolchain.LookPath("clang"); err != nil {
			t.Skip("no C compiler is installed")
		}
	}
	c := write("hello.c", "#include <stdio.h>\nint main(void) { puts(\"hello from C\"); return 0; }\n")
	runIt(c)
	q.pumpWithin(t, "the C program", time.Minute, func() bool { ts := ran("hello.c"); return ts != nil && ts.done })
	q.pumpUntil(t, "its output", func() bool { return gridCount(tm.sess.Grid(), "hello from C") > 0 })
	if ts := ran("hello.c"); ts.status != 0 || ts.argv == nil {
		t.Errorf("C program session %+v", ts)
	}
	if !slices.Contains(texts(tm.body), "hello.c exited.") {
		t.Errorf("the panel says %q, not that hello.c exited", texts(tm.body))
	}
	entries, _ := os.ReadDir(root)
	if names := len(entries); names != 2 { // greet.py or not, hello.c and nothing compiled
		var list []string
		for _, e := range entries {
			list = append(list, e.Name())
		}
		if !slices.Equal(list, []string{"greet.py", "hello.c"}) && !slices.Equal(list, []string{"hello.c"}) {
			t.Errorf("the project folder holds %v: the program was compiled into it", list)
		}
	}

	// Restart runs it again.
	before := ran("hello.c").buf
	tm.current = ran("hello.c")
	tm.restart()
	q.pumpWithin(t, "the second run", time.Minute, func() bool { ts := ran("hello.c"); return ts.buf != before && ts.done && ts.status == 0 })

	// Debug Current File: GDB stops at main in a session of its own; continue runs it out.
	if _, err := toolchain.LookPath("gdb"); err == nil {
		s.OpenFile(c)
		q.pumpUntil(t, "hello.c in the editor", func() bool { bt := s.activeBuffer(); return bt != nil && filepath.Base(bt.path) == "hello.c" })
		s.runCurrentFile(true)
		q.pumpWithin(t, "GDB at main", time.Minute, func() bool {
			return ran("hello.c") != nil && tm.current != nil && tm.current.name == "Debug hello.c" && gridCount(tm.sess.Grid(), "Breakpoint 1, main") > 0
		})
		for _, r := range "continue" {
			tm.view.TypedRune(r)
		}
		tm.view.TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})
		q.pumpWithin(t, "the program run out under GDB", time.Minute, func() bool { return gridCount(tm.sess.Grid(), "exited normally") > 0 })
	} else {
		t.Log("Debug part skipped: gdb is not installed")
	}

	// A breakpoint set with F9 in the editor: GDB runs to it, not to main.
	if _, err := toolchain.LookPath("gdb"); err == nil {
		count := write("count.c", "#include <stdio.h>\nint main(void) {\n  int n = 6 * 7;\n  printf(\"n is %d\\n\", n);\n  return 0;\n}\n")
		s.OpenFile(count)
		q.pumpUntil(t, "count.c in the editor", func() bool { bt := s.activeBuffer(); return bt != nil && filepath.Base(bt.path) == "count.c" })
		s.ed.sess.Input("4G<F9>")
		q.pumpUntil(t, "the breakpoint reported", func() bool {
			return len(s.breakpoints) == 1 && slices.Equal(s.breakpoints[0].Lines, []int{4})
		})
		s.runCurrentFile(true)
		q.pumpWithin(t, "GDB at line 4", time.Minute, func() bool {
			if tm.current == nil || tm.current.name != "Debug count.c" || gridCount(tm.sess.Grid(), "Breakpoint 1, main") == 0 {
				return false
			}
			g := tm.sess.Grid()
			for r := range g.Rows {
				if l := strings.TrimSpace(g.Line(r)); strings.HasPrefix(l, "4") && strings.Contains(l, "printf") {
					return true
				}
			}
			return false
		})
		if gridCount(tm.sess.Grid(), "n is 42") > 0 {
			t.Error("the program ran past its breakpoint")
		}
	}

	// A compile error: Problems has it, and nothing new runs.
	bad := write("broken.c", "int main(void) { return missing; }\n")
	runIt(bad)
	q.pumpWithin(t, "the failed compile", time.Minute, func() bool { return !s.buildp.running && len(s.buildp.diags) > 0 })
	if ran("broken.c") != nil {
		t.Error("a file that did not compile was run")
	}
	if d := s.buildp.diags[0]; !strings.EqualFold(filepath.Clean(d.path), filepath.Clean(bad)) || d.d.Line != 0 || !strings.Contains(d.d.Message, "missing") {
		t.Errorf("the compile error: %+v", d)
	}

	// A kind of file it does not know, said so.
	unknown := write("notes.xyz", "?\n")
	runIt(unknown)
	q.pumpUntil(t, "the refusal", func() bool { return logged(s, "Cannot run notes.xyz") })
}
