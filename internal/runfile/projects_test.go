package runfile

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/obstinix/PyxForge/internal/build"
	"github.com/obstinix/PyxForge/internal/toolchain"
)

// Real programs of more than one file, in a folder whose name has a space and a non-ASCII
// letter, built and run with the tools this machine has (PyxForge's own installs among them):
// what an ordinary Python, Java, C or C++ project needs from Run Current File.

// exec runs a plan as Run Current File does: its build steps, then the program, with input on
// its standard input. It returns the build's output and exit status, and the program's.
func execPlan(t *testing.T, p Plan, input string) (buildOut string, built bool, out string, exit int) {
	t.Helper()
	start := func(st Step, stdin string) (string, int) {
		path, err := toolchain.LookPath(st.Tool)
		if err != nil {
			path = st.Tool // a program just compiled
		}
		cmd := exec.Command(path, st.Args...)
		cmd.Dir = st.Dir
		cmd.Stdin = strings.NewReader(stdin)
		var b bytes.Buffer
		cmd.Stdout, cmd.Stderr = &b, &b
		err = cmd.Run()
		var ee *exec.ExitError
		switch {
		case errors.As(err, &ee):
			return b.String(), ee.ExitCode()
		case err != nil:
			t.Fatalf("%s %q: %v", st.Tool, st.Args, err)
		}
		return b.String(), 0
	}
	if p.Out != "" {
		if err := os.MkdirAll(p.Out, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, st := range p.Build {
		o, code := start(st, "")
		buildOut += o
		if code != 0 {
			return buildOut, false, "", code
		}
	}
	out, exit = start(p.Run, input)
	return buildOut, true, strings.ReplaceAll(out, "\r\n", "\n"), exit
}

func planOrSkip(t *testing.T, file, out string) Plan {
	t.Helper()
	p, err := For(file, out, toolchain.LookPath)
	var missing *MissingError
	if errors.As(err, &missing) {
		t.Skip(missing.Error())
	}
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS == "windows" {
		if path, _ := toolchain.LookPath(p.Run.Tool); strings.Contains(strings.ToLower(path), `\microsoft\windowsapps\`) && p.Run.Tool != "py" {
			t.Skip("only the Microsoft Store's stand-in for " + p.Run.Tool + " is installed")
		}
	}
	return p
}

func TestProjectsForReal(t *testing.T) {
	root := filepath.Join(t.TempDir(), "my projects ü")
	out := t.TempDir()

	t.Run("Python", func(t *testing.T) {
		dir := filepath.Join(root, "python app")
		write(t, filepath.Join(dir, "shapes", "__init__.py"), "from .area import square\n")
		write(t, filepath.Join(dir, "shapes", "area.py"), "def square(side):\n    if side < 0:\n        raise ValueError('a side cannot be negative')\n    return side * side\n")
		main := write(t, filepath.Join(dir, "main.py"), `import json
import sys
from shapes import square

name = input("name? ")
print(f"hello, {name}")
print(json.dumps({"area": square(4)}))
try:
    square(-1)
except ValueError as e:
    print("caught:", e)
print("café ü", file=sys.stdout)
`)
		p := planOrSkip(t, main, out)
		_, _, got, exit := execPlan(t, p, "Ada\n")
		for _, want := range []string{"name? hello, Ada", `{"area": 16}`, "caught: a side cannot be negative"} {
			if !strings.Contains(got, want) {
				t.Errorf("no %q in:\n%s", want, got)
			}
		}
		if exit != 0 {
			t.Errorf("exit %d:\n%s", exit, got)
		}
		// An uncaught exception: a traceback naming the file and line, and a failing status.
		bad := write(t, filepath.Join(dir, "bad.py"), "from shapes import square\nsquare(-2)\n")
		_, _, got, exit = execPlan(t, planOrSkip(t, bad, out), "")
		if exit == 0 || !strings.Contains(got, "ValueError: a side cannot be negative") || !strings.Contains(got, "bad.py\", line 2") {
			t.Errorf("uncaught exception: exit %d:\n%s", exit, got)
		}
	})

	t.Run("Java", func(t *testing.T) {
		dir := filepath.Join(root, "java app")
		write(t, filepath.Join(dir, "com", "example", "util", "Greeter.java"),
			"package com.example.util;\n\npublic class Greeter {\n    public String greet(String name) { return \"hello, \" + name; }\n}\n")
		main := write(t, filepath.Join(dir, "com", "example", "app", "Main.java"), `package com.example.app;

import com.example.util.Greeter;
import java.util.Scanner;

public class Main {
    public static void main(String[] args) {
        Scanner in = new Scanner(System.in);
        System.out.println(new Greeter().greet(in.nextLine()));
        System.out.println(new Helper().twice(21));
    }
}

class Helper {
    int twice(int n) { return n * 2; }
}
`)
		p := planOrSkip(t, main, out)
		if p.Build == nil {
			t.Skip("no javac: a Java runtime without a compiler")
		}
		buildOut, built, got, exit := execPlan(t, p, "Ada\n")
		if !built || exit != 0 || got != "hello, Ada\n42\n" {
			t.Errorf("built %v, exit %d, printed %q; javac said:\n%s", built, exit, got, buildOut)
		}
		// A compile error: javac's message names the file and line, and Problems can read it.
		bad := write(t, filepath.Join(dir, "com", "example", "app", "Bad.java"),
			"package com.example.app;\n\npublic class Bad {\n    public static void main(String[] args) {\n        int n = \"text\";\n    }\n}\n")
		buildOut, built, _, _ = execPlan(t, planOrSkip(t, bad, out), "")
		if built {
			t.Fatal("a type error compiled")
		}
		d := build.ParseDiagnostics(buildOut)
		if len(d) == 0 || !strings.HasSuffix(filepath.ToSlash(d[0].File), "com/example/app/Bad.java") || d[0].Line != 5 || d[0].Severity != "error" {
			t.Errorf("diagnostics %+v from:\n%s", d, buildOut)
		}
	})

	cFamily := func(lang, dir, mainName, main, libName, lib, header, headerText, want, badName, bad string, badLine int) {
		t.Run(lang, func(t *testing.T) {
			write(t, filepath.Join(dir, header), headerText)
			write(t, filepath.Join(dir, libName), lib)
			file := write(t, filepath.Join(dir, mainName), main)
			p := planOrSkip(t, file, out)
			buildOut, built, got, exit := execPlan(t, p, "Ada\n")
			if !built || exit != 0 || got != want {
				t.Errorf("built %v, exit %d, printed %q, want %q; the compiler said:\n%s", built, exit, got, want, buildOut)
			}
			// A second program in the folder, with its own main and an error: only it is built,
			// and the error is read with its file and line.
			badFile := write(t, filepath.Join(dir, badName), bad)
			buildOut, built, _, _ = execPlan(t, planOrSkip(t, badFile, out), "")
			if built {
				t.Fatal("an undeclared name compiled")
			}
			var found bool
			for _, d := range build.ParseDiagnostics(buildOut) {
				if d.Severity == "error" && filepath.Base(d.File) == badName && d.Line == badLine {
					found = true
				}
			}
			if !found {
				t.Errorf("no error at %s:%d in %+v from:\n%s", badName, badLine, build.ParseDiagnostics(buildOut), buildOut)
			}
			// The first program still builds with a second main beside it.
			if _, built, got, _ = execPlan(t, planOrSkip(t, file, out), "Ada\n"); !built || got != want {
				t.Errorf("with %s in the folder: built %v, printed %q", badName, built, got)
			}
		})
	}
	cFamily("C", filepath.Join(root, "c app"),
		"main.c", "#include <stdio.h>\n#include \"greet.h\"\n\nint main(void) {\n    char name[64];\n    if (scanf(\"%63s\", name) != 1) return 1;\n    greet(name);\n    printf(\"%d\\n\", twice(21));\n    return 0;\n}\n",
		"greet.c", "#include <stdio.h>\n#include \"greet.h\"\n\nvoid greet(const char *name) { printf(\"hello, %s\\n\", name); }\nint twice(int n) { return n * 2; }\n",
		"greet.h", "#ifndef GREET_H\n#define GREET_H\nvoid greet(const char *name);\nint twice(int n);\n#endif\n",
		"hello, Ada\n42\n",
		"bad.c", "int main(void) {\n    return missing;\n}\n", 2)
	cFamily("C++", filepath.Join(root, "cpp app"),
		"main.cpp", "#include <iostream>\n#include <string>\n#include <vector>\n#include \"greet.hpp\"\n\nint main() {\n    std::string name;\n    std::getline(std::cin, name);\n    std::cout << greet(name) << \"\\n\";\n    std::vector<int> v{20, 22};\n    std::cout << sum(v) << \"\\n\";\n}\n",
		"greet.cpp", "#include \"greet.hpp\"\n#include <numeric>\n\nstd::string greet(const std::string &name) { return \"hello, \" + name; }\nint sum(const std::vector<int> &v) { return std::accumulate(v.begin(), v.end(), 0); }\n",
		"greet.hpp", "#pragma once\n#include <string>\n#include <vector>\nstd::string greet(const std::string &name);\nint sum(const std::vector<int> &v);\n",
		"hello, Ada\n42\n",
		"bad.cpp", "int main() {\n    return missing;\n}\n", 2)
}

// TestJavaLayout: a file's class and source root follow its package declaration.
func TestJavaLayout(t *testing.T) {
	root := t.TempDir()
	deep := write(t, filepath.Join(root, "src", "com", "example", "Main.java"), "// a comment\npackage com.example;\nclass Main {}\n")
	if class, r := javaLayout(deep); class != "com.example.Main" || r != filepath.Join(root, "src") {
		t.Errorf("packaged: %s in %s", class, r)
	}
	flat := write(t, filepath.Join(root, "Hello.java"), "class Hello {}\n")
	if class, r := javaLayout(flat); class != "Hello" || r != root {
		t.Errorf("no package: %s in %s", class, r)
	}
	odd := write(t, filepath.Join(root, "misc", "Odd.java"), "package com.elsewhere;\nclass Odd {}\n")
	if class, r := javaLayout(odd); class != "com.elsewhere.Odd" || r != filepath.Join(root, "misc") {
		t.Errorf("package not matching its folders: %s in %s", class, r)
	}
}

// TestWithSiblings: the other sources of a program are found; ones with their own main aren't.
func TestWithSiblings(t *testing.T) {
	dir := t.TempDir()
	main := write(t, filepath.Join(dir, "main.c"), "int main(void) { return 0; }\n")
	write(t, filepath.Join(dir, "b.c"), "int b(void) { return 1; }\n")
	write(t, filepath.Join(dir, "a.c"), "/* int main(void) in a comment's first column would count; this one doesn't */\nint a(void) { return 1; }\n")
	write(t, filepath.Join(dir, "other.c"), "#include <stdio.h>\nint\tmain(int argc, char **argv) { return 0; }\n")
	write(t, filepath.Join(dir, "notes.txt"), "int main(void)")
	write(t, filepath.Join(dir, "x.cpp"), "int x() { return 1; }\n")
	if got := withSiblings(main, cExts); strings.Join(got, " ") != "main.c a.c b.c" {
		t.Errorf("got %v", got)
	}
}
