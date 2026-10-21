package shell

import (
	"errors"
	"path/filepath"

	"github.com/obstinix/PyxForge/internal/neovim"
	"github.com/obstinix/PyxForge/internal/ui/notifications"
	"github.com/obstinix/PyxForge/internal/workspace"
)

// Breakpoints set in the editor (PXF-FEAT-004) live in the editor's Neovim, as signs that move
// with their lines (nvim/lua/pyxforge/breakpoints.lua); F9 there, or Toggle Breakpoint here,
// sets or clears one. The shell keeps the last list Neovim reported, saves it with the
// workspace and gives it back when the editor starts again. Debug Current File and Debug in
// QEMU ask Neovim for the list as it is when they start, and a running QEMU session follows
// each change.

// toggleBreakpoint sets or clears the breakpoint on the cursor's line of the file shown.
func (s *Shell) toggleBreakpoint() {
	if s.activeBuffer() == nil {
		s.Notify(notifications.Info, "No file in the editor", "A breakpoint is a line of a file open in the editor.")
		return
	}
	sess := s.ed.sess
	go func() {
		_, err := sess.ToggleBreakpoint()
		s.dispatch(func() {
			switch {
			case errors.Is(err, neovim.ErrNotAFile):
				s.Notify(notifications.Info, "Not a file", "Save the file first: a breakpoint is a line of a file.")
			case err != nil:
				s.Notify(notifications.Error, "Breakpoint not set", err.Error())
			}
		})
	}()
}

// setBreakpoints records the list Neovim reported, and passes the change to a running debugger.
func (s *Shell) setBreakpoints(list []neovim.Breakpoint) {
	s.breakpoints = list
	s.mach.syncBreakpoints(list)
}

// currentBreakpoints asks the editor for its breakpoints where their lines are now; without an
// editor, or when it cannot answer, the last list kept.
func (s *Shell) currentBreakpoints() []neovim.Breakpoint {
	if s.ed != nil && s.ed.sess != nil {
		if list, err := s.ed.sess.Breakpoints(); err == nil {
			s.breakpoints = list
		}
	}
	return s.breakpoints
}

func keptBreakpoints(list []neovim.Breakpoint) []workspace.Breakpoint {
	var out []workspace.Breakpoint
	for _, b := range list {
		out = append(out, workspace.Breakpoint{File: b.File, Lines: b.Lines})
	}
	return out
}

func restoredBreakpoints(list []workspace.Breakpoint) []neovim.Breakpoint {
	var out []neovim.Breakpoint
	for _, b := range list {
		out = append(out, neovim.Breakpoint{File: b.File, Lines: b.Lines})
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
