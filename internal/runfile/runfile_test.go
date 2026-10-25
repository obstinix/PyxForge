package runfile

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/obstinix/PyxForge/internal/toolchain"
)

// installed is a look function that finds only names.
func installed(names ...string) func(string) (string, error) {
	return func(n string) (string, error) {
		if slices.Contains(names, n) {
			return "/bin/" + n, nil
		}
		return "", exec.ErrNotFound
	}
}

func write(t *testing.T, path, body string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestSameNameInTwoFolders: a/main.c and b/main.c build different programs, each still named
// main, with debugger scripts of their own; the same folder spelled in another case on Windows
// builds the same one (#22).
func TestSameNameInTwoFolders(t *testing.T) {
	root, out := t.TempDir(), t.TempDir()
	a := write(t, filepath.Join(root, "a", "main.c"), "")
	b := write(t, filepath.Join(root, "b", "main.c"), "")
	look := installed("gcc", "gdb")
	pa, errA := For(a, out, look)
	pb, errB := For(b, out, look)
	if errA != nil || errB != nil {
		t.Fatal(errA, errB)
	}
	if pa.Run.Tool == pb.Run.Tool {
		t.Fatalf("a/main.c and b/main.c both build %s", pa.Run.Tool)
	}
	for _, p := range []Plan{pa, pb} {
		if name := strings.TrimSuffix(filepath.Base(p.Run.Tool), ".exe"); name != "main" || !strings.HasPrefix(p.Run.Tool, out) {
			t.Errorf("program %s: want one named main under %s", p.Run.Tool, out)
		}
	}
	da, _ := ForDebug(a, out, look, nil)
	db, _ := ForDebug(b, out, look, nil)
	for f := range da.Files {
		if _, shared := db.Files[f]; shared {
			t.Errorf("both programs' debuggers use %s", f)
		}
	}
	if runtime.GOOS == "windows" {
		if pu, _ := For(strings.ToUpper(a[:1])+strings.ToUpper(a[1:]), out, look); !strings.EqualFold(pu.Run.Tool, pa.Run.Tool) {
			t.Errorf("the same file spelled in capitals builds %s, not %s", pu.Run.Tool, pa.Run.Tool)
		}
	}
}

// TestPlans: each kind of file gets the runner of its language, the first installed one when
// there are several; a module's or a crate's file runs through go run . or cargo run; compiled
// languages build into out first; HTML opens; what is missing or unknown says so.
func TestPlans(t *testing.T) {
	root, out := t.TempDir(), t.TempDir()
	exe := func(file string) string { // the program built from file
		stem := strings.TrimSuffix(filepath.Base(file), filepath.Ext(file))
		if runtime.GOOS == "windows" {
			stem += ".exe"
		}
		return filepath.Join(outFor(out, filepath.Join(root, file)), stem)
	}
	all := installed("py", "python", "python3", "node", "tsx", "go", "cargo", "rustc", "gcc", "g++", "java", "bash", "pwsh", "powershell", "cmd", "lua", "ruby", "perl", "php")
	py := pythonNames()[0]

	for _, tc := range []struct {
		file  string
		look  func(string) (string, error)
		want  Plan
		build bool // Build is checked by its tool only
	}{
		{file: "a.py", look: all, want: Plan{Language: "Python", Run: Step{Tool: py, Args: []string{"a.py"}}}},
		{file: "a.py", look: installed("python"), want: Plan{Language: "Python", Run: Step{Tool: "python", Args: []string{"a.py"}}}},
		{file: "a.js", look: all, want: Plan{Language: "JavaScript", Run: Step{Tool: "node", Args: []string{"a.js"}}}},
		{file: "a.ts", look: all, want: Plan{Language: "TypeScript", Run: Step{Tool: "tsx", Args: []string{"a.ts"}}}},
		{file: "a.ts", look: installed("deno", "tsx"), want: Plan{Language: "TypeScript", Run: Step{Tool: "deno", Args: []string{"run", "a.ts"}}}},
		{file: "a.go", look: all, want: Plan{Language: "Go", Run: Step{Tool: "go", Args: []string{"run", "a.go"}}}},
		{file: "mod/cmd/main.go", look: all, want: Plan{Language: "Go", Run: Step{Tool: "go", Args: []string{"run", "."}}}},
		{file: "crate/src/main.rs", look: all, want: Plan{Language: "Rust", Run: Step{Tool: "cargo", Args: []string{"run"}}}},
		{file: "lone.rs", look: all, want: Plan{Language: "Rust", Build: []Step{{Tool: "rustc"}}, Run: Step{Tool: exe("lone.rs")}}, build: true},
		{file: "crate/src/main.rs", look: installed("rustc"), want: Plan{Language: "Rust", Build: []Step{{Tool: "rustc"}}, Run: Step{Tool: exe("crate/src/main.rs")}}, build: true},
		{file: "hello.c", look: installed("clang"), want: Plan{Language: "C", Build: []Step{{Tool: "clang"}}, Run: Step{Tool: exe("hello.c")}}, build: true},
		{file: "hello.cpp", look: all, want: Plan{Language: "C++", Build: []Step{{Tool: "g++"}}, Run: Step{Tool: exe("hello.cpp")}}, build: true},
		{file: "Hello.java", look: all, want: Plan{Language: "Java", Run: Step{Tool: "java", Args: []string{"Hello.java"}}}},
		{file: "run.ps1", look: installed("powershell"), want: Plan{Language: "PowerShell", Run: Step{Tool: "powershell", Args: []string{"-NoProfile", "-File", "run.ps1"}}}},
		{file: "a.lua", look: all, want: Plan{Language: "Lua", Run: Step{Tool: "lua", Args: []string{"a.lua"}}}},
		{file: "index.HTML", look: all, want: Plan{Language: "HTML", Open: "index.HTML"}},
	} {
		file := write(t, filepath.Join(root, filepath.FromSlash(tc.file)), "")
		write(t, filepath.Join(root, "mod", "go.mod"), "module example.com/mod\n")
		write(t, filepath.Join(root, "crate", "Cargo.toml"), "[package]\nname = \"crate\"\n")
		got, err := For(file, out, tc.look)
		if err != nil {
			t.Errorf("%s: %v", tc.file, err)
			continue
		}
		want := tc.want
		abs := func(rel string) string { return filepath.Join(filepath.Dir(file), rel) }
		for i, a := range want.Run.Args {
			if a == filepath.Base(file) {
				want.Run.Args[i] = file
			}
		}
		if want.Open != "" {
			want.Open = abs(want.Open)
		}
		switch {
		case got.Language != want.Language || got.Open != want.Open || got.Run.Tool != want.Run.Tool:
			t.Errorf("%s: got %+v, want %+v", tc.file, got, want)
		case !tc.build && !slices.Equal(got.Run.Args, want.Run.Args):
			t.Errorf("%s: run args %q, want %q", tc.file, got.Run.Args, want.Run.Args)
		case tc.build && (len(got.Build) != 1 || got.Build[0].Tool != want.Build[0].Tool ||
			!slices.Contains(got.Build[0].Args, file) || !slices.Contains(got.Build[0].Args, want.Run.Tool)):
			t.Errorf("%s: build %+v", tc.file, got.Build)
		}
	}
	if p, _ := For(filepath.Join(root, "crate", "src", "main.rs"), out, all); p.Run.Dir != filepath.Join(root, "crate") {
		t.Errorf("cargo runs in %q, not the crate's folder", p.Run.Dir)
	}

	var missing *MissingError
	if _, err := For(write(t, filepath.Join(root, "b.py"), ""), out, installed()); !errors.As(err, &missing) || missing.Language != "Python" || missing.Hint == "" {
		t.Errorf("Python missing: %v", err)
	}
	if _, err := For(write(t, filepath.Join(root, "notes.xyz"), ""), out, all); !errors.Is(err, ErrUnknown) {
		t.Errorf("an unknown kind of file: %v", err)
	}
}

// TestRunsForReal builds and runs a small program in each language whose tools this machine
// has, and checks what it prints. Languages without their tools are skipped by name.
func TestRunsForReal(t *testing.T) {
	look := toolchain.LookPath
	for _, tc := range []struct {
		name, file, body string
	}{
		{"Python", "hello.py", "print('hello from Python')\n"},
		{"JavaScript", "hello.js", "console.log('hello from JavaScript')\n"},
		{"Go", "hello.go", "package main\n\nimport \"fmt\"\n\nfunc main() { fmt.Println(\"hello from Go\") }\n"},
		{"C", "hello.c", "#include <stdio.h>\nint main(void) { puts(\"hello from C\"); return 0; }\n"},
		{"C++", "hello.cpp", "#include <iostream>\nint main() { std::cout << \"hello from C++\" << std::endl; }\n"},
		{"Rust", "hello.rs", "fn main() { println!(\"hello from Rust\"); }\n"},
		{"Java", "Hello.java", "class Hello { public static void main(String[] a) { System.out.println(\"hello from Java\"); } }\n"},
		{"Shell script", "hello.sh", "echo 'hello from Shell script'\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir, out := t.TempDir(), t.TempDir()
			file := write(t, filepath.Join(dir, tc.file), tc.body)
			p, err := For(file, out, look)
			var missing *MissingError
			if errors.As(err, &missing) {
				t.Skip(missing.Error())
			}
			if err != nil {
				t.Fatal(err)
			}
			if tc.name == "Python" && runtime.GOOS == "windows" {
				if path, _ := look(p.Run.Tool); strings.Contains(strings.ToLower(path), `\microsoft\windowsapps\`) && p.Run.Tool != "py" {
					t.Skip("only the Microsoft Store's stand-in for Python is installed")
				}
			}
			if p.Out != "" { // the compiled program's folder, as Run Current File makes it
				if err := os.MkdirAll(p.Out, 0o755); err != nil {
					t.Fatal(err)
				}
			}
			for _, st := range p.Build {
				run(t, st)
			}
			if got := strings.TrimSpace(run(t, p.Run)); got != "hello from "+tc.name {
				t.Errorf("printed %q", got)
			}
		})
	}
}

func run(t *testing.T, st Step) string {
	t.Helper()
	path, err := toolchain.LookPath(st.Tool)
	if err != nil {
		path = st.Tool // a program just compiled
	}
	cmd := exec.Command(path, st.Args...)
	cmd.Dir = st.Dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %q: %v\n%s", st.Tool, st.Args, err, out)
	}
	return string(out)
}

// TestDebugPlans: each debuggable kind of file gets its language's debugger, C and C++ built
// without optimisation and stopped at main; a missing debugger names itself; others are
// refused.
func TestDebugPlans(t *testing.T) {
	root, out := t.TempDir(), t.TempDir()
	all := installed("py", "python", "python3", "node", "go", "dlv", "gcc", "g++", "gdb")
	c := write(t, filepath.Join(root, "hello.c"), "")
	p, err := ForDebug(c, out, all, nil)
	if err != nil || p.Run.Tool != "gdb" || len(p.Build) != 1 || p.Build[0].Args[0] != "-O0" ||
		!slices.Contains(p.Build[0].Args, "-g") || len(p.Files) != 2 {
		t.Errorf("C: %+v, %v", p, err)
	}
	var scripts []string // the -x files, in order
	for i, a := range p.Run.Args {
		if a == "-x" && i+1 < len(p.Run.Args) {
			scripts = append(scripts, p.Files[p.Run.Args[i+1]])
		}
		if strings.Contains(a, " ") && !strings.HasPrefix(a, out) { // a launcher may split it
			t.Errorf("GDB argument %q has a space", a)
		}
	}
	// A short terminal would stop GDB at "--Type <RET>"; Ubuntu's would ask about debuginfod.
	if len(scripts) != 2 || scripts[0] != "set pagination off\nset debuginfod enabled off\n" || scripts[1] != "break main\nrun\n" {
		t.Errorf("GDB's command files: %q", scripts)
	}
	if exe := p.Run.Args[len(p.Run.Args)-1]; filepath.Dir(exe) != outFor(out, c) {
		t.Errorf("GDB debugs %q, not the program built into %s", exe, outFor(out, c))
	}
	py := write(t, filepath.Join(root, "a.py"), "")
	if p, err := ForDebug(py, out, all, nil); err != nil || !slices.Equal(p.Run.Args, []string{"-m", "pdb", py}) {
		t.Errorf("Python: %+v, %v", p, err)
	}
	js := write(t, filepath.Join(root, "a.js"), "")
	if p, err := ForDebug(js, out, all, nil); err != nil || p.Run.Tool != "node" || !slices.Equal(p.Run.Args, []string{"inspect", js}) {
		t.Errorf("JavaScript: %+v, %v", p, err)
	}
	gofile := write(t, filepath.Join(root, "a.go"), "")
	if p, err := ForDebug(gofile, out, all, nil); err != nil || p.Run.Tool != "dlv" || !slices.Equal(p.Run.Args, []string{"debug", gofile}) {
		t.Errorf("Go: %+v, %v", p, err)
	}
	var missing *MissingError
	if _, err := ForDebug(gofile, out, installed("go"), nil); !errors.As(err, &missing) || missing.Tools[0] != "dlv" {
		t.Errorf("Go without Delve: %v", err)
	}
	if _, err := ForDebug(c, out, installed("gcc"), nil); !errors.As(err, &missing) || missing.Tools[0] != "gdb" {
		t.Errorf("C without GDB: %v", err)
	}
	if _, err := ForDebug(write(t, filepath.Join(root, "page.html"), ""), out, all, nil); !errors.Is(err, ErrNoDebugger) {
		t.Errorf("HTML: %v", err)
	}

	// With breakpoints from the editor: each debugger gets those of its program and runs to them.
	elsewhere := filepath.Join(t.TempDir(), "elsewhere.c")
	mod := write(t, filepath.Join(root, "pkg", "util.py"), "")
	breaks := Breakpoints{c: {7, 3}, elsewhere: {1}, py: {2}, mod: {9}, js: {1}, gofile: {4}}
	if p, err := ForDebug(c, out, all, breaks); err != nil || p.Files[p.Run.Args[4]] != "break hello.c:3\nbreak hello.c:7\nrun\n" {
		t.Errorf("C with breakpoints: %+v, %v", p.Files, err)
	}
	p, err = ForDebug(py, out, all, breaks)
	want := []string{"-m", "pdb", "-c", "break 2", "-c", "break " + realPath(mod) + ":9", "-c", "continue", py}
	if err != nil || !slices.Equal(p.Run.Args, want) {
		t.Errorf("Python with breakpoints: %q, %v", p.Run.Args, err)
	}
	if p, err := ForDebug(js, out, all, breaks); err != nil || p.Note == "" {
		t.Errorf("JavaScript with breakpoints says nothing: %+v, %v", p, err)
	}
	p, err = ForDebug(gofile, out, all, breaks)
	if err != nil || len(p.Run.Args) != 4 || p.Run.Args[1] != "--init" || p.Files[p.Run.Args[2]] != "break "+filepath.ToSlash(realPath(gofile))+":4\ncontinue\n" {
		t.Errorf("Go with breakpoints: %q, %+v, %v", p.Run.Args, p.Files, err)
	}
}

// TestDebugsForReal starts each debugger this machine has on a small program, gives it a few
// commands and checks it stopped where it should and showed a value.
func TestDebugsForReal(t *testing.T) {
	for _, tc := range []struct {
		name, file, body, commands string
		want                       []string
		breakAt                    int // a breakpoint from the editor, or 0
	}{
		{"C", "hello.c", "#include <stdio.h>\nint main(void) {\n  int answer = 6 * 7;\n  printf(\"%d\\n\", answer);\n  return 0;\n}\n",
			"next\nprint answer\ncontinue\n", []string{"Breakpoint 1, main", "$1 = 42"}, 0},
		{"Python", "hello.py", "answer = 6 * 7\nprint(answer)\n", "next\np answer\ncontinue\n", []string{"(Pdb)", "42"}, 0},
		// A breakpoint from the editor on the printing line: the debugger runs to it.
		{"C at a breakpoint", "hello.c", "#include <stdio.h>\nint main(void) {\n  int answer = 6 * 7;\n  printf(\"%d\\n\", answer);\n  return 0;\n}\n",
			"print answer\ncontinue\n", []string{"hello.c:4", "$1 = 42"}, 4},
		{"Python at a breakpoint", "hello.py", "answer = 6 * 7\nprint(answer)\n", "p answer + 1\ncontinue\n", []string{"hello.py(2)", "43"}, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir, out := t.TempDir(), t.TempDir()
			file := write(t, filepath.Join(dir, tc.file), tc.body)
			var breaks Breakpoints
			if tc.breakAt > 0 {
				breaks = Breakpoints{file: {tc.breakAt}}
			}
			p, err := ForDebug(file, out, toolchain.LookPath, breaks)
			var missing *MissingError
			if errors.As(err, &missing) {
				t.Skip(missing.Error())
			}
			if err != nil {
				t.Fatal(err)
			}
			if p.Out != "" { // the compiled program's folder, as Run Current File makes it
				if err := os.MkdirAll(p.Out, 0o755); err != nil {
					t.Fatal(err)
				}
			}
			for _, st := range p.Build {
				run(t, st)
			}
			if err := p.WriteFiles(); err != nil {
				t.Fatal(err)
			}
			path, err := toolchain.LookPath(p.Run.Tool)
			if err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command(path, p.Run.Args...)
			cmd.Dir, cmd.Stdin = p.Run.Dir, strings.NewReader(tc.commands)
			got, _ := cmd.CombinedOutput() // a debugger told to continue may end with the program's status
			for _, w := range tc.want {
				if !strings.Contains(string(got), w) {
					t.Errorf("the debugger did not show %q:\n%s", w, got)
				}
			}
		})
	}
}

// TestShellScriptBash: a shell script runs under a real bash, never a stand-in: on Windows,
// System32's bash.exe (WSL, which cannot open a Windows path) and a Store alias are passed over.
func TestShellScriptBash(t *testing.T) {
	file := write(t, filepath.Join(t.TempDir(), "run.sh"), "")
	look := func(paths map[string]string) func(string) (string, error) {
		return func(n string) (string, error) {
			if p, ok := paths[n]; ok {
				return p, nil
			}
			return "", exec.ErrNotFound
		}
	}
	if runtime.GOOS != "windows" {
		if p, err := For(file, t.TempDir(), installed("bash")); err != nil || p.Run.Tool != "bash" || !slices.Equal(p.Run.Args, []string{file}) {
			t.Errorf("run.sh: %+v, %v", p, err)
		}
		return
	}
	wsl := map[string]string{"bash": `C:\Windows\System32\bash.exe`, "sh": `C:\Users\x\AppData\Local\Microsoft\WindowsApps\sh.exe`}
	var missing *MissingError
	if _, err := For(file, t.TempDir(), look(wsl)); !errors.As(err, &missing) {
		t.Errorf("only stand-ins installed: %v", err)
	}
	git := filepath.Join(t.TempDir(), "Git")
	write(t, filepath.Join(git, "bin", "bash.exe"), "")
	write(t, filepath.Join(git, "cmd", "git.exe"), "")
	wsl["git"] = filepath.Join(git, "cmd", "git.exe")
	if p, err := For(file, t.TempDir(), look(wsl)); err != nil || p.Run.Tool != filepath.Join(git, "bin", "bash.exe") {
		t.Errorf("Git for Windows' bash: %+v, %v", p.Run, err)
	}
	if p, err := For(file, t.TempDir(), look(map[string]string{"bash": `C:\msys64\usr\bin\bash.exe`})); err != nil || p.Run.Tool != "bash" {
		t.Errorf("another bash on PATH: %+v, %v", p.Run, err)
	}
}
