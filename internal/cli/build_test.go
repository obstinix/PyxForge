package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// buildProject writes a pyxforge.toml. The profiles run `go`, which every machine running
// these tests has.
func buildProject(t *testing.T, profiles string) string {
	t.Helper()
	root := t.TempDir()
	body := "[project]\nname = \"demo\"\n\n" + profiles
	if err := os.WriteFile(filepath.Join(root, "pyxforge.toml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestBuild(t *testing.T) {
	root := buildProject(t, `
[profiles.tools]
tool = "go"
args = ["version"]

[profiles.image]
tool = "go"
args = ["env", "GOOS"]
description = "Pretend to link"
depends_on = ["tools"]
`)
	sub := filepath.Join(root, "src")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}

	// From a subfolder: the project is found above it and the target builds after its dependency.
	r := do(t, sub, nil, "build")
	if r.res.Exit != ExitOK || r.res.OpenGUI {
		t.Fatalf("build: %+v\n%s%s", r.res, r.stdout, r.stderr)
	}
	for _, want := range []string{"Building demo: tools, image", "==> tools: go version", "go version go", "==> image: go env GOOS", "ok     image", "Build succeeded: 0 errors, 0 warnings."} {
		if !strings.Contains(r.stdout, want) {
			t.Errorf("output lacks %q:\n%s", want, r.stdout)
		}
	}

	r = do(t, root, nil, "build", "--list")
	if r.res.Exit != ExitOK || !strings.Contains(r.stdout, "image            go  (after tools)  [default]  Pretend to link") {
		t.Errorf("--list:\n%s", r.stdout)
	}

	r = do(t, root, nil, "build", "tools", "--json")
	var got struct {
		OK    bool
		Steps []struct{ Profile, Output string }
	}
	if err := json.Unmarshal([]byte(r.stdout), &got); err != nil || !got.OK || len(got.Steps) != 1 || !strings.HasPrefix(got.Steps[0].Output, "go version") {
		t.Errorf("--json: %v\n%s", err, r.stdout)
	}
}

func TestBuildFailures(t *testing.T) {
	root := buildProject(t, `
[profiles.asm]
tool = "nasm-not-installed-here"
`)
	r := do(t, root, nil, "build")
	if r.res.Exit != ExitFailure || !strings.Contains(r.stdout, "FAILED asm: nasm-not-installed-here is not installed") ||
		!strings.Contains(r.stdout, "Build failed") {
		t.Errorf("missing tool: %+v\n%s", r.res, r.stdout)
	}
	if r := do(t, root, nil, "build", "nope"); r.res.Exit != ExitUsage || !strings.Contains(r.stderr, "nope") {
		t.Errorf("unknown profile: %+v %q", r.res, r.stderr)
	}
	if r := do(t, t.TempDir(), nil, "build"); r.res.Exit != ExitFailure || !strings.Contains(r.stderr, "no pyxforge.toml") {
		t.Errorf("no project: %+v %q", r.res, r.stderr)
	}
}
