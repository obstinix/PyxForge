package vt

import (
	"net/url"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// Shell integration (D24): a shell that loads PyxForge's script marks where its prompt, the
// command typed at it, the command's output and its end are, with OSC 133, and says its folder
// with OSC 7. The screen records each mark at its absolute position (a history line number
// that scrolling does not change), so marks survive scrolling and are dropped with the lines
// they are on when the history is trimmed or cleared.

// MarkKind is which OSC 133 mark.
type MarkKind byte

const (
	MarkPrompt  MarkKind = 'A' // the prompt starts
	MarkCommand MarkKind = 'B' // the prompt ends: what is typed next is the command
	MarkOutput  MarkKind = 'C' // the command runs: its output starts
	MarkEnd     MarkKind = 'D' // the command ended, with its exit status
)

// Mark is one shell-integration mark.
type Mark struct {
	Kind MarkKind
	At   Pos       // where the cursor was: an absolute line and a column
	Exit int       // for MarkEnd: the exit status, -1 when the shell gave none
	Cwd  string    // the folder the shell last reported (OSC 7) when the mark was made
	Seq  uint64    // the order of marks, from 1
	Time time.Time // when the mark arrived, for a command's duration
}

// maxMarks bounds the marks kept, whatever the history: 4 per command.
const maxMarks = 4 * 4096

// Marks are the shell-integration marks on lines still kept, oldest first.
func (s *Screen) Marks() []Mark {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Mark(nil), s.marks...)
}

// Cwd is the folder the shell last reported with OSC 7, or "".
func (s *Screen) Cwd() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cwd
}

// MarkSeq is the sequence number of the last mark recorded (0: none yet): it changes whenever
// a mark arrives.
func (s *Screen) MarkSeq() uint64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.markSeq
}

// FirstLine is the absolute number of the oldest line kept.
func (s *Screen) FirstLine() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.dropped
}

// EndLine is the absolute number just past the last screen line.
func (s *Screen) EndLine() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.dropped + len(s.history) + s.rows
}

// LineText is absolute line ln's text, trailing blanks dropped, and whether it continues on
// the next (the terminal wrapped it); "" for a line no longer kept.
func (s *Screen) LineText(ln int) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	l, ok := s.line(ln)
	if !ok {
		return "", false
	}
	return cellText(l.Cells, true), l.Wrapped
}

// screenTop is the absolute line of the screen's first row.
func (s *Screen) screenTop() int { return s.dropped + len(s.history) }

// osc133 records a mark. Marks on the alternate screen (a full-screen program) are ignored.
func (s *Screen) osc133(arg string) {
	if s.altOn || arg == "" {
		return
	}
	kind := MarkKind(arg[0])
	switch kind {
	case MarkPrompt, MarkCommand, MarkOutput, MarkEnd:
	default:
		return
	}
	m := Mark{Kind: kind, At: Pos{Line: s.screenTop() + s.cur.y, Col: s.cur.x}, Exit: -1, Cwd: s.cwd, Time: now()}
	if kind == MarkEnd {
		if _, rest, ok := strings.Cut(arg, ";"); ok {
			code, _, _ := strings.Cut(rest, ";")
			if n, err := strconv.Atoi(code); err == nil {
				m.Exit = n
			}
		}
	}
	s.markSeq++
	m.Seq = s.markSeq
	s.marks = append(s.marks, m)
	if len(s.marks) > maxMarks {
		s.marks = append(s.marks[:0:0], s.marks[len(s.marks)-maxMarks:]...)
	}
}

// osc7 records the shell's folder: file://host/path, the path percent-encoded.
func (s *Screen) osc7(arg string) {
	u, err := url.Parse(arg)
	if err != nil || u.Scheme != "file" {
		return
	}
	p := u.Path
	if runtime.GOOS == "windows" {
		// file://host/C:/Users/me → C:\Users\me
		p = strings.TrimPrefix(p, "/")
		p = strings.ReplaceAll(p, "/", `\`)
	}
	if p != "" {
		s.cwd = p
	}
}

// dropMarksBefore forgets marks on lines before ln (trimmed or cleared history).
func (s *Screen) dropMarksBefore(ln int) {
	i := 0
	for i < len(s.marks) && s.marks[i].At.Line < ln {
		i++
	}
	if i > 0 {
		s.marks = append(s.marks[:0:0], s.marks[i:]...)
	}
}

// dropMarksFrom forgets marks on line ln and after (the screen was erased).
func (s *Screen) dropMarksFrom(ln int) {
	i := len(s.marks)
	for i > 0 && s.marks[i-1].At.Line >= ln {
		i--
	}
	s.marks = s.marks[:i]
}

// now is the clock marks are stamped with; tests replace it.
var now = time.Now

// LineAt is absolute line ln, fitted to the screen's width, and whether it is still kept.
func (s *Screen) LineAt(ln int) (Line, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	l, ok := s.line(ln)
	if !ok {
		return Line{}, false
	}
	l.Cells = append([]Cell(nil), l.Cells...) // the caller reads it after the lock is gone
	return fitLine(l, s.cols), true
}

// AltScreen reports whether a full-screen program's alternate screen shows.
func (s *Screen) AltScreen() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.altOn
}
