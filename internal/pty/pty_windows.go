//go:build windows

package pty

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"time"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows"
)

// PTY is a program running on a pseudo console (ConPTY). Its handles live in PyxForge's job
// object (proc.ContainChildren), so the program ends with PyxForge (D10).
type PTY struct {
	base
	mu      sync.Mutex // guards console against Resize after close: the crash of #80
	console windows.Handle
	in      *os.File // the console's input: what is written here is typed
	out     *os.File // the console's output
	proc    windows.Handle
	pid     int
}

// Start runs o.Argv on a new pseudo console of o.Cols by o.Rows.
func Start(o Options) (*PTY, error) {
	if len(o.Argv) == 0 {
		return nil, errors.New("no program to run")
	}
	path, err := exec.LookPath(o.Argv[0])
	if err != nil {
		return nil, err
	}
	cols, rows := size(o)
	var inR, inW, outR, outW windows.Handle
	if err := windows.CreatePipe(&inR, &inW, nil, 0); err != nil {
		return nil, fmt.Errorf("console input pipe: %w", err)
	}
	if err := windows.CreatePipe(&outR, &outW, nil, 0); err != nil {
		closeAll(inR, inW)
		return nil, fmt.Errorf("console output pipe: %w", err)
	}
	var con windows.Handle
	if err := windows.CreatePseudoConsole(windows.Coord{X: int16(cols), Y: int16(rows)}, inR, outW, 0, &con); err != nil {
		closeAll(inR, inW, outR, outW)
		return nil, fmt.Errorf("pseudo console: %w", err)
	}
	// The console holds its own copies of its ends of the pipes.
	closeAll(inR, outW)
	fail := func(err error) (*PTY, error) {
		windows.ClosePseudoConsole(con)
		closeAll(inW, outR)
		return nil, err
	}

	attrs, err := windows.NewProcThreadAttributeList(1)
	if err != nil {
		return fail(err)
	}
	defer attrs.Delete()
	// The attribute's value is the console handle itself, not a pointer to it.
	if err := attrs.Update(windows.PROC_THREAD_ATTRIBUTE_PSEUDOCONSOLE, *(*unsafe.Pointer)(unsafe.Pointer(&con)), unsafe.Sizeof(con)); err != nil {
		return fail(err)
	}
	si := windows.StartupInfoEx{ProcThreadAttributeList: attrs.List()}
	si.Cb = uint32(unsafe.Sizeof(si))
	// No standard handles of PyxForge's: the program's are the console's (as Windows Terminal
	// does; a PyxForge started from a console would otherwise pass its own down).
	si.Flags = windows.STARTF_USESTDHANDLES

	cmdline, err := windows.UTF16PtrFromString(windows.ComposeCommandLine(append([]string{path}, o.Argv[1:]...)))
	if err != nil {
		return fail(err)
	}
	var dir *uint16
	if o.Dir != "" {
		if dir, err = windows.UTF16PtrFromString(o.Dir); err != nil {
			return fail(err)
		}
	}
	env := o.Env
	if env == nil {
		env = os.Environ()
	}
	block := envBlock(env)
	var pi windows.ProcessInformation
	flags := uint32(windows.EXTENDED_STARTUPINFO_PRESENT | windows.CREATE_UNICODE_ENVIRONMENT)
	if err := windows.CreateProcess(nil, cmdline, nil, nil, false, flags, &block[0], dir, &si.StartupInfo, &pi); err != nil {
		return fail(fmt.Errorf("start %s: %w", o.Argv[0], err))
	}
	windows.CloseHandle(pi.Thread)
	p := &PTY{
		base:    newBase(),
		console: con,
		in:      os.NewFile(uintptr(inW), "conpty-in"),
		out:     os.NewFile(uintptr(outR), "conpty-out"),
		proc:    pi.Process,
		pid:     int(pi.ProcessId),
	}
	go p.wait()
	return p, nil
}

func (p *PTY) wait() {
	_, _ = windows.WaitForSingleObject(p.proc, windows.INFINITE)
	var code uint32
	status := -1
	if err := windows.GetExitCodeProcess(p.proc, &code); err == nil {
		status = int(int32(code))
	}
	p.exit(status, p.closeConsole)
}

// closeConsole ends the pseudo console: what still runs on it gets CTRL_CLOSE_EVENT, and its
// output pipe ends once the console has written the rest, so the reader sees io.EOF.
func (p *PTY) closeConsole() {
	p.mu.Lock()
	con := p.console
	p.console = 0
	p.mu.Unlock()
	if con != 0 {
		// ClosePseudoConsole waits for the console to finish writing, which needs the reader
		// to keep reading: run it aside.
		go windows.ClosePseudoConsole(con)
	}
	_ = p.in.Close()
}

// Pid is the program's process id.
func (p *PTY) Pid() int { return p.pid }

// Read reads the program's output; io.EOF once the program has exited and its output is all
// read. Keep reading until then: the console waits for its output to be read.
func (p *PTY) Read(b []byte) (int, error) {
	n, err := p.read(p.out.Read, b)
	if err != nil {
		_ = p.out.Close()
	}
	return n, err
}

// Write types b into the program.
func (p *PTY) Write(b []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.console == 0 {
		return 0, ErrClosed
	}
	return p.in.Write(b)
}

// Resize gives the terminal a new size; after Close it does nothing and returns ErrClosed.
func (p *PTY) Resize(cols, rows int) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.console == 0 {
		return ErrClosed
	}
	return windows.ResizePseudoConsole(p.console, windows.Coord{X: int16(max(cols, 2)), Y: int16(max(rows, 1))})
}

// Close ends the program: its console closes, and if it still runs closeGrace later it is
// killed. It does not wait; Done tells when the program has gone.
func (p *PTY) Close() error {
	if !p.closing.CompareAndSwap(false, true) {
		return nil
	}
	p.once.Do(p.closeConsole)
	go func() {
		select {
		case <-p.exited:
		case <-time.After(closeGrace):
			_ = windows.TerminateProcess(p.proc, 1)
		}
		<-p.exited
		windows.CloseHandle(p.proc)
	}()
	return nil
}

// envBlock is env as a Windows environment block: each "name=value" ending in a NUL, and a NUL
// after the last.
func envBlock(env []string) []uint16 {
	var b []uint16
	for _, kv := range env {
		b = append(b, utf16.Encode([]rune(kv))...)
		b = append(b, 0)
	}
	if len(b) == 0 {
		b = append(b, 0)
	}
	return append(b, 0)
}

func closeAll(hs ...windows.Handle) {
	for _, h := range hs {
		windows.CloseHandle(h)
	}
}
