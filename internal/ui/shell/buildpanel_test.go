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
	if !slices.Contains(b.lines, "boot.asm:7: error: instruction expected") || !slices.ContainsFunc(b.lines, func(l string) bool { return strings.HasPrefix(l, "FAILED broken") }) {
		t.Errorf("output %q", b.lines)
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
	if len(s.problems) != 0 || !strings.HasPrefix(b.state.Text, "Built in") || !slices.Contains(b.lines, "assembled") {
		t.Errorf("after a good build: problems %+v, state %q, lines %q", s.problems, b.state.Text, b.lines)
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
