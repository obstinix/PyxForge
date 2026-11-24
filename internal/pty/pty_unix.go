//go:build !windows

package pty

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	cpty "github.com/creack/pty"

	"github.com/obstinix/PyxForge/internal/proc"
)

// PTY is a program running on a Unix pseudo-terminal, in a session of its own, ended with
// PyxForge (proc.Bind, D10).
type PTY struct {
	base
	mu     sync.Mutex
	master *os.File // nil once closed
	cmd    *exec.Cmd
}

// Start runs o.Argv on a new pseudo-terminal of o.Cols by o.Rows.
func Start(o Options) (*PTY, error) {
	if len(o.Argv) == 0 {
		return nil, errors.New("no program to run")
	}
	cmd := exec.Command(o.Argv[0], o.Argv[1:]...)
	cmd.Dir = o.Dir
	env := o.Env
	if env == nil {
		env = os.Environ()
	}
	cmd.Env = append(env[:len(env):len(env)], "TERM=xterm-256color", "COLORTERM=truecolor")
	proc.Bind(cmd)
	cols, rows := size(o)
	f, err := cpty.StartWithSize(cmd, &cpty.Winsize{Cols: uint16(cols), Rows: uint16(rows)})
	if err != nil {
		return nil, err
	}
	p := &PTY{base: newBase(), master: f, cmd: cmd}
	go p.wait()
	return p, nil
}

func (p *PTY) wait() {
	err := p.cmd.Wait()
	status := 0
	var ee *exec.ExitError
	switch {
	case errors.As(err, &ee):
		status = ee.ExitCode()
		if ws, ok := ee.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
			status = 128 + int(ws.Signal())
		}
	case err != nil:
		status = -1
	}
	p.exit(status, p.closeMaster)
}

func (p *PTY) closeMaster() {
	p.mu.Lock()
	f := p.master
	p.master = nil
	p.mu.Unlock()
	if f != nil {
		_ = f.Close()
	}
}

// Pid is the program's process id.
func (p *PTY) Pid() int { return p.cmd.Process.Pid }

// Read reads the program's output; io.EOF once the program has exited and its output is read.
func (p *PTY) Read(b []byte) (int, error) {
	p.mu.Lock()
	f := p.master
	p.mu.Unlock()
	if f == nil {
		return 0, io.EOF
	}
	n, err := p.read(f.Read, b)
	if err != nil { // EIO: nothing has the terminal open any more
		p.once.Do(p.closeMaster)
	}
	return n, err
}

// Write types b into the program.
func (p *PTY) Write(b []byte) (int, error) {
	p.mu.Lock()
	f := p.master
	p.mu.Unlock()
	if f == nil {
		return 0, ErrClosed
	}
	return f.Write(b)
}

// Resize gives the terminal a new size; after Close it does nothing and returns ErrClosed.
func (p *PTY) Resize(cols, rows int) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.master == nil {
		return ErrClosed
	}
	return cpty.Setsize(p.master, &cpty.Winsize{Cols: uint16(max(cols, 2)), Rows: uint16(max(rows, 1))})
}

// Close hangs up: the program's process group gets SIGHUP and, if it still runs closeGrace
// later, SIGKILL. It does not wait; Done tells when the program has gone.
func (p *PTY) Close() error {
	if !p.closing.CompareAndSwap(false, true) {
		return nil
	}
	pid := p.cmd.Process.Pid
	_ = syscall.Kill(-pid, syscall.SIGHUP) // the session's process group: what the shell runs too
	go func() {
		select {
		case <-p.exited:
		case <-time.After(closeGrace):
			_ = syscall.Kill(-pid, syscall.SIGKILL)
		}
		<-p.exited
		p.once.Do(p.closeMaster)
	}()
	return nil
}
