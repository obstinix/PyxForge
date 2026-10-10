package shell

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2/test"
)

// The test binary doubles as a build tool: with PYX_HELPER=1 it prints its arguments as
// lines (an argument "sleep" waits instead, "fail" exits 1) rather than running tests.
func TestMain(m *testing.M) {
	if os.Getenv("PYX_HELPER") == "1" {
		code := 0
		for _, a := range os.Args[1:] {
			switch a {
			case "sleep":
				time.Sleep(30 * time.Second)
			case "fail":
				code = 1
			default:
				fmt.Println(a)
			}
		}
		os.Exit(code)
	}
	os.Exit(m.Run())
}

func TestBuildPanel(t *testing.T) {
	root := t.TempDir()
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	toml := strings.ReplaceAll(`[project]
name = "demo"

[profiles.ok]
tool = TOOL
args = ["assembled"]
env = { PYX_HELPER = "1" }

[profiles.broken]
tool = TOOL
args = ["boot.asm:7: error: instruction expected", "fail"]
env = { PYX_HELPER = "1" }

[profiles.slow]
tool = TOOL
args = ["sleep"]
env = { PYX_HELPER = "1" }
`, "TOOL", "'"+self+"'")

	a := test.NewTempApp(t)
	q := make(queue, 1024)
	s := NewWithOptions(a, root, Options{Dispatch: q.post})
	b := s.buildp
	buildTab := slices.Index(s.dock.Items, b.tab)

	// Without pyxforge.toml the tab says what to add.
	s.showDockTab(buildTab)
	b.reload()
	if b.body.Objects[0] == b.output {
		t.Fatal("the Build tab shows output controls without a pyxforge.toml")
	}

	if err := os.WriteFile(filepath.Join(root, "pyxforge.toml"), []byte(toml), 0o644); err != nil {
		t.Fatal(err)
	}
	if !b.reload() || b.body.Objects[0] != b.output {
		t.Fatal("the Build tab did not pick up pyxforge.toml")
	}
	if want := []string{defaultTargets, "ok", "broken", "slow"}; !slices.Equal(b.pick.Options, want) {
		t.Errorf("profiles %v, want %v", b.pick.Options, want)
	}

	runBuild := func(profile string) {
		t.Helper()
		b.pick.SetSelected(profile)
		b.start()
		if !b.running || b.stop.Disabled() || !b.run.Disabled() {
			t.Fatal("the build did not start")
		}
		q.pumpUntil(t, profile+" to finish", func() bool { return !b.running })
	}

	// A failing tool: its output streams in and its error joins Problems.
	runBuild("broken")
	if !slices.Contains(b.log.Lines(), "boot.asm:7: error: instruction expected") || !slices.ContainsFunc(b.log.Lines(), func(l string) bool { return strings.HasPrefix(l, "FAILED broken") }) {
		t.Errorf("output %q", b.log.Lines())
	}
	if len(s.problems) != 1 || s.problems[0].path != filepath.Join(root, "boot.asm") || s.problems[0].d.Line != 6 ||
		s.problems[0].d.Severity != 1 || s.problemsTab.Text != "Problems (1)" {
		t.Errorf("problems %+v, tab %q", s.problems, s.problemsTab.Text)
	}
	if !strings.HasPrefix(b.state.Text, "Build failed · 1 error") {
		t.Errorf("state %q", b.state.Text)
	}

	// A good build clears the old build errors.
	runBuild("ok")
	if len(s.problems) != 0 || !strings.HasPrefix(b.state.Text, "Built in") || !slices.Contains(b.log.Lines(), "assembled") {
		t.Errorf("after a good build: problems %+v, state %q, lines %q", s.problems, b.state.Text, b.log.Lines())
	}
	if got := a.Preferences().String(prefBuildPick); got != "ok" {
		t.Errorf("remembered profile %q", got)
	}

	// Stop ends a running build.
	b.pick.SetSelected("slow")
	b.start()
	b.halt()
	q.pumpUntil(t, "the stopped build", func() bool { return !b.running })
	if b.state.Text != "Build stopped" {
		t.Errorf("state %q", b.state.Text)
	}
}

// TestBuildProblemsInTheEditor runs a failing build with a real Neovim: located errors show in
// Problems once (not again from the editor) and in the open file, a message without a file is
// listed under its tool, and next/previous navigation opens the file at the line.
func TestBuildProblemsInTheEditor(t *testing.T) {
	s, root, q := newEditorShell(t)
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	boot := filepath.Join(root, "boot.asm")
	if err := os.WriteFile(boot, []byte("org 0x7c00\nmov ax,\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	toml := strings.ReplaceAll(`[project]
name = "demo"

[profiles.boot]
tool = TOOL
args = ["boot.asm:2: error: comma expected", "ld: cannot find -lkernel", "fail"]
env = { PYX_HELPER = "1" }

[profiles.silent]
tool = TOOL
args = ["fail"]
env = { PYX_HELPER = "1" }
`, "TOOL", "'"+self+"'")
	if err := os.WriteFile(filepath.Join(root, "pyxforge.toml"), []byte(toml), 0o644); err != nil {
		t.Fatal(err)
	}
	s.OpenFile(boot)
	q.pumpUntil(t, "the editor", func() bool { return len(s.editors.Items) == 1 })

	b := s.buildp
	b.reload()
	b.pick.SetSelected("boot")
	b.start()
	q.pumpUntil(t, "the build and the editor's copy", func() bool {
		if b.running {
			return false
		}
		for _, bd := range s.ed.diags {
			for _, d := range bd.items {
				if d.Source == "build: boot" && d.Line == 1 {
					return true
				}
			}
		}
		return false
	})
	q.pumpUntil(t, "the Problems list", func() bool { return len(s.problems) == 2 })
	var places []string
	for _, p := range s.problems {
		places = append(places, s.problemPlace(p))
	}
	if !slices.Contains(places, "boot.asm:2:1") || !slices.Contains(places, "ld") {
		t.Errorf("problems at %q", places)
	}
	if !strings.HasPrefix(s.problemSummary.Text, "2 errors") {
		t.Errorf("summary %q", s.problemSummary.Text)
	}

	// Next Problem opens the file at the error.
	s.stepProblem(1)
	q.pumpUntil(t, "the cursor on line 2", func() bool {
		var line int
		_ = s.ed.sess.ExecLua(`return vim.api.nvim_win_get_cursor(0)[1]`, &line)
		return line == 2
	})

	// A failing tool that printed nothing readable still gets an entry; the old ones go.
	b.pick.SetSelected("silent")
	b.start()
	q.pumpUntil(t, "the second build", func() bool { return !b.running && len(s.problems) == 1 })
	if !strings.Contains(s.problems[0].d.Message, "exited with status 1") || s.problems[0].located() {
		t.Errorf("silent failure: %+v", s.problems[0])
	}
	q.pumpUntil(t, "the editor cleared", func() bool {
		for _, bd := range s.ed.diags {
			for _, d := range bd.items {
				if strings.HasPrefix(d.Source, buildSource) {
					return false
				}
			}
		}
		return true
	})
}
