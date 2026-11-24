// Package term is a terminal session: a program on a pseudo-terminal (internal/pty) whose
// output an emulated screen (internal/vt) reads. The Terminal panel draws Session.Screen and
// writes keys to the session (D17). Nothing here draws or depends on Fyne.
package term

import (
	"strings"
	"sync"
	"time"

	"github.com/obstinix/PyxForge/internal/pty"
	"github.com/obstinix/PyxForge/internal/vt"
)

// Session is one program in a terminal.
type Session struct {
	Screen *vt.Screen
	Argv   []string

	p       *pty.PTY
	replies chan []byte
	ended   chan struct{} // closed once the program has exited and its output is all read
	status  int
	mu      sync.Mutex
	size    [2]int
}

// Hooks are called from the session's own goroutines.
type Hooks struct {
	// Changed is called after output changed the screen. It is called often; coalesce.
	Changed func()
	// Exited is called once, after the program exited and its last output is on the screen.
	Exited func(status int)
}

// Start runs o.Argv on a terminal of o.Cols by o.Rows.
func Start(o pty.Options, h Hooks) (*Session, error) {
	cols, rows := max(o.Cols, 2), max(o.Rows, 1)
	o.Cols, o.Rows = cols, rows
	p, err := pty.Start(o)
	if err != nil {
		return nil, err
	}
	s := &Session{
		Screen:  vt.New(cols, rows),
		Argv:    append([]string(nil), o.Argv...),
		p:       p,
		replies: make(chan []byte, 64),
		ended:   make(chan struct{}),
		size:    [2]int{cols, rows},
	}
	// Answers to the program's queries go back from a goroutine of their own, so a full
	// input pipe never holds up reading output.
	s.Screen.SetReply(func(b []byte) {
		select {
		case s.replies <- b:
		default:
		}
	})
	go func() {
		for {
			select {
			case b := <-s.replies:
				_, _ = p.Write(b)
			case <-s.ended:
				return
			}
		}
	}()
	go s.read(h)
	return s, nil
}

func (s *Session) read(h Hooks) {
	buf := make([]byte, 32*1024)
	for {
		n, err := s.p.Read(buf)
		if n > 0 {
			s.Screen.Write(buf[:n])
			if h.Changed != nil {
				h.Changed()
			}
		}
		if err != nil {
			break
		}
	}
	s.status = s.p.Wait()
	close(s.ended)
	if h.Exited != nil {
		h.Exited(s.status)
	}
}

// Write types b into the program.
func (s *Session) Write(b []byte) error {
	_, err := s.p.Write(b)
	return err
}

// Resize sets the terminal's size, the screen's first so the program's redraw fits it.
func (s *Session) Resize(cols, rows int) {
	cols, rows = max(cols, 2), max(rows, 1)
	s.mu.Lock()
	same := s.size == [2]int{cols, rows}
	s.size = [2]int{cols, rows}
	s.mu.Unlock()
	if same {
		return
	}
	s.Screen.Resize(cols, rows)
	_ = s.p.Resize(cols, rows)
}

// Close ends the program; Ended tells when it has.
func (s *Session) Close() { _ = s.p.Close() }

// Ended is closed once the program has exited and its output is read.
func (s *Session) Ended() <-chan struct{} { return s.ended }

// Running reports whether the program has not ended.
func (s *Session) Running() bool {
	select {
	case <-s.ended:
		return false
	default:
		return true
	}
}

// Status is the program's exit status, once Ended.
func (s *Session) Status() int {
	<-s.ended
	return s.status
}

// Pid is the program's process id.
func (s *Session) Pid() int { return s.p.Pid() }

// Clear drops the scrollback and, when the program is a shell still running, has it clear the
// screen: cmd.exe is typed cls (it shows Ctrl+L as ^L), other shells get Ctrl+L. Nothing is
// typed into another program, whose input that would be.
func (s *Session) Clear() {
	s.Screen.ClearHistory()
	if !s.Running() {
		return
	}
	switch pty.IsShell(s.Argv) {
	case "cmd":
		_ = s.Write([]byte("cls\r"))
	case "ctrl-l":
		_ = s.Write([]byte{0x0c})
	}
}

// TypeAtPrompt types lines into the program: the first once its output shows prompt, each
// next one once the prompt shows again. With after, only prompts on lines after the first one
// showing after count: a program can prompt before it is ready (node inspect does, before it
// pauses at the program's start). It gives up after timeout for each line, and when the
// program ends. It returns how many lines were typed.
func (s *Session) TypeAtPrompt(after, prompt string, lines []string, timeout time.Duration) int {
	count := func() int {
		all := s.Screen.Lines()
		start := 0
		if after != "" {
			start = -1
			for i, l := range all {
				if strings.Contains(l, after) {
					start = i + 1
					break
				}
			}
			if start < 0 {
				return 0
			}
		}
		n := 0
		for _, l := range all[start:] {
			if strings.Contains(l, prompt) {
				n++
			}
		}
		return n
	}
	seen := 0
	for i, l := range lines {
		end := time.Now().Add(timeout)
		for n := count(); n <= seen; n = count() {
			if !s.Running() || time.Now().After(end) {
				return i
			}
			time.Sleep(50 * time.Millisecond)
		}
		seen = count()
		if err := s.Write([]byte(l + "\r")); err != nil {
			return i
		}
	}
	return len(lines)
}
