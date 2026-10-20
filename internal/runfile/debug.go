package runfile

import (
	"errors"
	"path/filepath"
	"strings"
)

// Debug Current File (PXF-FEAT-004, a first part): the file runs under its language's own
// debugger, in a terminal, where the debugger's commands are typed: GDB for C and C++, built
// with debug information and stopped at main; pdb for Python and node inspect for JavaScript,
// both stopped at the first line; Delve for Go. Breakpoints set in the editor are not passed
// on yet.

// ErrNoDebugger means Debug Current File has no debugger for files of this kind.
var ErrNoDebugger = errors.New("not a file Debug Current File knows how to debug")

// Debuggable lists the languages Debug Current File knows, for messages.
const Debuggable = "C and C++ (GDB), Python (pdb), JavaScript (node inspect) and Go (Delve)"

// ForDebug plans debugging file, as For plans running it.
func ForDebug(file, out string, look func(string) (string, error)) (Plan, error) {
	file, err := filepath.Abs(file)
	if err != nil {
		return Plan{}, err
	}
	dir := filepath.Dir(file)
	gdbHint := hint("winget install --id BrechtSanders.WinLibs.POSIX.UCRT --exact", "sudo apt-get install gdb")
	switch strings.ToLower(filepath.Ext(file)) {
	case ".c", ".cpp", ".cc", ".cxx", ".c++":
		p, err := For(file, out, look)
		if err != nil {
			return Plan{}, err
		}
		if _, err := look("gdb"); err != nil {
			return Plan{}, &MissingError{Language: p.Language + " debugging", Tools: []string{"gdb"}, Hint: gdbHint}
		}
		for i, st := range p.Build { // no optimisation, so every line and variable is there
			p.Build[i].Args = append([]string{"-O0"}, st.Args...)
		}
		exe := p.Run.Tool
		// Neither paging, which a short terminal turns on, nor debuginfod's question (Ubuntu
		// sets DEBUGINFOD_URLS) may stop GDB before main; [n] is that question's own default.
		p.Run = Step{Tool: "gdb", Args: []string{"-q", "-ex", "set pagination off", "-ex", "set debuginfod enabled off", "-ex", "break main", "-ex", "run", "--args", exe}, Dir: dir}
		return p, nil
	case ".py", ".pyw":
		p, err := For(file, out, look)
		if err != nil {
			return Plan{}, err
		}
		p.Run.Args = append([]string{"-m", "pdb"}, p.Run.Args...)
		return p, nil
	case ".js", ".mjs", ".cjs":
		p, err := For(file, out, look)
		if err != nil {
			return Plan{}, err
		}
		p.Run.Args = append([]string{"inspect"}, p.Run.Args...)
		return p, nil
	case ".go":
		if _, err := look("dlv"); err != nil {
			return Plan{}, &MissingError{Language: "Go debugging", Tools: []string{"dlv"}, Hint: "go install github.com/go-delve/delve/cmd/dlv@latest"}
		}
		p, err := For(file, out, look)
		if err != nil {
			return Plan{}, err
		}
		target := file
		if len(p.Run.Args) == 2 && p.Run.Args[1] == "." {
			target = "." // a package in a module: debug it whole
		}
		p.Run = Step{Tool: "dlv", Args: []string{"debug", target}, Dir: dir}
		return p, nil
	}
	return Plan{}, ErrNoDebugger
}
