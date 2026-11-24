package term

import (
	"os"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/obstinix/PyxForge/internal/pty"
	"github.com/obstinix/PyxForge/internal/vt"
)

const esc = "\x1b"

// script is a command run by the platform's shell: cmd.exe on Windows, sh elsewhere.
func script(win, unix string) []string {
	if runtime.GOOS == "windows" {
		return []string{"cmd.exe", "/d", "/c", win}
	}
	return []string{"/bin/sh", "-c", unix}
}

func start(t *testing.T, argv []string, env []string) (*Session, chan int) {
	t.Helper()
	exited := make(chan int, 1)
	s, err := Start(pty.Options{Argv: argv, Dir: t.TempDir(), Env: env, Cols: 60, Rows: 12},
		Hooks{Exited: func(st int) { exited <- st }})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		s.Close()
		select {
		case <-s.Ended():
		case <-time.After(10 * time.Second):
			t.Error("the program outlived its test")
		}
	})
	return s, exited
}

func waitScreen(t *testing.T, s *Session, want string) {
	t.Helper()
	end := time.Now().Add(30 * time.Second)
	for !strings.Contains(s.Screen.String(), want) {
		if time.Now().After(end) {
			t.Fatalf("the screen never showed %q:\n%s", want, s.Screen.String())
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// waitQuiet waits until the screen has not changed for 300 ms (at most 10 s).
func waitQuiet(s *Session) {
	end := time.Now().Add(10 * time.Second)
	last, since := s.Screen.Generation(), time.Now()
	for time.Now().Before(end) && time.Since(since) < 300*time.Millisecond {
		time.Sleep(20 * time.Millisecond)
		if g := s.Screen.Generation(); g != last {
			last, since = g, time.Now()
		}
	}
}

func waitExit(t *testing.T, exited chan int) int {
	t.Helper()
	select {
	case st := <-exited:
		return st
	case <-time.After(30 * time.Second):
		t.Fatal("no exit")
	}
	return 0
}

// TestOutputColourAndPosition: what a program prints reaches the screen through the
// pseudo-terminal (ConPTY re-renders it), colours and cursor positioning included, and its
// exit is reported after its last output.
func TestOutputColourAndPosition(t *testing.T) {
	s, exited := start(t, script("echo "+esc+"[31mRED"+esc+"[0m plain&echo "+esc+"[8;20HPLACED& exit 3",
		`printf '\033[31mRED\033[0m plain\n\033[8;20HPLACED'; exit 3`), nil)
	if st := waitExit(t, exited); st != 3 {
		t.Errorf("exit status %d", st)
	}
	text := s.Screen.String()
	if !strings.Contains(text, "RED plain") || !strings.Contains(text, "PLACED") {
		t.Fatalf("screen:\n%s", text)
	}
	v := s.Screen.View(0)
	found := false
	for y, l := range v.Lines {
		for x, c := range l.Cells {
			if c.R == 'R' && x+2 < len(l.Cells) && l.Cells[x+1].R == 'E' {
				found = true
				if c.FG != vt.Indexed(1) && c.FG != vt.Indexed(9) {
					t.Errorf("RED's colour %x", c.FG)
				}
			}
			if c.R == 'P' && x+1 < len(l.Cells) && l.Cells[x+1].R == 'L' && (y != 7 || x != 19) {
				t.Errorf("PLACED at row %d column %d, want 8, 20", y+1, x+1)
			}
		}
	}
	if !found {
		t.Error("RED not found on the screen")
	}
	if s.Running() {
		t.Error("Running after the exit")
	}
}

// TestShellSession types into a shell at its prompt, resizes it, clears it, and ends it.
func TestShellSession(t *testing.T) {
	argv, env := []string{"cmd.exe", "/d", "/k", "prompt PYX$G"}, []string(nil)
	if runtime.GOOS != "windows" {
		argv, env = []string{"/bin/sh", "-i"}, append(os.Environ(), "PS1=PYX> ", "ENV=")
	}
	s, exited := start(t, argv, env)
	waitScreen(t, s, "PYX>")
	if n := s.TypeAtPrompt("", "PYX>", []string{"echo first-line", "echo second-line"}, 20*time.Second); n != 2 {
		t.Fatalf("typed %d of 2:\n%s", n, s.Screen.String())
	}
	waitScreen(t, s, "second-line")
	s.Resize(40, 8)
	if c, r := s.Screen.Size(); c != 40 || r != 8 {
		t.Errorf("screen size %dx%d", c, r)
	}
	for i := range 20 { // enough to scroll into the history
		_ = s.Write([]byte("echo filler-" + string(rune('a'+i)) + "\r"))
	}
	waitScreen(t, s, "filler-t")
	if s.Screen.View(0).History == 0 {
		t.Errorf("no history after scrolling:\n%s", s.Screen.String())
	}
	waitQuiet(s) // the prompt after the last echo can still be on its way, and would scroll
	s.Clear()
	if h := s.Screen.View(0).History; h != 0 {
		t.Errorf("history %d after Clear", h)
	}
	_ = s.Write([]byte("exit\r"))
	if st := waitExit(t, exited); st != 0 {
		t.Errorf("the shell's exit status %d", st)
	}
}
