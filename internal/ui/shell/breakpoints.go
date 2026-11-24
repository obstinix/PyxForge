package shell

import (
	"path/filepath"

	"github.com/obstinix/PyxForge/internal/ui/notifications"
	"github.com/obstinix/PyxForge/internal/workspace"
)

// Breakpoints set in the editor (PXF-FEAT-004) are kept by the native host, moving with their
// lines as the text changes; F9 there, a click on a line number, or Toggle Breakpoint here,
// sets or clears one. The shell keeps the list, saves it with the workspace and gives it back
// when the workspace opens again. Debug Current File and Debug in QEMU read the list as it is
// when they start, and a running QEMU session follows each change.

// toggleBreakpoint sets or clears the breakpoint on the cursor's line of the file shown.
func (s *Shell) toggleBreakpoint() {
	d := s.activeDoc()
	switch {
	case d == nil:
		s.Notify(notifications.Info, "No file in the editor", "A breakpoint is a line of a file open in the editor.")
	case d.path == "":
		s.Notify(notifications.Info, "Not a file", "Save the file first: a breakpoint is a line of a file.")
	default:
		s.nat.toggleBreakpointAt(d, d.ed.Cur.Line+1)
	}
}

// setBreakpoints records the list, and passes the change to a running debugger.
func (s *Shell) setBreakpoints(list []Breakpoint) {
	s.breakpoints = list
	s.mach.syncBreakpoints(list)
}

// currentBreakpoints is every breakpoint, at the lines where they are now.
func (s *Shell) currentBreakpoints() []Breakpoint { return s.breakpoints }

func keptBreakpoints(list []Breakpoint) []workspace.Breakpoint {
	var out []workspace.Breakpoint
	for _, b := range list {
		out = append(out, workspace.Breakpoint{File: b.File, Lines: b.Lines})
	}
	return out
}

func restoredBreakpoints(list []workspace.Breakpoint) []Breakpoint {
	var out []Breakpoint
	for _, b := range list {
		out = append(out, Breakpoint{File: b.File, Lines: b.Lines})
	}
	return out
}

// resolved is p with links resolved and short (8.3) names spelled out, so two spellings of one
// path compare equal.
func resolved(p string) string {
	p = filepath.Clean(p)
	if r, err := filepath.EvalSymlinks(p); err == nil {
		return r
	}
	if dir := filepath.Dir(p); dir != p { // a file that is not there (yet): its folder, resolved
		return filepath.Join(resolved(dir), filepath.Base(p))
	}
	return p
}
