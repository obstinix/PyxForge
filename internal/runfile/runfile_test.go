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

// TestPlans: each kind of file gets the runner of its language, the first installed one when
// there are several; a module's or a crate's file runs through go run . or cargo run; compiled
// languages build into out first; HTML opens; what is missing or unknown says so.
func TestPlans(t *testing.T) {
	root, out := t.TempDir(), t.TempDir()
	exe := func(stem string) string {
		if runtime.GOOS == "windows" {
			return filepath.Join(out, stem+".exe")
		}
		return filepath.Join(out, stem)
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
		{file: "lone.rs", look: all, want: Plan{Language: "Rust", Build: []Step{{Tool: "rustc"}}, Run: Step{Tool: exe("lone")}}, build: true},
		{file: "crate/src/main.rs", look: installed("rustc"), want: Plan{Language: "Rust", Build: []Step{{Tool: "rustc"}}, Run: Step{Tool: exe("main")}}, build: true},
		{file: "hello.c", look: installed("clang"), want: Plan{Language: "C", Build: []Step{{Tool: "clang"}}, Run: Step{Tool: exe("hello")}}, build: true},
		{file: "hello.cpp", look: all, want: Plan{Language: "C++", Build: []Step{{Tool: "g++"}}, Run: Step{Tool: exe("hello")}}, build: true},
		{file: "Hello.java", look: all, want: Plan{Language: "Java", Run: Step{Tool: "java", Args: []string{"Hello.java"}}}},
		{file: "run.sh", look: all, want: Plan{Language: "Shell script", Run: Step{Tool: "bash", Args: []string{"run.sh"}}}},
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
