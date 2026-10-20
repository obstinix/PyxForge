package shell

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

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
	q.pumpUntil(t, "the C program", func() bool { ts := ran("hello.c"); return ts != nil && ts.done })
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
	q.pumpUntil(t, "the second run", func() bool { ts := ran("hello.c"); return ts.buf != before && ts.done && ts.status == 0 })

	// A compile error: Problems has it, and nothing new runs.
	bad := write("broken.c", "int main(void) { return missing; }\n")
	runIt(bad)
	q.pumpUntil(t, "the failed compile", func() bool { return !s.buildp.running && len(s.buildp.diags) > 0 })
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
