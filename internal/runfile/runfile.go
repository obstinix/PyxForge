// Package runfile decides how to run one source file on its own, as its language runs a
// program: through an interpreter (Python, JavaScript, TypeScript, shell and PowerShell
// scripts, Lua, Ruby, Perl, PHP), through the language's own runner (go run, cargo run, Java's
// single-file launch), or by compiling it and then running what was made (C, C++, Rust without
// Cargo). An HTML file is opened in the browser instead. The desktop app's Run Current File
// uses it (PXF-FEAT-003); a project's pyxforge.toml profiles are separate.
package runfile

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/obstinix/PyxForge/internal/toolchain"
)

// Step is one program to run: a name looked up on PATH, or a path.
type Step struct {
	Tool string
	Args []string
	Dir  string // working folder
}

// Plan is how to run a file.
type Plan struct {
	Language string
	Build    []Step            // compile first, in order; empty when the file runs as it is
	Out      string            // the folder Build writes the program into, to create first, or ""
	Run      Step              // the program, run where it can read input (a terminal)
	Open     string            // set instead for a file shown in the browser (HTML)
	Files    map[string]string // to write before Run (path → text): a debugger's commands
	Note     string            // something to tell the user before it runs, or ""
}

// WriteFiles writes the plan's Files.
func (p Plan) WriteFiles() error {
	for path, text := range p.Files {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			return err
		}
	}
	return nil
}

// ErrUnknown means there is no way to run files of this kind.
var ErrUnknown = errors.New("not a file Run Current File knows how to run")

// MissingError means no program for the file's language is installed.
type MissingError struct {
	Language string
	Tools    []string // what was looked for
	Hint     string   // how to install one
}

func (e *MissingError) Error() string {
	msg := fmt.Sprintf("%s needs %s, and none is installed or on PATH", e.Language, strings.Join(e.Tools, " or "))
	if e.Hint != "" {
		msg += "; install: " + e.Hint
	}
	return msg
}

// Known lists the languages Run Current File knows, for messages.
const Known = "Python, JavaScript, TypeScript, Go, Rust, C, C++, assembly (NASM boot sectors, booted in QEMU; GNU .s programs), Java, shell, PowerShell and batch scripts, Lua, Ruby, Perl, PHP, and HTML (opened in the browser)"

// ErrNotBootSector means a NASM file is not a boot sector, the one kind Run Current File can
// build and run alone: anything else needs a format, a linker and an entry point, which a
// project's pyxforge.toml names.
var ErrNotBootSector = errors.New("this assembly file is not a boot sector (org 0x7c00, or the 0xAA55 signature): build it through a pyxforge.toml profile that names its format and linker")

// bootSectorRE spots a BIOS boot sector in NASM source: its load address, its padding to 510
// bytes or its signature.
var bootSectorRE = regexp.MustCompile(`(?im)^[^;\n]*\b(org\s+(0x7c00|7c00h)\b|times\s+510\b|0xaa55\b|0x55\s*,\s*0xaa\b)`)

// pythonNames are tried in order: Doctor's candidates for Python (on Windows the py launcher
// first, which picks the newest Python; python3 is usually the Store's stand-in).
func pythonNames() []string {
	for _, t := range toolchain.Tools {
		if t.ID == "python" {
			return t.Candidates
		}
	}
	return []string{"python3", "python"}
}

// For plans running file. out is a folder, outside the project, for programs compiled from it;
// look finds a program by name.
func For(file, out string, look func(string) (string, error)) (Plan, error) {
	file, err := filepath.Abs(file)
	if err != nil {
		return Plan{}, err
	}
	dir, base := filepath.Dir(file), filepath.Base(file)
	stem := strings.TrimSuffix(base, filepath.Ext(base))
	exe := filepath.Join(outFor(out, file), stem)
	if runtime.GOOS == "windows" {
		exe += ".exe"
	}
	first := func(language, hint string, names ...string) (string, error) {
		for _, n := range names {
			if _, err := look(n); err == nil {
				return n, nil
			}
		}
		return "", &MissingError{Language: language, Tools: names, Hint: hint}
	}
	interpreted := func(language, hint string, args []string, names ...string) (Plan, error) {
		tool, err := first(language, hint, names...)
		if err != nil {
			return Plan{}, err
		}
		return Plan{Language: language, Run: Step{Tool: tool, Args: append(args, file), Dir: dir}}, nil
	}
	compiled := func(language, hint string, flags []string, names ...string) (Plan, error) {
		tool, err := first(language, hint, names...)
		if err != nil {
			return Plan{}, err
		}
		args := append(append([]string{}, flags...), file, "-o", exe)
		return Plan{Language: language, Build: []Step{{Tool: tool, Args: args, Dir: dir}}, Out: filepath.Dir(exe), Run: Step{Tool: exe, Dir: dir}}, nil
	}

	switch ext := strings.ToLower(filepath.Ext(base)); ext {
	case ".py", ".pyw":
		return interpreted("Python", hint("python.org, or winget install --id Python.Python.3.13 --exact", "sudo apt-get install python3"), nil, pythonNames()...)
	case ".js", ".mjs", ".cjs":
		return interpreted("JavaScript", hint("winget install --id OpenJS.NodeJS.LTS --exact", "sudo apt-get install nodejs"), nil, "node")
	case ".ts", ".mts", ".cts":
		if _, err := look("deno"); err == nil {
			return Plan{Language: "TypeScript", Run: Step{Tool: "deno", Args: []string{"run", file}, Dir: dir}}, nil
		}
		return interpreted("TypeScript", "npm install --global tsx, or install Deno from deno.com", nil, "tsx", "ts-node")
	case ".go":
		if _, err := first("Go", hint("winget install --id GoLang.Go --exact", "install Go from go.dev/dl"), "go"); err != nil {
			return Plan{}, err
		}
		if up(dir, "go.mod") != "" { // a package in a module: run it, with its other files
			return Plan{Language: "Go", Run: Step{Tool: "go", Args: []string{"run", "."}, Dir: dir}}, nil
		}
		return Plan{Language: "Go", Run: Step{Tool: "go", Args: []string{"run", file}, Dir: dir}}, nil
	case ".rs":
		if crate := up(dir, "Cargo.toml"); crate != "" {
			if _, err := look("cargo"); err == nil {
				return Plan{Language: "Rust", Run: Step{Tool: "cargo", Args: []string{"run"}, Dir: filepath.Dir(crate)}}, nil
			}
		}
		return compiled("Rust", "install Rust with rustup from rustup.rs", nil, "rustc")
	case ".c":
		return compiled("C", hint("winget install --id MartinStorsjo.LLVM-MinGW.UCRT --exact", "sudo apt-get install gcc"), []string{"-g", "-Wall"}, "gcc", "clang", "cc")
	case ".cpp", ".cc", ".cxx", ".c++":
		return compiled("C++", hint("winget install --id MartinStorsjo.LLVM-MinGW.UCRT --exact", "sudo apt-get install g++"), []string{"-g", "-Wall"}, "g++", "clang++", "c++")
	case ".s": // GNU assembly (.S is preprocessed first): a program with main, built as C is
		return compiled("Assembly", hint("winget install --id MartinStorsjo.LLVM-MinGW.UCRT --exact", "sudo apt-get install gcc"), []string{"-g"}, "gcc", "clang", "cc")
	case ".asm", ".nasm":
		return bootSector(file, dir, filepath.Join(outFor(out, file), stem+".bin"), first)
	case ".java":
		return interpreted("Java", hint("winget install --id Microsoft.OpenJDK.21 --exact", "sudo apt-get install openjdk-21-jdk"), nil, "java")
	case ".sh", ".bash":
		if runtime.GOOS == "windows" {
			if bash := windowsBash(look); bash != "" {
				return Plan{Language: "Shell script", Run: Step{Tool: bash, Args: []string{file}, Dir: dir}}, nil
			}
			return Plan{}, &MissingError{Language: "Shell script", Tools: []string{"bash (Git for Windows)"}, Hint: "winget install --id Git.Git --exact"}
		}
		return interpreted("Shell script", hint("installed with Git for Windows", "installed with the system"), nil, "bash", "sh")
	case ".ps1":
		return interpreted("PowerShell", "install PowerShell from aka.ms/powershell", []string{"-NoProfile", "-File"}, "pwsh", "powershell")
	case ".bat", ".cmd":
		if runtime.GOOS != "windows" {
			return Plan{}, ErrUnknown
		}
		return interpreted("Batch file", "", []string{"/c"}, "cmd")
	case ".lua":
		return interpreted("Lua", "install Lua from lua.org", nil, "lua", "luajit")
	case ".rb":
		return interpreted("Ruby", hint("winget install --id RubyInstallerTeam.Ruby.3.3 --exact", "sudo apt-get install ruby"), nil, "ruby")
	case ".pl":
		return interpreted("Perl", hint("installed with Git for Windows (Git Bash)", "installed with the system"), nil, "perl")
	case ".php":
		return interpreted("PHP", hint("install PHP from php.net", "sudo apt-get install php-cli"), nil, "php")
	case ".html", ".htm", ".xhtml", ".svg":
		return Plan{Language: "HTML", Open: file}, nil
	}
	return Plan{}, ErrUnknown
}

// bootSector plans a NASM boot sector: assembled to a flat 512-byte image, then booted in QEMU
// with its serial port in the terminal (and the screen in QEMU's window).
func bootSector(file, dir, image string, first func(string, string, ...string) (string, error)) (Plan, error) {
	src, err := os.ReadFile(file)
	if err != nil {
		return Plan{}, err
	}
	if !bootSectorRE.Match(src) {
		return Plan{}, ErrNotBootSector
	}
	nasm, err := first("Assembly", hint("winget install --id NASM.NASM --exact", "sudo apt-get install nasm"), "nasm")
	if err != nil {
		return Plan{}, err
	}
	qemu, err := first("QEMU", hint("winget install --id SoftwareFreedomConservancy.QEMU --exact", "sudo apt-get install qemu-system-x86"), "qemu-system-i386", "qemu-system-x86_64")
	if err != nil {
		return Plan{}, err
	}
	drive := "format=raw,file=" + strings.ReplaceAll(image, ",", ",,") // QEMU doubles a comma in a value
	return Plan{Language: "Assembly (boot sector)",
		Build: []Step{{Tool: nasm, Args: []string{"-f", "bin", file, "-o", image}, Dir: dir}},
		Out:   filepath.Dir(image),
		Run:   Step{Tool: qemu, Args: []string{"-drive", drive, "-serial", "stdio"}, Dir: dir}}, nil
}

// outFor is the folder under out for what is made from file: one per source folder, so
// a/main.c and b/main.c do not build the same program, or share its debugger scripts (#22).
func outFor(out, file string) string {
	dir := filepath.Clean(filepath.Dir(file))
	if runtime.GOOS == "windows" {
		dir = strings.ToLower(dir) // one folder however it is spelled
	}
	sum := sha256.Sum256([]byte(dir))
	return filepath.Join(out, hex.EncodeToString(sum[:4]))
}

// windowsBash is a bash for a Windows path: Git for Windows' own, beside git, or else one on
// PATH that is no stand-in. System32's bash.exe starts WSL, whose Linux bash cannot open a
// Windows path, and a WindowsApps bash.exe is a Store alias; in PowerShell or a program started
// from Explorer, System32's comes first on PATH.
func windowsBash(look func(string) (string, error)) string {
	if git, err := look("git"); err == nil {
		d := filepath.Dir(git) // Git\cmd, or Git\mingw64\bin
		for _, c := range []string{filepath.Join(d, "..", "bin", "bash.exe"), filepath.Join(d, "..", "..", "bin", "bash.exe"),
			filepath.Join(d, "..", "usr", "bin", "bash.exe"), filepath.Join(d, "..", "..", "usr", "bin", "bash.exe")} {
			if fi, err := os.Stat(c); err == nil && !fi.IsDir() {
				return filepath.Clean(c)
			}
		}
	}
	for _, n := range []string{"bash", "sh"} {
		if p, err := look(n); err == nil && !standIn(p) {
			return n
		}
	}
	return ""
}

// standIn reports a program that only stands in for the real one: the WSL launcher in System32,
// or a Microsoft Store alias.
func standIn(path string) bool {
	p := strings.ToLower(filepath.Clean(path))
	return strings.Contains(p, `\windows\system32\`) || strings.Contains(p, `\microsoft\windowsapps\`)
}

// hint picks this system's install hint.
func hint(windows, other string) string {
	if runtime.GOOS == "windows" {
		return windows
	}
	return other
}

// up finds name in dir or the folders above it, and returns its path, or "".
func up(dir, name string) string {
	for {
		p := filepath.Join(dir, name)
		if _, err := os.Stat(p); err == nil {
			return p
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}
