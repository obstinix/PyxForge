package neovim

import (
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestTerminalLifecycleStress runs the terminal sequences that crashed the terminal's Neovim on
// Windows (0xC0000005, seen on CI at 6efbb66 and earlier): an interactive program that ends
// while its terminal is in terminal mode, the next program started at once while the first
// one's transcript is being kept, programs that end before or as their terminal takes keys,
// the UI resized while they start, and quick switches between sessions and a running shell.
// Neovim must live through all of it, and every transcript must hold its program's output.
// PYXFORGE_TERMINAL_STRESS=<n> runs n rounds instead of the default.
func TestTerminalLifecycleStress(t *testing.T) {
	rounds := 8
	if n, err := strconv.Atoi(os.Getenv("PYXFORGE_TERMINAL_STRESS")); err == nil && n > 0 {
		rounds = n
	}
	events := make(chan Event, 4096)
	s := startReal(t, events)
	dir := t.TempDir()
	interactive := []string{"sh", "-c", `read answer; echo "got $answer"`}
	fast := func(i int) []string { return []string{"sh", "-c", fmt.Sprintf("echo fast-%d", i)} }
	if runtime.GOOS == "windows" {
		interactive = []string{"cmd.exe", "/c", "pause"}
		fast = func(i int) []string { return []string{"cmd.exe", "/c", fmt.Sprintf("echo fast-%d", i)} }
	}

	alive := func(when string) {
		t.Helper()
		if s.Exited() {
			t.Fatalf("Neovim ended %s: %v", when, s.exitStatus())
		}
	}
	var mu sync.Mutex
	closed := map[int]bool{}
	// closedSoon waits for buf's TermClose, handing every TermClose it reads to keep, as the
	// terminal panel does: transcripts are kept in the background, while the test goes on.
	var wg sync.WaitGroup
	transcripts := map[int]int{} // program buffer -> its transcript
	keep := func(buf int) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			out, err := s.KeepTranscript(buf)
			if err != nil {
				t.Errorf("KeepTranscript(%d): %v", buf, err)
				return
			}
			mu.Lock()
			transcripts[buf] = out
			mu.Unlock()
		}()
	}
	closedSoon := func(buf int, when string) {
		t.Helper()
		for end := time.Now().Add(20 * time.Second); ; {
			mu.Lock()
			done := closed[buf]
			mu.Unlock()
			if done {
				return
			}
			select {
			case e := <-events:
				if e.Kind == "TermClose" {
					mu.Lock()
					closed[e.Buffer] = true
					mu.Unlock()
					keep(e.Buffer)
				}
			case <-time.After(20 * time.Millisecond):
			}
			alive(when)
			if time.Now().After(end) {
				t.Fatalf("no TermClose for buffer %d %s", buf, when)
			}
		}
	}
	mode := func() string {
		var m string
		_ = s.ExecLua(`return vim.api.nvim_get_mode().mode`, &m)
		return m
	}

	shell, err := s.NewTerminal()
	if err != nil {
		t.Fatal(err)
	}
	fastBufs := map[int]int{} // round -> buffer
	for i := 0; i < rounds; i++ {
		when := fmt.Sprintf("in round %d", i)
		// An interactive program, typed into in terminal mode, ends there.
		ask, err := s.RunInTerminal(interactive, dir)
		if err != nil {
			t.Fatalf("starting the interactive program %s: %v", when, err)
		}
		for end := time.Now().Add(10 * time.Second); mode() != "t"; time.Sleep(10 * time.Millisecond) {
			alive(when)
			if time.Now().After(end) {
				t.Fatalf("the interactive program's terminal never took keys %s", when)
			}
		}
		s.Input("x<CR>")
		closedSoon(ask, when)

		// The next program starts at once, its terminal resized as it does, then the user flicks
		// between it, the shell and back.
		time.Sleep(time.Duration(i%4) * 25 * time.Millisecond)
		f, err := s.RunInTerminal(fast(i), dir)
		if err != nil {
			t.Fatalf("starting the fast program %s: %v", when, err)
		}
		fastBufs[i] = f
		for k := 0; k < 3; k++ {
			_ = s.Resize(80+(i+k)%5, 10+k%3)
		}
		for k := 0; k < 3; k++ {
			if err := s.ShowTerminal(shell); err != nil {
				t.Fatalf("showing the shell %s: %v", when, err)
			}
			if err := s.ShowTerminal(f); err != nil {
				t.Fatalf("showing the fast program %s: %v", when, err)
			}
		}
		closedSoon(f, when)
		alive(when)
	}
	wg.Wait()
	alive("after the last transcript")

	// Every fast program's transcript holds what it printed, and the shell still runs.
	for i, f := range fastBufs {
		out := transcripts[f]
		var text string
		if err := s.ExecLua(`local buf = ...
if not vim.api.nvim_buf_is_valid(buf) then return "" end
return table.concat(vim.api.nvim_buf_get_lines(buf, 0, -1, false), "\n")`, &text, out); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(text, fmt.Sprintf("fast-%d", i)) {
			t.Errorf("round %d's transcript (buffer %d) lacks its output:\n%s", i, out, text)
		}
	}
	var running bool
	if err := s.ExecLua(`local job = vim.b[...].terminal_job_id
return job ~= nil and vim.fn.jobwait({ job }, 0)[1] == -1`, &running, shell); err != nil || !running {
		t.Errorf("the shell stopped running (%v)", err)
	}
}

// TestTerminalModeOnlyWhileStarted: a program waiting for input has its terminal take keys
// when RunInTerminal returns; one that ends at once never leaves its terminal in terminal mode
// once its transcript is kept, where a key would close it and where Neovim crashed on Windows.
func TestTerminalModeOnlyWhileStarted(t *testing.T) {
	events := make(chan Event, 64)
	s := startReal(t, events)
	quick := []string{"sh", "-c", "exit 0"}
	wait := []string{"sh", "-c", "read x"}
	if runtime.GOOS == "windows" {
		quick = []string{"cmd.exe", "/c", "exit 0"}
		wait = []string{"cmd.exe", "/c", "pause"}
	}
	mode := func() string {
		var m string
		if err := s.ExecLua(`return vim.api.nvim_get_mode().mode`, &m); err != nil {
			t.Fatal(err)
		}
		return m
	}
	buf, err := s.RunInTerminal(wait, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if m := mode(); m != "t" {
		t.Errorf("a program waiting for input: mode %q when RunInTerminal returned, want terminal mode", m)
	}
	s.Input("x<CR>")
	waitEvent(t, events, "TermClose", func(e Event) bool { return e.Buffer == buf })
	if _, err := s.KeepTranscript(buf); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		b, err := s.RunInTerminal(quick, t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		waitEvent(t, events, "TermClose", func(e Event) bool { return e.Buffer == b })
		if _, err := s.KeepTranscript(b); err != nil {
			t.Fatal(err)
		}
		if m := mode(); m == "t" {
			t.Errorf("run %d: still in terminal mode on an ended program's transcript", i)
		}
	}
	if s.Exited() {
		t.Fatalf("Neovim ended: %v", s.exitStatus())
	}
}
