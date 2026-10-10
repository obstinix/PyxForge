package build

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/obstinix/PyxForge/internal/config"
)

// The test binary doubles as a build tool: with PYX_HELPER=1 it runs its arguments as a
// script (print:text, exit:N, sleep:seconds, env:NAME, spawn:seconds) instead of the tests.
func TestMain(m *testing.M) {
	if os.Getenv("PYX_HELPER") == "1" {
		helper(os.Args[1:])
		return
	}
	os.Exit(m.Run())
}

func helper(args []string) {
	for _, a := range args {
		op, val, _ := strings.Cut(a, ":")
		switch op {
		case "print":
			fmt.Println(val)
		case "stderr":
			fmt.Fprintln(os.Stderr, val)
		case "env":
			fmt.Println(val + "=" + os.Getenv(val))
		case "sleep":
			n, _ := strconv.Atoi(val)
			time.Sleep(time.Duration(n) * time.Second)
		case "spawn": // a grandchild that outlives us unless the whole tree is stopped
			c := exec.Command(os.Args[0], "sleep:"+val)
			c.Stdout = os.Stdout
			_ = c.Start()
		case "exit":
			n, _ := strconv.Atoi(val)
			os.Exit(n)
		}
	}
}

// project writes a pyxforge.toml whose profiles run the helper, and loads it.
func project(t *testing.T, profiles string) (*config.Config, string) {
	t.Helper()
	root := t.TempDir()
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	body := "[project]\nname = \"demo\"\n\n" + strings.ReplaceAll(profiles, "TOOL", "'"+self+"'")
	if err := os.WriteFile(filepath.Join(root, "pyxforge.toml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := config.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	return c, root
}

func TestOrderAndTargets(t *testing.T) {
	c, err := config.Parse(`
[project]
name = "x"
[profiles.boot]
tool = "nasm"
[profiles.kernel]
tool = "gcc"
[profiles.image]
tool = "make"
depends_on = ["boot", "kernel"]
[profiles.iso]
tool = "make"
depends_on = ["image", "boot"]
[profiles.docs]
tool = "make"
`)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Order(c, "iso", "docs")
	if want := []string{"boot", "kernel", "image", "iso", "docs"}; err != nil || !slices.Equal(got, want) {
		t.Errorf("Order = %v, %v; want %v", got, err, want)
	}
	if got, want := Targets(c), []string{"iso", "docs"}; !slices.Equal(got, want) {
		t.Errorf("Targets = %v, want %v", got, want)
	}
	if _, err := Order(c, "nope"); err == nil {
		t.Error("an unknown profile was accepted")
	}

	// Validate rejects cycles in a file; Order guards on its own too.
	c.Profiles["boot"].DependsOn = []string{"iso"}
	if _, err := Order(c, "iso"); err == nil || !strings.Contains(err.Error(), "Circular dependency detected") {
		t.Errorf("cycle: err = %v", err)
	}
}

func TestRunStreamsOutputAndParsesDiagnostics(t *testing.T) {
	c, root := project(t, `
[profiles.boot]
tool = TOOL
args = ["print:assembling", "stderr:boot.asm:3: warning: label alone on a line", "env:PYX_MODE"]
env = { PYX_HELPER = "1", PYX_MODE = "debug" }

[profiles.image]
tool = TOOL
args = ["print:linking"]
depends_on = ["boot"]
env = { PYX_HELPER = "1" }
`)
	var mu sync.Mutex
	var steps, lines []string
	res, err := Run(context.Background(), c, nil, Options{Root: root,
		OnStep: func(p string, argv []string) { mu.Lock(); steps = append(steps, p); mu.Unlock() },
		OnLine: func(p, l string) { mu.Lock(); lines = append(lines, p+": "+l); mu.Unlock() },
	})
	if err != nil || !res.OK() {
		t.Fatalf("Run = %+v, %v", res, err)
	}
	if want := []string{"boot", "image"}; !slices.Equal(steps, want) {
		t.Errorf("steps %v, want %v", steps, want)
	}
	for _, want := range []string{"boot: assembling", "boot: PYX_MODE=debug", "image: linking"} {
		if !slices.Contains(lines, want) {
			t.Errorf("line %q missing from %q", want, lines)
		}
	}
	d := res.Diagnostics()
	if len(d) != 1 || d[0].File != filepath.Join(root, "boot.asm") || d[0].Line != 3 || d[0].Severity != "warning" {
		t.Errorf("diagnostics %+v", d)
	}
	if _, err := os.Stat(filepath.Join(root, "build")); err != nil {
		t.Errorf("output_dir not created: %v", err)
	}
}

func TestRunStopsAtTheFirstFailure(t *testing.T) {
	c, root := project(t, `
[profiles.boot]
tool = TOOL
args = ["print:boot.asm:7: error: instruction expected", "exit:2"]
env = { PYX_HELPER = "1" }

[profiles.image]
tool = TOOL
depends_on = ["boot"]
env = { PYX_HELPER = "1" }
`)
	res, err := Run(context.Background(), c, []string{"image"}, Options{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	if res.OK() || len(res.Steps) != 1 || res.Steps[0].Exit != 2 {
		t.Fatalf("steps %+v", res.Steps)
	}
	if d := res.Steps[0].Diagnostics; len(d) != 1 || d[0].Message != "instruction expected" {
		t.Errorf("diagnostics %+v", d)
	}
}

func TestRunReportsMissingToolsAndFolders(t *testing.T) {
	c, root := project(t, `
[profiles.asm]
tool = "nasm-not-installed-here"

[profiles.src]
tool = TOOL
source_dir = "no/such/folder"
`)
	res, _ := Run(context.Background(), c, []string{"asm"}, Options{Root: root})
	if st := res.Steps[0]; st.OK() || !strings.Contains(st.Err, "nasm-not-installed-here is not installed") {
		t.Errorf("missing tool: %+v", st)
	}
	res, _ = Run(context.Background(), c, []string{"src"}, Options{Root: root})
	if st := res.Steps[0]; !strings.Contains(st.Err, "source_dir 'no/such/folder' does not exist") {
		t.Errorf("missing folder: %+v", st)
	}
	if !strings.Contains(MissingTool("nasm"), "install: ") {
		t.Errorf("no install hint for nasm: %q", MissingTool("nasm"))
	}
}

func TestCancelStopsTheWholeTree(t *testing.T) {
	c, root := project(t, `
[profiles.slow]
tool = TOOL
args = ["spawn:30", "print:started", "sleep:30"]
env = { PYX_HELPER = "1" }
`)
	ctx, cancel := context.WithCancel(context.Background())
	started := time.Now()
	res, err := Run(ctx, c, nil, Options{Root: root, OnLine: func(_, l string) {
		if l == "started" {
			cancel()
		}
	}})
	if err != nil {
		t.Fatal(err)
	}
	if st := res.Steps[0]; st.Err != "stopped" {
		t.Errorf("step %+v", st)
	}
	if d := time.Since(started); d > 10*time.Second {
		t.Errorf("cancelling took %v: a grandchild kept the build waiting", d)
	}
}

func TestCancelBeforeTheToolStarts(t *testing.T) {
	c, root := project(t, `
[profiles.a]
tool = TOOL
env = { PYX_HELPER = "1" }
`)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	res, err := Run(ctx, c, nil, Options{Root: root})
	if err != nil || len(res.Steps) != 1 || res.Steps[0].Err != "stopped" {
		t.Errorf("Run = %+v, %v", res, err)
	}
}

func TestRunStaysInsideTheProject(t *testing.T) {
	c, root := project(t, `
[profiles.escape]
tool = TOOL
output_dir = "../outside-build"
env = { PYX_HELPER = "1" }

[profiles.elsewhere]
tool = TOOL
source_dir = "../.."
env = { PYX_HELPER = "1" }
`)
	for _, name := range []string{"escape", "elsewhere"} {
		res, err := Run(context.Background(), c, []string{name}, Options{Root: root})
		if err != nil || len(res.Steps) != 1 || !strings.Contains(res.Steps[0].Err, "is outside the project") {
			t.Errorf("%s: %+v, %v", name, res.Steps, err)
		}
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(root), "outside-build")); err == nil {
		t.Error("output_dir was created outside the project")
	}
}
