package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/obstinix/PyxForge/internal/buildinfo"
	"github.com/obstinix/PyxForge/internal/toolchain"
)

type run struct {
	res            Result
	stdout, stderr string
}

func testEnv(t *testing.T, cwd string, st []toolchain.Status) (Env, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	var out, errOut bytes.Buffer
	return Env{
		Ctx:    context.Background(),
		Stdout: &out,
		Stderr: &errOut,
		Getwd:  func() (string, error) { return cwd, nil },
		Detect: func(context.Context) []toolchain.Status { return st },
	}, &out, &errOut
}

func do(t *testing.T, cwd string, st []toolchain.Status, args ...string) run {
	t.Helper()
	env, out, errOut := testEnv(t, cwd, st)
	res := Run(args, env)
	return run{res, out.String(), errOut.String()}
}

func TestOpenIsTheDefault(t *testing.T) {
	dir := t.TempDir()
	for _, args := range [][]string{nil, {"open"}, {dir}, {"open", dir}} {
		r := do(t, dir, nil, args...)
		if !r.res.OpenGUI || r.res.Folder != dir || r.stderr != "" {
			t.Errorf("%v: %+v %q", args, r.res, r.stderr)
		}
	}
	missing := filepath.Join(dir, "missing")
	if r := do(t, dir, nil, missing); r.res.OpenGUI || r.res.Exit != ExitUsage || !strings.Contains(r.stderr, "missing") {
		t.Errorf("a missing folder opened: %+v %q", r.res, r.stderr)
	}
	if r := do(t, dir, nil, "--frobnicate"); r.res.OpenGUI || r.res.Exit != ExitUsage {
		t.Errorf("an unknown option was accepted: %+v", r.res)
	}
}

func TestHelpAndVersion(t *testing.T) {
	r := do(t, ".", nil, "help")
	for _, c := range []string{"open", "info", "doctor", "version", "help"} {
		if !strings.Contains(r.stdout, "  "+c+" ") {
			t.Errorf("help does not list %s:\n%s", c, r.stdout)
		}
	}
	if r := do(t, ".", nil, "--help"); r.res.Exit != ExitOK || !strings.Contains(r.stdout, "Usage:") {
		t.Errorf("--help: %+v", r)
	}
	if r := do(t, ".", nil, "help", "doctor"); !strings.Contains(r.stdout, "pyxforge doctor [--json]") {
		t.Errorf("help doctor: %q", r.stdout)
	}
	if r := do(t, ".", nil, "help", "nope"); r.res.Exit != ExitUsage {
		t.Errorf("help for an unknown command: %+v", r.res)
	}

	r = do(t, ".", nil, "--version")
	if r.res.Exit != ExitOK || !strings.HasPrefix(r.stdout, "pyxforge "+buildinfo.Version+" (go") {
		t.Errorf("--version: %q", r.stdout)
	}
	r = do(t, ".", nil, "version", "--json")
	var info buildinfo.Info
	if err := json.Unmarshal([]byte(r.stdout), &info); err != nil || info.Version != buildinfo.Version {
		t.Errorf("version --json: %v %q", err, r.stdout)
	}
	if r := do(t, ".", nil, "version", "extra"); r.res.Exit != ExitUsage {
		t.Errorf("version took an argument: %+v", r.res)
	}
}

func TestInfo(t *testing.T) {
	repo := t.TempDir()
	write := func(p, s string) {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(repo, ".git", "HEAD"), "ref: refs/heads/main\n")
	write(filepath.Join(repo, "pyxforge.toml"), "")
	sub := filepath.Join(repo, "boot")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}

	r := do(t, sub, nil, "info")
	for _, want := range []string{"Project root  " + repo, filepath.Join(repo, "pyxforge.toml"), "on main"} {
		if !strings.Contains(r.stdout, want) {
			t.Errorf("info is missing %q:\n%s", want, r.stdout)
		}
	}
	r = do(t, repo, nil, "info", "--json", sub) // options before the folder work too
	var got infoJSON
	if err := json.Unmarshal([]byte(r.stdout), &got); err != nil || got.ProjectRoot != repo || got.GitBranch != "main" || got.Folder != sub {
		t.Errorf("info --json: %v %+v", err, got)
	}
	if r := do(t, repo, nil, "info", "a", "b"); r.res.Exit != ExitUsage {
		t.Errorf("info took two folders: %+v", r.res)
	}
	plain := t.TempDir()
	if r := do(t, plain, nil, "info"); !strings.Contains(r.stdout, "none (no pyxforge.toml") || !strings.Contains(r.stdout, "not a Git checkout") {
		t.Errorf("info outside a project:\n%s", r.stdout)
	}
}

func TestDoctor(t *testing.T) {
	nvim := toolchain.Tool{ID: "nvim", Label: "Neovim", Area: toolchain.AreaEditor, Hint: map[string]string{"windows": "x", "linux": "x", "darwin": "x"}}
	qemu := toolchain.Tool{ID: "qemu", Label: "QEMU", Area: toolchain.AreaRun, Hint: map[string]string{"windows": "get qemu", "linux": "get qemu", "darwin": "get qemu"}}
	arm := toolchain.Tool{ID: "arm", Label: "QEMU (ARM)", Area: toolchain.AreaRun, Optional: true}

	all := []toolchain.Status{{Tool: nvim, Path: "/bin/nvim", Version: "0.12.5"}, {Tool: qemu, Path: "/bin/qemu", Version: "11.1.0"}, {Tool: arm}}
	r := do(t, ".", all, "doctor")
	if r.res.Exit != ExitOK || !strings.Contains(r.stdout, "0.12.5") || !strings.Contains(r.stdout, "QEMU (ARM) (optional)") ||
		!strings.Contains(r.stdout, "2 of 3 tools found; every required tool is present.") {
		t.Errorf("doctor with an optional tool missing: exit %d\n%s", r.res.Exit, r.stdout)
	}

	broken := []toolchain.Status{{Tool: nvim, Path: "/bin/nvim", Version: "0.12.5"}, {Tool: qemu}, {Tool: arm}}
	r = do(t, ".", broken, "doctor")
	if r.res.Exit != ExitFailure || !strings.Contains(r.stdout, "install: get qemu") || !strings.Contains(r.stdout, "1 required tool(s) missing") {
		t.Errorf("doctor with QEMU missing: exit %d\n%s", r.res.Exit, r.stdout)
	}

	r = do(t, ".", broken, "doctor", "--json")
	var got struct {
		Tools           []toolJSON
		MissingRequired int
	}
	if err := json.Unmarshal([]byte(r.stdout), &got); err != nil || got.MissingRequired != 1 || len(got.Tools) != 3 ||
		got.Tools[1].Found || got.Tools[1].Hint != "get qemu" || r.res.Exit != ExitFailure {
		t.Errorf("doctor --json: %v %+v exit %d", err, got, r.res.Exit)
	}

	// Ctrl+C during detection is reported, not mistaken for a result.
	env, _, errOut := testEnv(t, ".", all)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	env.Ctx = ctx
	if res := Run([]string{"doctor"}, env); res.Exit != ExitFailure || !strings.Contains(errOut.String(), "interrupted") {
		t.Errorf("interrupted doctor: %+v %q", res, errOut.String())
	}
}
