// Package qemu launches the QEMU machine a pyxforge.toml describes, follows its serial
// output, and controls it over QMP (pause, resume, reset, monitor commands, quit).
package qemu

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/obstinix/PyxForge/internal/config"
	"github.com/obstinix/PyxForge/internal/proc"
	"github.com/obstinix/PyxForge/internal/toolchain"
)

// Args builds QEMU's command line from [qemu] (2.x parity: qemu.rs build_qemu_args). With
// debug, QEMU starts paused with its GDB stub on the configured port. qmp is the loopback
// address for the QMP server; "" leaves QMP out.
func Args(q *config.Qemu, root string, debug bool, qmp string) []string {
	return args(q, root, debug, qmp, "")
}

// args is Args, booting the image through overlay (a qcow2 file backed by it) when given.
func args(q *config.Qemu, root string, debug bool, qmp, overlay string) []string {
	args := []string{"-machine", q.Machine, "-m", q.Memory}
	switch {
	case q.BootImage != "" && overlay != "":
		args = append(args, "-drive", "format=qcow2,file="+overlay)
	case q.BootImage != "":
		args = append(args, "-drive", "format=raw,file="+filepath.Join(root, filepath.FromSlash(q.BootImage)))
	}
	if q.Kernel != "" {
		args = append(args, "-kernel", filepath.Join(root, filepath.FromSlash(q.Kernel)))
	}
	args = append(args, q.ExtraArgs...)
	if debug && q.Debug.Enabled {
		if q.Debug.GdbPort == 1234 {
			args = append(args, "-s")
		} else {
			args = append(args, "-gdb", "tcp::"+strconv.Itoa(int(q.Debug.GdbPort)))
		}
		args = append(args, "-S")
	}
	if qmp != "" {
		args = append(args, "-qmp", "tcp:"+qmp+",server=on,wait=off")
	}
	return args
}

// Options control a launch.
type Options struct {
	Root  string // the project root; image paths are relative to it
	Debug bool   // start paused with the GDB stub
	// Overlay boots the image through this qcow2 overlay (PrepareOverlay), so machine
	// snapshots can be saved.
	Overlay string
	// OnOutput receives QEMU's output line by line: the serial port with -serial stdio, and
	// QEMU's own messages. It runs on a reader goroutine.
	OnOutput func(line string)
}

// Instance is a running QEMU.
type Instance struct {
	Path string   // the executable
	Args []string // its arguments
	QMP  *QMP     // the machine protocol connection; nil if it could not connect
	cmd  *exec.Cmd

	done    chan struct{}
	mu      sync.Mutex
	exitErr error
	tail    []string // the last lines of output, for failure messages
}

// Launch starts QEMU and connects to its QMP server. The image must exist: a missing boot
// image is reported before anything starts.
func Launch(ctx context.Context, q *config.Qemu, o Options) (*Instance, error) {
	for _, img := range []string{q.BootImage, q.Kernel} {
		if img == "" {
			continue
		}
		p := filepath.Join(o.Root, filepath.FromSlash(img))
		if _, err := os.Stat(p); err != nil {
			return nil, fmt.Errorf("%s does not exist: build the project first", img)
		}
	}
	exe, err := toolchain.LookPath(q.Executable)
	if err != nil {
		return nil, errors.New(missing(q.Executable))
	}
	port, err := freePort()
	if err != nil {
		return nil, err
	}
	qmpAddr := "127.0.0.1:" + strconv.Itoa(port)
	inst := &Instance{Path: exe, Args: args(q, o.Root, o.Debug, qmpAddr, o.Overlay), done: make(chan struct{})}
	cmd := exec.Command(exe, inst.Args...)
	cmd.Dir = o.Root
	proc.Bind(cmd)
	pr, pw, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	cmd.Stdout, cmd.Stderr = pw, pw
	cmd.Stdin = nil
	if err := cmd.Start(); err != nil {
		pr.Close()
		pw.Close()
		return nil, fmt.Errorf("start %s: %w", filepath.Base(exe), err)
	}
	pw.Close()
	inst.cmd = cmd
	read := make(chan struct{})
	go func() {
		defer close(read)
		inst.pump(pr, o.OnOutput)
	}()
	go func() {
		err := cmd.Wait()
		<-read
		pr.Close()
		inst.mu.Lock()
		inst.exitErr = err
		inst.mu.Unlock()
		close(inst.done)
	}()

	qctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	type dialed struct {
		q   *QMP
		err error
	}
	got := make(chan dialed, 1)
	go func() {
		c, err := DialQMP(qctx, qmpAddr)
		got <- dialed{c, err}
	}()
	select {
	case d := <-got:
		if d.err != nil {
			// QEMU that refuses its configuration exits while the connection is being made;
			// then its own message, not the connection error, says what is wrong.
			select {
			case <-inst.done:
				return nil, fmt.Errorf("QEMU exited at once%s%s", exitSuffix(inst.Err()), inst.tailText())
			case <-time.After(time.Second):
			}
			inst.Kill()
			return nil, fmt.Errorf("QEMU started but its QMP server did not answer: %w%s", d.err, inst.tailText())
		}
		inst.QMP = d.q
	case <-inst.done:
		cancel()
		return nil, fmt.Errorf("QEMU exited at once%s%s", exitSuffix(inst.Err()), inst.tailText())
	}
	return inst, nil
}

func (i *Instance) pump(r io.Reader, on func(string)) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimRight(sc.Text(), "\r")
		i.mu.Lock()
		i.tail = append(i.tail, line)
		if len(i.tail) > 20 {
			i.tail = i.tail[len(i.tail)-20:]
		}
		i.mu.Unlock()
		if on != nil {
			on(line)
		}
	}
}

func (i *Instance) tailText() string {
	i.mu.Lock()
	defer i.mu.Unlock()
	if len(i.tail) == 0 {
		return ""
	}
	return ":\n" + strings.Join(i.tail, "\n")
}

func exitSuffix(err error) string {
	if err == nil {
		return ""
	}
	return " (" + err.Error() + ")"
}

// Done is closed when QEMU exits.
func (i *Instance) Done() <-chan struct{} { return i.done }

// Err is how QEMU exited, once Done is closed.
func (i *Instance) Err() error {
	i.mu.Lock()
	defer i.mu.Unlock()
	return i.exitErr
}

// Pid is QEMU's process ID.
func (i *Instance) Pid() int { return i.cmd.Process.Pid }

// Stop asks QEMU to quit over QMP and waits up to two seconds, then kills it.
func (i *Instance) Stop() {
	if i.QMP != nil {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		_ = i.QMP.Execute(ctx, "quit", nil, nil)
		cancel()
	}
	select {
	case <-i.done:
	case <-time.After(2 * time.Second):
		i.Kill()
	}
	if i.QMP != nil {
		i.QMP.Close()
	}
}

// Kill ends QEMU at once and waits for it.
func (i *Instance) Kill() {
	_ = i.cmd.Process.Kill()
	<-i.done
	if i.QMP != nil {
		i.QMP.Close()
	}
}

func freePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, fmt.Errorf("find a free port for QMP: %w", err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

func missing(exe string) string {
	msg := exe + " is not installed or not on PATH"
	for _, t := range toolchain.Tools {
		for _, c := range t.Candidates {
			if c == exe {
				if h := (toolchain.Status{Tool: t}).Hint(); h != "" {
					msg += "; install: " + h
				}
				return msg
			}
		}
	}
	return msg
}
