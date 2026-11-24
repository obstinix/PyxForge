package shell

import (
	"os"
)

// showStopLine opens the file a debugger stopped in and marks the line (#58); without a file
// and line (code with no debug information, such as a boot sector) it clears the mark. The
// mark is set after the file has opened: both go through the editor's ordered queue.
func (s *Shell) showStopLine(file string, line int) {
	if file == "" || line < 1 {
		s.clearStopLine()
		return
	}
	if _, err := os.Stat(file); err != nil {
		s.clearStopLine() // the debug information names a source this machine does not have
		return
	}
	if s.startNative() {
		s.nat.showStop(file, line)
	}
}

// clearStopLine removes the stopped-line mark, when the program runs on or the session ends.
func (s *Shell) clearStopLine() {
	if s.nat != nil {
		s.nat.clearStop()
	}
}
