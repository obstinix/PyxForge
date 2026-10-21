package runfile

import (
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
)

// Debug Current File (PXF-FEAT-004): the file runs under its language's own debugger, in a
// terminal, where the debugger's commands are typed: GDB for C and C++, built with debug
// information; pdb for Python; Delve for Go; node inspect for JavaScript. The breakpoints set in
// the editor are given to the debugger, which runs to the first of them; without any, GDB stops
// at main, and pdb, Delve and node inspect at the start. node inspect takes no breakpoints from
// the command line, so for it they are only mentioned (Plan.Note).

// ErrNoDebugger means Debug Current File has no debugger for files of this kind.
var ErrNoDebugger = errors.New("not a file Debug Current File knows how to debug")

// Debuggable lists the languages Debug Current File knows, for messages.
const Debuggable = "C and C++ (GDB), Python (pdb), JavaScript (node inspect) and Go (Delve)"

// Breakpoints are the editor's breakpoints: absolute file → 1-based lines.
type Breakpoints map[string][]int

// at lists the breakpoints of the files keep accepts, as (file, line), in order.
func (b Breakpoints) at(keep func(file string) bool) []fileLine {
	var out []fileLine
	for f, lines := range b {
		if keep(f) {
			for _, l := range lines {
				out = append(out, fileLine{f, l})
			}
		}
	}
	slices.SortFunc(out, func(a, b fileLine) int {
		if c := strings.Compare(a.file, b.file); c != 0 {
			return c
		}
		return a.line - b.line
	})
	return out
}

type fileLine struct {
	file string
	line int
}

// realPath is p with links resolved and, on Windows, short (8.3) names spelled out: the name a
// debugger gives the file it runs (pdb does; GitHub's Windows runner has a short temp folder).
func realPath(p string) string {
	p = filepath.Clean(p)
	if r, err := filepath.EvalSymlinks(p); err == nil {
		return r
	}
	if dir := filepath.Dir(p); dir != p { // a file that is not there (yet): its folder, resolved
		return filepath.Join(realPath(dir), filepath.Base(p))
	}
	return p
}

// samePath compares paths as the file system does: resolved, and without case on Windows.
func samePath(a, b string) bool {
	a, b = realPath(a), realPath(b)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}
	return a == b
}

// inside reports whether file is in dir or below it, with extension ext.
func inside(dir, ext string) func(string) bool {
	dir = realPath(dir)
	return func(f string) bool {
		rel, err := filepath.Rel(dir, realPath(f))
		return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) &&
			strings.EqualFold(filepath.Ext(f), ext)
	}
}

// ForDebug plans debugging file, as For plans running it, stopping at breaks.
func ForDebug(file, out string, look func(string) (string, error), breaks Breakpoints) (Plan, error) {
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
		// The commands are files, not -ex arguments: a gdb behind a launcher that splits its
		// arguments at spaces (GitHub's Windows runners have one) got "set" and "break" alone.
		// The settings come first, in a file of their own, so a GDB too old for debuginfod
		// stops only that file, and the breakpoints and run still follow. The program is this
		// one file, named as GDB knows it.
		start := ""
		for _, b := range breaks.at(func(f string) bool { return samePath(f, file) }) {
			start += fmt.Sprintf("break %s:%d\n", filepath.Base(file), b.line)
		}
		if start == "" {
			start = "break main\n"
		}
		setup, script := exe+".setup.gdb", exe+".start.gdb"
		p.Files = map[string]string{
			setup:  "set pagination off\nset debuginfod enabled off\n",
			script: start + "run\n",
		}
		p.Run = Step{Tool: "gdb", Args: []string{"-q", "-x", setup, "-x", script, "--args", exe}, Dir: dir}
		return p, nil
	case ".py", ".pyw":
		p, err := For(file, out, look)
		if err != nil {
			return Plan{}, err
		}
		// The script's own lines are given alone: pdb starts in it, under its resolved name, and
		// a breakpoint under another spelling of the path (a short name) would never be hit.
		// The modules beside or below it are named by their resolved paths.
		var args []string
		for _, b := range breaks.at(inside(dir, ".py")) {
			at := fmt.Sprintf("%s:%d", realPath(b.file), b.line)
			if samePath(b.file, file) {
				at = fmt.Sprint(b.line)
			}
			args = append(args, "-c", "break "+at)
		}
		if args != nil {
			args = append(args, "-c", "continue")
		}
		p.Run.Args = append(append([]string{"-m", "pdb"}, args...), p.Run.Args...)
		return p, nil
	case ".js", ".mjs", ".cjs":
		p, err := For(file, out, look)
		if err != nil {
			return Plan{}, err
		}
		p.Run.Args = append([]string{"inspect"}, p.Run.Args...)
		if len(breaks.at(inside(dir, filepath.Ext(file)))) > 0 {
			p.Note = "node inspect takes no breakpoints from the editor: it stops at the first line, where setBreakpoint(line) or sb(line) sets one."
		}
		return p, nil
	case ".go":
		if _, err := look("dlv"); err != nil {
			return Plan{}, &MissingError{Language: "Go debugging", Tools: []string{"dlv"}, Hint: "go install github.com/go-delve/delve/cmd/dlv@latest"}
		}
		p, err := For(file, out, look)
		if err != nil {
			return Plan{}, err
		}
		target, scope := file, func(f string) bool { return samePath(f, file) }
		if len(p.Run.Args) == 2 && p.Run.Args[1] == "." {
			target, scope = ".", inside(dir, ".go") // a package in a module: debug it whole
		}
		args := []string{"debug"}
		if bs := breaks.at(scope); len(bs) > 0 {
			init := ""
			for _, b := range bs {
				init += fmt.Sprintf("break %s:%d\n", filepath.ToSlash(realPath(b.file)), b.line)
			}
			script := filepath.Join(out, strings.TrimSuffix(filepath.Base(file), ".go")+".dlv")
			p.Files = map[string]string{script: init + "continue\n"}
			args = append(args, "--init", script)
		}
		p.Run = Step{Tool: "dlv", Args: append(args, target), Dir: dir}
		return p, nil
	}
	return Plan{}, ErrNoDebugger
}
