package qemu

import (
	"bufio"
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/obstinix/PyxForge/internal/config"
	"github.com/obstinix/PyxForge/internal/toolchain"
)

func qemuConfig(t *testing.T, toml string) *config.Config {
	t.Helper()
	c, err := config.Parse("[project]\nname = \"os\"\n" + toml)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// The first four cases port legacy/core/src/qemu.rs's tests.
func TestArgs(t *testing.T) {
	root := filepath.FromSlash("/Projects/my-os")
	c := qemuConfig(t, "[qemu]\nboot_image = \"build/boot.bin\"\n")
	args := Args(c.Qemu, root, true, "127.0.0.1:4444")
	want := []string{"-machine", "pc", "-m", "128M", "-drive", "format=raw,file=" + filepath.Join(root, "build", "boot.bin"),
		"-s", "-S", "-qmp", "tcp:127.0.0.1:4444,server=on,wait=off"}
	if !slices.Equal(args, want) {
		t.Errorf("default:\n got %q\nwant %q", args, want)
	}

	c = qemuConfig(t, "[qemu]\nboot_image = \"build/boot.bin\"\n[qemu.debug]\ngdb_port = 5678\n")
	args = Args(c.Qemu, root, true, "")
	if !slices.Contains(args, "-gdb") || !slices.Contains(args, "tcp::5678") || !slices.Contains(args, "-S") || slices.Contains(args, "-s") {
		t.Errorf("custom port: %q", args)
	}
	if slices.Contains(args, "-qmp") {
		t.Errorf("QMP without an address: %q", args)
	}

	if args := Args(c.Qemu, root, false, ""); slices.Contains(args, "-S") || slices.Contains(args, "-gdb") {
		t.Errorf("without debug: %q", args)
	}

	c = qemuConfig(t, "[qemu]\nmemory = \"256M\"\nkernel = \"build/kernel.elf\"\nextra_args = [\"-serial\", \"stdio\"]\n")
	args = Args(c.Qemu, root, true, "")
	if slices.Contains(args, "-drive") || !slices.Contains(args, filepath.Join(root, "build", "kernel.elf")) ||
		!slices.Contains(args, "256M") || !slices.Contains(args, "stdio") {
		t.Errorf("kernel boot: %q", args)
	}

	c = qemuConfig(t, "[qemu]\nboot_image = \"b.bin\"\n[qemu.debug]\nenabled = false\n")
	if args := Args(c.Qemu, root, true, ""); slices.Contains(args, "-S") {
		t.Errorf("debug disabled in the file still paused: %q", args)
	}
}

// fakeQMP serves the QMP handshake and a few commands on a loopback port.
func fakeQMP(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { l.Close() })
	go func() {
		c, err := l.Accept()
		if err != nil {
			return
		}
		defer c.Close()
		w := func(s string) { _, _ = c.Write([]byte(s + "\r\n")) }
		w(`{"QMP": {"version": {"qemu": {"major": 11, "minor": 1, "micro": 0}}, "capabilities": []}}`)
		sc := bufio.NewScanner(c)
		running := true
		for sc.Scan() {
			var m struct {
				Execute   string            `json:"execute"`
				Arguments map[string]string `json:"arguments"`
			}
			_ = json.Unmarshal(sc.Bytes(), &m)
			switch m.Execute {
			case "qmp_capabilities":
				w(`{"return": {}}`)
			case "query-status":
				st := "paused"
				if running {
					st = "running"
				}
				w(`{"return": {"status": "` + st + `", "running": ` + map[bool]string{true: "true", false: "false"}[running] + `}}`)
			case "stop":
				running = false
				w(`{"timestamp": {"seconds": 1, "microseconds": 2}, "event": "STOP"}`)
				w(`{"return": {}}`)
			case "human-monitor-command":
				w(`{"return": "EAX=00000000 ` + m.Arguments["command-line"] + `\r\n"}`)
			default:
				w(`{"error": {"class": "CommandNotFound", "desc": "The command ` + m.Execute + ` has not been found"}}`)
			}
		}
	}()
	return l.Addr().String()
}

func TestQMPClient(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	q, err := DialQMP(ctx, fakeQMP(t))
	if err != nil {
		t.Fatal(err)
	}
	defer q.Close()
	if st, running, err := q.Status(ctx); err != nil || st != "running" || !running {
		t.Errorf("Status = %q %v %v", st, running, err)
	}
	if err := q.Execute(ctx, "stop", nil, nil); err != nil {
		t.Fatal(err)
	}
	select {
	case ev := <-q.Events():
		if ev.Name != "STOP" {
			t.Errorf("event %+v", ev)
		}
	case <-time.After(5 * time.Second):
		t.Error("no STOP event")
	}
	if st, running, _ := q.Status(ctx); st != "paused" || running {
		t.Errorf("after stop: %q %v", st, running)
	}
	if out, err := q.HMP(ctx, "info registers"); err != nil || !strings.Contains(out, "EAX=") {
		t.Errorf("HMP = %q, %v", out, err)
	}
	if err := q.Execute(ctx, "nonsense", nil, nil); err == nil || !strings.Contains(err.Error(), "has not been found") {
		t.Errorf("unknown command: %v", err)
	}
	q.Close()
	if err := q.Execute(ctx, "query-status", nil, nil); err == nil {
		t.Error("a command succeeded after Close")
	}
}

// bootSector is a hand-assembled 512-byte sector: it writes "OK\n" to COM1 and halts.
//
//	mov dx, 0x3f8 / mov al, 'O' / out dx, al / mov al, 'K' / out dx, al / mov al, 10 /
//	out dx, al / cli / hlt / jmp $-1
func bootSector() []byte {
	b := make([]byte, 512)
	copy(b, []byte{0xBA, 0xF8, 0x03, 0xB0, 'O', 0xEE, 0xB0, 'K', 0xEE, 0xB0, 0x0A, 0xEE, 0xFA, 0xF4, 0xEB, 0xFD})
	b[510], b[511] = 0x55, 0xAA
	return b
}

// writeImage writes bootSector to a new folder as boot.bin and returns the folder.
func writeImage(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "boot.bin"), bootSector(), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func needQEMU(t *testing.T) {
	t.Helper()
	if _, err := toolchain.LookPath("qemu-system-x86_64"); err != nil {
		t.Skip("qemu-system-x86_64 is not installed")
	}
}

func TestLaunchRunsTheMachine(t *testing.T) {
	needQEMU(t)
	root := writeImage(t)
	c := qemuConfig(t, "[qemu]\nmemory = \"16M\"\nboot_image = \"boot.bin\"\nextra_args = [\"-serial\", \"stdio\", \"-display\", \"none\", \"-no-reboot\"]\n")
	var mu sync.Mutex
	var out []string
	inst, err := Launch(context.Background(), c.Qemu, Options{Root: root, OnOutput: func(l string) {
		mu.Lock()
		out = append(out, l)
		mu.Unlock()
	}})
	if err != nil {
		t.Fatal(err)
	}
	defer inst.Kill()
	deadline := time.Now().Add(10 * time.Second)
	for {
		mu.Lock()
		ok := slices.Contains(out, "OK")
		mu.Unlock()
		if ok {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("no serial output; got %q", out)
		}
		time.Sleep(20 * time.Millisecond)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, running, err := inst.QMP.Status(ctx); err != nil || !running {
		t.Errorf("status: running %v, %v", running, err)
	}
	regs, err := inst.QMP.HMP(ctx, "info registers")
	if err != nil || !strings.Contains(regs, "EIP=") {
		t.Errorf("info registers: %v\n%s", err, regs)
	}
	if err := inst.QMP.Execute(ctx, "stop", nil, nil); err != nil {
		t.Fatal(err)
	}
	if st, running, _ := inst.QMP.Status(ctx); running || st != "paused" {
		t.Errorf("after stop: %q", st)
	}
	inst.Stop()
	select {
	case <-inst.Done():
	default:
		t.Error("QEMU still running after Stop")
	}
}

func TestLaunchReportsProblems(t *testing.T) {
	root := t.TempDir()
	c := qemuConfig(t, "[qemu]\nboot_image = \"build/boot.bin\"\n")
	if _, err := Launch(context.Background(), c.Qemu, Options{Root: root}); err == nil || !strings.Contains(err.Error(), "build the project first") {
		t.Errorf("missing image: %v", err)
	}
	root = writeImage(t)
	c = qemuConfig(t, "[qemu]\nexecutable = \"qemu-system-nonesuch\"\nboot_image = \"boot.bin\"\n")
	if _, err := Launch(context.Background(), c.Qemu, Options{Root: root}); err == nil || !strings.Contains(err.Error(), "not installed") {
		t.Errorf("missing QEMU: %v", err)
	}
	needQEMU(t)
	c = qemuConfig(t, "[qemu]\nboot_image = \"boot.bin\"\nextra_args = [\"-no-such-option\"]\n")
	if _, err := Launch(context.Background(), c.Qemu, Options{Root: root}); err == nil || !strings.Contains(err.Error(), "no-such-option") {
		t.Errorf("bad option: %v", err)
	}
}
