package gdb

import (
	"bufio"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/obstinix/PyxForge/internal/proc"
	"github.com/obstinix/PyxForge/internal/toolchain"
)

// Options control a GDB session.
type Options struct {
	Executable string // "gdb" by default
	// OnConsole receives GDB's console and log output (what a terminal GDB would print).
	OnConsole func(text string)
	// OnExec receives execution state changes: *stopped and *running records.
	OnExec func(Record)
}

// Session is one GDB process speaking GDB/MI.
type Session struct {
	cmd   *exec.Cmd
	stdin io.WriteCloser
	opts  Options

	wmu     sync.Mutex // one command at a time
	pmu     sync.Mutex
	next    int
	pending map[int]chan Record
	console strings.Builder // console text of the command in flight
	done    chan struct{}
	err     error

	regNames []string
}

// Start runs GDB with the MI interpreter, without any .gdbinit (-nx).
func Start(o Options) (*Session, error) {
	exe := o.Executable
	if exe == "" {
		exe = "gdb"
	}
	path, err := toolchain.LookPath(exe)
	if err != nil {
		return nil, fmt.Errorf("%s is not installed or not on PATH; pyxforge doctor shows how to install GDB", exe)
	}
	cmd := exec.Command(path, "--interpreter=mi", "-q", "-nx")
	proc.Bind(cmd)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	cmd.Stderr = cmd.Stdout
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start %s: %w", exe, err)
	}
	s := &Session{cmd: cmd, stdin: stdin, opts: o, next: 1, pending: map[int]chan Record{}, done: make(chan struct{})}
	go s.read(stdout)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for _, c := range []string{"-gdb-set confirm off", "-gdb-set pagination off", "-gdb-set width 0", "-gdb-set height 0"} {
		if _, err := s.Run(ctx, c); err != nil {
			s.Close()
			return nil, err
		}
	}
	return s, nil
}

func (s *Session) read(r io.Reader) {
	defer func() {
		s.pmu.Lock()
		if s.err == nil {
			s.err = errors.New("GDB exited")
		}
		for _, ch := range s.pending {
			close(ch)
		}
		s.pending = map[int]chan Record{}
		s.pmu.Unlock()
		close(s.done)
	}()
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 8*1024*1024)
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "(gdb)") || strings.TrimSpace(line) == "" {
			continue
		}
		rec, err := ParseRecord(line)
		if err != nil {
			if s.opts.OnConsole != nil {
				s.opts.OnConsole(line + "\n") // the program's own output, or something MI did not frame
			}
			continue
		}
		switch rec.Kind {
		case '^':
			s.pmu.Lock()
			ch := s.pending[rec.Token]
			delete(s.pending, rec.Token)
			s.pmu.Unlock()
			if ch != nil {
				ch <- rec
			}
		case '~', '@', '&':
			s.pmu.Lock()
			s.console.WriteString(rec.Text)
			s.pmu.Unlock()
			if s.opts.OnConsole != nil {
				s.opts.OnConsole(rec.Text)
			}
		case '*':
			if s.opts.OnExec != nil {
				s.opts.OnExec(rec)
			}
		}
	}
}

// Error is GDB's ^error reply.
type Error struct{ Msg string }

func (e *Error) Error() string { return e.Msg }

// Run sends one MI command and waits for its result. The console text GDB printed while it ran
// is in the result's Text.
func (s *Session) Run(ctx context.Context, command string) (Record, error) {
	s.wmu.Lock()
	defer s.wmu.Unlock()
	s.pmu.Lock()
	if s.pending == nil || isClosed(s.done) {
		s.pmu.Unlock()
		return Record{}, errors.New("GDB is not running")
	}
	tok := s.next
	s.next++
	ch := make(chan Record, 1)
	s.pending[tok] = ch
	s.console.Reset()
	s.pmu.Unlock()
	if _, err := io.WriteString(s.stdin, strconv.Itoa(tok)+command+"\n"); err != nil {
		return Record{}, fmt.Errorf("send to GDB: %w", err)
	}
	select {
	case rec, ok := <-ch:
		if !ok {
			return Record{}, s.exitErr()
		}
		s.pmu.Lock()
		rec.Text = s.console.String()
		s.pmu.Unlock()
		if rec.Class == "error" {
			return rec, &Error{Msg: Str(rec.Results, "msg")}
		}
		return rec, nil
	case <-ctx.Done():
		s.pmu.Lock()
		delete(s.pending, tok)
		s.pmu.Unlock()
		return Record{}, fmt.Errorf("GDB: %s: %w", command, ctx.Err())
	}
}

func (s *Session) exitErr() error {
	s.pmu.Lock()
	defer s.pmu.Unlock()
	return s.err
}

func isClosed(c chan struct{}) bool {
	select {
	case <-c:
		return true
	default:
		return false
	}
}

// Console runs a command as typed at GDB's prompt and returns what it printed.
func (s *Session) Console(ctx context.Context, line string) (string, error) {
	rec, err := s.Run(ctx, "-interpreter-exec console "+quote(line))
	return rec.Text, err
}

func quote(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`)
	return `"` + r.Replace(s) + `"`
}

// Done is closed when GDB exits.
func (s *Session) Done() <-chan struct{} { return s.done }

// Close ends GDB: -gdb-exit, then a kill if it has not gone within two seconds.
func (s *Session) Close() {
	if isClosed(s.done) {
		_ = s.cmd.Wait()
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	_, _ = s.Run(ctx, "-gdb-exit")
	cancel()
	s.stdin.Close()
	select {
	case <-s.done:
	case <-time.After(2 * time.Second):
		_ = s.cmd.Process.Kill()
		<-s.done
	}
	_ = s.cmd.Wait()
}

// Attach loads symbols from file when given, connects to a remote stub such as QEMU's (target
// "localhost:1234"), then sets the architecture GDB decodes instructions for. The order
// matters: QEMU describes its registers when GDB connects, and an architecture set before
// that (i8086 against qemu-system-x86_64) makes GDB reject the register packet.
func (s *Session) Attach(ctx context.Context, arch, file, target string) error {
	if file != "" {
		if _, err := s.Run(ctx, "-file-symbol-file "+quote(file)); err != nil {
			return err
		}
	}
	if _, err := s.Run(ctx, "-target-select remote "+target); err != nil {
		return err
	}
	if arch != "" && arch != "auto" {
		if _, err := s.Console(ctx, "set architecture "+arch); err != nil {
			return err
		}
	}
	return nil
}

// Register is one register's name and value.
type Register struct {
	Name  string
	Value string // hexadecimal, as GDB formats it ("0x7c00"); flags and vectors as GDB prints them
}

// Registers reads every general register GDB knows for the target.
func (s *Session) Registers(ctx context.Context) ([]Register, error) {
	if s.regNames == nil {
		rec, err := s.Run(ctx, "-data-list-register-names")
		if err != nil {
			return nil, err
		}
		for _, n := range List(rec.Results, "register-names") {
			name, _ := n.(string)
			s.regNames = append(s.regNames, name)
		}
	}
	rec, err := s.Run(ctx, "-data-list-register-values --skip-unavailable x")
	if err != nil {
		return nil, err
	}
	var out []Register
	for _, v := range List(rec.Results, "register-values") {
		n, err := strconv.Atoi(Str(v, "number"))
		if err != nil || n < 0 || n >= len(s.regNames) || s.regNames[n] == "" {
			continue
		}
		out = append(out, Register{Name: s.regNames[n], Value: Str(v, "value")})
	}
	return out, nil
}

// ReadMemory reads n bytes at addr.
func (s *Session) ReadMemory(ctx context.Context, addr uint64, n int) ([]byte, error) {
	rec, err := s.Run(ctx, fmt.Sprintf("-data-read-memory-bytes 0x%x %d", addr, n))
	if err != nil {
		return nil, err
	}
	var out []byte
	for _, blk := range List(rec.Results, "memory") {
		b, err := hex.DecodeString(Str(blk, "contents"))
		if err != nil {
			return nil, err
		}
		out = append(out, b...)
	}
	return out, nil
}

// Instruction is one disassembled instruction.
type Instruction struct {
	Addr uint64
	Text string // mnemonic and operands
	Func string // the enclosing symbol, if any, with offset
}

// Disassemble decodes the instructions in [start, end).
func (s *Session) Disassemble(ctx context.Context, start, end uint64) ([]Instruction, error) {
	rec, err := s.Run(ctx, fmt.Sprintf("-data-disassemble -s 0x%x -e 0x%x -- 0", start, end))
	if err != nil {
		return nil, err
	}
	var out []Instruction
	for _, in := range List(rec.Results, "asm_insns") {
		addr, _ := strconv.ParseUint(strings.TrimPrefix(Str(in, "address"), "0x"), 16, 64)
		ins := Instruction{Addr: addr, Text: strings.Join(strings.Fields(Str(in, "inst")), " ")}
		if f := Str(in, "func-name"); f != "" {
			ins.Func = f + "+" + Str(in, "offset")
		}
		out = append(out, ins)
	}
	return out, nil
}

// Frame is one stack frame.
type Frame struct {
	Level int
	Addr  uint64
	Func  string
	File  string
	Line  int
}

// Frames lists the call stack, innermost first.
func (s *Session) Frames(ctx context.Context) ([]Frame, error) {
	rec, err := s.Run(ctx, "-stack-list-frames 0 31")
	if err != nil {
		return nil, err
	}
	var out []Frame
	for _, f := range List(rec.Results, "stack") {
		fr := Frame{Func: Str(f, "func"), File: Str(f, "fullname")}
		fr.Level, _ = strconv.Atoi(Str(f, "level"))
		fr.Addr, _ = strconv.ParseUint(strings.TrimPrefix(Str(f, "addr"), "0x"), 16, 64)
		fr.Line, _ = strconv.Atoi(Str(f, "line"))
		out = append(out, fr)
	}
	return out, nil
}

// Breakpoint is a breakpoint GDB set.
type Breakpoint struct {
	Number   string
	Location string // what it was set at ("*0x7c00", "kmain")
	Addr     string
}

// Break sets a breakpoint at a location GDB understands: "*0x7c00", "kmain", "boot.asm:20".
func (s *Session) Break(ctx context.Context, location string) (Breakpoint, error) {
	rec, err := s.Run(ctx, "-break-insert "+location)
	if err != nil {
		return Breakpoint{}, err
	}
	return Breakpoint{Number: Str(rec.Results, "bkpt", "number"), Location: location, Addr: Str(rec.Results, "bkpt", "addr")}, nil
}

// Delete removes a breakpoint by number.
func (s *Session) Delete(ctx context.Context, number string) error {
	_, err := s.Run(ctx, "-break-delete "+number)
	return err
}

// Continue, Interrupt and the steps return once GDB accepts them; the stop that follows
// arrives through Options.OnExec as a *stopped record.
func (s *Session) Continue(ctx context.Context) error { return s.exec(ctx, "-exec-continue") }

// Interrupt stops a running target.
func (s *Session) Interrupt(ctx context.Context) error { return s.exec(ctx, "-exec-interrupt") }

// StepInstruction runs one machine instruction, into calls.
func (s *Session) StepInstruction(ctx context.Context) error {
	return s.exec(ctx, "-exec-step-instruction")
}

// NextInstruction runs one machine instruction, over calls.
func (s *Session) NextInstruction(ctx context.Context) error {
	return s.exec(ctx, "-exec-next-instruction")
}

func (s *Session) exec(ctx context.Context, cmd string) error {
	_, err := s.Run(ctx, cmd)
	return err
}

// Stop is a *stopped record in plain terms.
type Stop struct {
	Reason string // breakpoint-hit, end-stepping-range, signal-received…
	Addr   uint64
	Func   string
}

// StopOf reads a *stopped record.
func StopOf(r Record) Stop {
	st := Stop{Reason: Str(r.Results, "reason"), Func: Str(r.Results, "frame", "func")}
	st.Addr, _ = strconv.ParseUint(strings.TrimPrefix(Str(r.Results, "frame", "addr"), "0x"), 16, 64)
	return st
}
