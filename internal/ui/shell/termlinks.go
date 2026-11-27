package shell

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/obstinix/PyxForge/internal/build"
	"github.com/obstinix/PyxForge/internal/term"
	"github.com/obstinix/PyxForge/internal/ui/codeedit"
	"github.com/obstinix/PyxForge/internal/ui/notifications"
)

// Output that links into the IDE (identity prompt Phase 4c).

// resolveTerminalLink turns a path a command printed into a file in the project: relative
// paths against the folder the command ran in (cwd, from the shell's OSC 7; the project root
// when the shell never said), and only a file that exists inside the project (relWithin, the
// containment rule Show Changes and Run Current File keep).
func (s *Shell) resolveTerminalLink(cwd, p string) (string, bool) {
	if s.root == "" || p == "" {
		return "", false
	}
	if cwd == "" {
		cwd = s.root
	}
	if !filepath.IsAbs(p) {
		p = filepath.Join(cwd, filepath.FromSlash(p))
	}
	p = filepath.Clean(p)
	if _, ok := relWithin(s.root, p); !ok {
		return "", false
	}
	if fi, err := os.Stat(p); err != nil || fi.IsDir() {
		return "", false
	}
	return p, true
}

// openAt opens a file in the editor at a 0-based line and column.
func (s *Shell) openAt(path string, line, col int) {
	if !s.startNative() {
		s.OpenFile(path)
		return
	}
	if d := s.nat.open(path, true); d != nil {
		d.ed.Reveal(codeedit.Pos{Line: line, Col: col})
	}
}

// sendBlockToProblems reads a command block's output with the build's diagnostics parsers
// (GNU, MSVC and rustc text, Cargo's JSON, GCC's JSON, SARIF) and puts what it finds in
// Problems with the source "Terminal: <command>". Sending a command again replaces what it
// sent before; paths outside the project are kept, without a link, as the build does.
func (s *Shell) sendBlockToProblems(b term.Block, output string) {
	ds := build.ParseDiagnostics(output)
	source := "Terminal: " + b.Command
	if s.termProblems == nil {
		s.termProblems = map[string][]problem{}
	}
	var ps []problem
	for _, d := range ds {
		path := d.File
		if path != "" && !filepath.IsAbs(path) {
			dir := b.Cwd
			if dir == "" {
				dir = s.root
			}
			path = filepath.Join(dir, filepath.FromSlash(path))
		}
		ps = append(ps, problem{path: path, noLine: d.Line == 0, tool: d.Tool,
			d: Diagnostic{Line: max(d.Line-1, 0), Col: max(d.Column-1, 0), Severity: severityOf(d.Severity), Message: d.Message, Source: source}})
	}
	s.termProblems[b.Command] = ps
	s.refreshProblems()
	if len(ps) == 0 {
		s.Notify(notifications.Info, "No problems found", "Nothing in the output of "+quoteCommand(b.Command)+" reads as a compiler or linker message.")
		return
	}
	s.showDockItem(s.problemsTab)
	s.logf("Terminal: %d problems from %s", len(ps), b.Command)
}

func quoteCommand(c string) string {
	if r := []rune(c); len(r) > 40 {
		c = string(r[:39]) + "…"
	}
	return fmt.Sprintf("“%s”", strings.TrimSpace(c))
}
