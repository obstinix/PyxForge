package build

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/obstinix/PyxForge/internal/config"
)

// TestRealMSVCDiagnostics builds with Microsoft's compiler, linker and assembler where Visual
// Studio or its Build Tools are installed (every GitHub Windows runner), from a folder whose
// name has a space and a non-ASCII letter, through PyxForge's own build runner (#71). Every
// problem must point at the real file, line and column, with its code. The messages' words are
// not compared: they change between versions.
//
// cl prints file names in its console's code page, not UTF-8, so this also checks that the
// runner reads that code page: before it did, "src dir ü" came back as "src dir \x81".
func TestRealMSVCDiagnostics(t *testing.T) {
	msvcEnv(t)
	root := t.TempDir()
	const dir = "src dir ü"
	src := filepath.Join(root, dir)
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, text := range map[string]string{
		"t.c":        "int f(void);\r\nint main(void) {\r\n    int n = \"x\";\r\n    return y + f() + n;\r\n}\r\n",
		"link.c":     "int f(void);\r\nint main(void) { return f(); }\r\n",
		"boot64.asm": ".code\r\nmain proc\r\n    mov rax, undefined_sym\r\n    ret\r\nmain endp\r\nend\r\n",
	} {
		if err := os.WriteFile(filepath.Join(src, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	compile := exec.Command("cl.exe", "/nologo", "/c", "link.c") // the linker's input
	compile.Dir = src
	if out, err := compile.CombinedOutput(); err != nil {
		t.Fatalf("cl link.c: %v\n%s", err, out)
	}
	toml := "[project]\nname = \"demo\"\n"
	profiles := map[string][]string{
		"fullpath": {"cl", "/nologo", "/c", "/W4", "/diagnostics:column", "/FC", "t.c"},
		"caret":    {"cl", "/nologo", "/c", "/W4", "/diagnostics:caret", "t.c"},
		"link":     {"link", "/nologo", "link.obj", "/OUT:prog.exe"},
		"ml64":     {"ml64", "/nologo", "/c", "boot64.asm"},
	}
	for name, p := range profiles {
		toml += fmt.Sprintf("[profiles.%s]\ntool = %q\nargs = %s\nsource_dir = %q\n", name, p[0], tomlList(p[1:]), dir)
	}
	if err := os.WriteFile(filepath.Join(root, "pyxforge.toml"), []byte(toml), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	type want struct {
		file         string // in src; "" for a message about no particular source file
		line, column int
		severity     string
		code         string
	}
	cases := []struct {
		profile string
		want    []want
	}{
		{"fullpath", []want{{"t.c", 4, 12, "error", "C2065"}, {"t.c", 3, 9, "warning", "C4047"}}},
		{"caret", []want{{"t.c", 4, 12, "error", "C2065"}, {"t.c", 3, 9, "warning", "C4047"}}},
		{"link", []want{{"", 0, 0, "error", "LNK2019"}, {"", 0, 0, "error", "LNK1120"}}},
		{"ml64", []want{{"boot64.asm", 3, 0, "error", "A2006"}}},
	}
	for _, c := range cases {
		t.Run(c.profile, func(t *testing.T) {
			res, err := Run(context.Background(), cfg, []string{c.profile}, Options{Root: root})
			if err != nil || res.OK() {
				t.Fatalf("the build passed or did not run: %+v, %v", res.Steps, err)
			}
			got := res.Diagnostics()
			output := res.Steps[len(res.Steps)-1].Output
			for _, w := range c.want {
				found := false
				for _, d := range got {
					if d.Line == w.line && d.Column == w.column && d.Severity == w.severity &&
						strings.HasPrefix(d.Message, w.code+": ") && sameFile(d.File, w.file, src) {
						found = true
					}
				}
				if !found {
					t.Errorf("no %+v in %+v; the output was:\n%q", w, got, output)
				}
			}
			if len(got) != len(c.want) {
				t.Errorf("%d diagnostics, want %d: %+v", len(got), len(c.want), got)
			}
		})
	}
}

// sameFile says whether a diagnostic's file is name in dir, by the file itself: the compiler
// may spell the folder with its short (8.3) name, as TEMP is on GitHub's runners.
func sameFile(got, name, dir string) bool {
	if name == "" {
		return got == ""
	}
	a, err := os.Stat(got)
	if err != nil {
		return false
	}
	b, err := os.Stat(filepath.Join(dir, name))
	return err == nil && os.SameFile(a, b)
}

// msvcEnv sets up the environment vcvars64.bat does (PATH, INCLUDE, LIB) for the test, from the
// newest Visual Studio or Build Tools with the C++ tools, found with vswhere where the Visual
// Studio Installer puts it.
func msvcEnv(t *testing.T) {
	t.Helper()
	vswhere := filepath.Join(os.Getenv("ProgramFiles(x86)"), "Microsoft Visual Studio", "Installer", "vswhere.exe")
	out, err := exec.Command(vswhere, "-latest", "-products", "*", "-requires",
		"Microsoft.VisualStudio.Component.VC.Tools.x86.x64", "-property", "installationPath").Output()
	install := strings.TrimSpace(string(out))
	if err != nil || install == "" {
		t.Skip("MSVC is not installed (no Visual Studio or Build Tools with the C++ tools)")
	}
	vcvars := filepath.Join(install, "VC", "Auxiliary", "Build", "vcvars64.bat")
	comspec := os.Getenv("ComSpec")
	cmd := exec.Command(comspec)
	// cmd.exe parses its own command line: /s /c "<command>" keeps the quotes around the path.
	cmd.SysProcAttr = &syscall.SysProcAttr{CmdLine: `"` + comspec + `" /s /c ""` + vcvars + `" >nul && set"`}
	out, err = cmd.Output()
	if err != nil {
		t.Fatalf("vcvars64.bat: %v", err)
	}
	for line := range strings.SplitSeq(string(out), "\n") {
		key, value, ok := strings.Cut(strings.TrimRight(line, "\r"), "=")
		if ok && key != "" && os.Getenv(key) != value {
			t.Setenv(key, value)
		}
	}
}
