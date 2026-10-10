package toolchain

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestParseVersion(t *testing.T) {
	for out, want := range map[string]string{
		"NVIM v0.12.5\nBuild type: Release":                                    "0.12.5",
		"NASM version 3.01 compiled on Aug  7 2026":                            "3.01",
		"QEMU emulator version 11.1.0 (v11.1.0-12130-ge470268ff4)":             "11.1.0",
		"GNU gdb (GDB for MinGW-W64 x86_64, built by Brecht Sanders, r1) 17.2": "17.2",
		"git version 2.55.0.windows.1":                                         "2.55.0",
		"\n\nLLD 22.1.8 (compatible with GNU linkers)":                         "22.1.8",
		"some tool without a number":                                           "some tool without a number",
		"":                                                                     "",
	} {
		if got := ParseVersion(out); got != want {
			t.Errorf("ParseVersion(%q) = %q, want %q", out, got, want)
		}
	}
}

// programFiles is an absolute install root on every platform the tests run on.
var programFiles = filepath.Join(os.TempDir(), "Program Files")

// fake is an operating system with a fixed set of executables.
func fake(exes map[string]string) *Detector {
	return &Detector{
		LookPath: func(name string) (string, error) {
			if _, ok := exes[name]; ok {
				return name, nil
			}
			return "", exec.ErrNotFound
		},
		Run: func(ctx context.Context, path string, args []string) (string, error) {
			out := exes[path]
			if out == "HANG" {
				<-ctx.Done()
				return "", ctx.Err()
			}
			return out, nil
		},
		Getenv:  func(k string) string { return map[string]string{"ProgramFiles": programFiles}[k] },
		Timeout: 50 * time.Millisecond,
	}
}

func TestDetect(t *testing.T) {
	qemuDir := filepath.Join(programFiles, "qemu", "qemu-system-x86_64")
	d := fake(map[string]string{
		"clang":   "clang version 22.1.8",         // second candidate: gcc is missing
		qemuDir:   "QEMU emulator version 11.1.0", // only in the install folder, not on PATH
		"nvim":    "HANG",                         // found, but never answers
		"git":     "git version 2.55.0",
		"nasm":    "",
		"objdump": "GNU objdump 2.47",
	})
	tools := []Tool{
		{ID: "cc", Candidates: []string{"gcc", "clang"}},
		{ID: "qemu", Candidates: []string{"qemu-system-x86_64"}, Dirs: []string{`${ProgramFiles}\qemu`}},
		{ID: "nvim", Candidates: []string{"nvim"}},
		{ID: "missing", Candidates: []string{"nope"}},
		{ID: "git", Candidates: []string{"git"}},
	}
	if runtime.GOOS != "windows" {
		tools[1].Dirs = []string{"${ProgramFiles}/qemu"}
	}
	st := d.Detect(context.Background(), tools)
	if len(st) != len(tools) {
		t.Fatalf("%d statuses for %d tools", len(st), len(tools))
	}
	check := func(id, path, version string, wantErr bool) {
		t.Helper()
		s, ok := ByID(st, id)
		if !ok {
			t.Fatalf("no status for %s", id)
		}
		if !strings.HasSuffix(s.Path, path) || s.Version != version || (s.Err != nil) != wantErr {
			t.Errorf("%s: path %q version %q err %v", id, s.Path, s.Version, s.Err)
		}
	}
	check("cc", "clang", "22.1.8", false)
	check("qemu", "qemu-system-x86_64", "11.1.0", false)
	check("nvim", "nvim", "", true) // timed out, reported, not hung
	check("git", "git", "2.55.0", false)
	if s, _ := ByID(st, "missing"); s.Found() || s.Err != nil {
		t.Errorf("missing tool reported as %+v", s)
	}
	if s, _ := ByID(st, "nvim"); s.Err == nil || !strings.Contains(s.Err.Error(), "timed out") {
		t.Errorf("hung tool error = %v", s.Err)
	}
}

// TestDetectRealTool runs the real detector on the Go toolchain running this test.
func TestDetectRealTool(t *testing.T) {
	goTool := Tool{ID: "go", Candidates: []string{"go"}, VersionArgs: []string{"version"}}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go is not on PATH")
	}
	s := New().Detect(context.Background(), []Tool{goTool})[0]
	want := strings.TrimPrefix(runtime.Version(), "go")
	if !s.Found() || s.Err != nil || !strings.HasPrefix(want, s.Version) || s.Version == "" {
		t.Errorf("go: path %q version %q err %v; running %s", s.Path, s.Version, s.Err, runtime.Version())
	}
	if !filepath.IsAbs(s.Path) {
		t.Errorf("path %q is not absolute", s.Path)
	}
}

func TestToolsAreWellFormed(t *testing.T) {
	seen := map[string]bool{}
	for _, tool := range Tools {
		if tool.ID == "" || tool.Label == "" || len(tool.Candidates) == 0 || len(tool.VersionArgs) == 0 || seen[tool.ID] {
			t.Errorf("tool %+v is incomplete or duplicated", tool)
		}
		seen[tool.ID] = true
		known := false
		for _, a := range Areas {
			known = known || a == tool.Area
		}
		if !known {
			t.Errorf("%s has unknown area %q", tool.ID, tool.Area)
		}
		if tool.Hint["windows"] == "" || tool.Hint["linux"] == "" {
			t.Errorf("%s has no install hint for Windows or Linux", tool.ID)
		}
	}
	if errors.Is(nil, exec.ErrNotFound) {
		t.Fatal("unreachable")
	}
}

// TestVersionErrorIsNotAVersion: a tool that rejects the version arguments prints an error,
// which must be reported as a failed query, not shown as the tool's version (asm-lsp did this
// with --version).
func TestVersionErrorIsNotAVersion(t *testing.T) {
	d := fake(map[string]string{"asm-lsp": "error: unexpected argument '--version' found\n\nUsage: asm-lsp [COMMAND]"})
	st := d.Detect(context.Background(), []Tool{{ID: "asm-lsp", Candidates: []string{"asm-lsp"}, VersionArgs: []string{"x"}}})
	if !st[0].Found() || st[0].Version != "" || st[0].Err == nil || !strings.Contains(st[0].Err.Error(), "unexpected argument") {
		t.Errorf("status %+v", st[0])
	}
	for _, tool := range Tools {
		if tool.ID == "asm-lsp" && strings.Join(tool.VersionArgs, " ") != "version" {
			t.Errorf("asm-lsp's version arguments are %q; it has a version subcommand", tool.VersionArgs)
		}
	}
}
