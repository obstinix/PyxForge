package gdb

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/obstinix/PyxForge/internal/config"
	"github.com/obstinix/PyxForge/internal/inspect"
	"github.com/obstinix/PyxForge/internal/qemu"
	"github.com/obstinix/PyxForge/internal/toolchain"
)

func TestParseRecord(t *testing.T) {
	r, err := ParseRecord(`12^done,bkpt={number="1",type="breakpoint",addr="0x00007c00",thread-groups=["i1"]}`)
	if err != nil || r.Token != 12 || r.Kind != '^' || r.Class != "done" ||
		Str(r.Results, "bkpt", "addr") != "0x00007c00" || !reflect.DeepEqual(List(r.Results, "bkpt", "thread-groups"), []any{"i1"}) {
		t.Errorf("result: %+v, %v", r, err)
	}
	r, err = ParseRecord(`*stopped,reason="breakpoint-hit",disp="keep",bkptno="1",frame={addr="0x00007c00",func="??",args=[]},thread-id="1",stopped-threads="all"`)
	if err != nil || r.Token != -1 || r.Kind != '*' || r.Class != "stopped" {
		t.Fatalf("stopped: %+v, %v", r, err)
	}
	if st := StopOf(r); st.Reason != "breakpoint-hit" || st.Addr != 0x7c00 {
		t.Errorf("StopOf = %+v", st)
	}
	r, _ = ParseRecord(`^done,stack=[frame={level="0",addr="0x7c03",func="??"},frame={level="1",addr="0x0"}]`)
	if l := List(r.Results, "stack"); len(l) != 2 || Str(l[1], "addr") != "0x0" {
		t.Errorf("list of results: %+v", l)
	}
	r, _ = ParseRecord(`~"rax            0x0\t0\n\"q\" \101\\"`)
	if r.Kind != '~' || r.Text != "rax            0x0\t0\n\"q\" A\\" {
		t.Errorf("stream: %q", r.Text)
	}
	r, _ = ParseRecord(`^error,msg="No symbol table is loaded.  Use the \"file\" command."`)
	if r.Class != "error" || Str(r.Results, "msg") != `No symbol table is loaded.  Use the "file" command.` {
		t.Errorf("error: %+v", r)
	}
	if r, err := ParseRecord(`^running`); err != nil || r.Class != "running" || len(r.Results) != 0 {
		t.Errorf("bare class: %+v %v", r, err)
	}
	for _, bad := range []string{"", "^done,x=", `~"open`, "#what"} {
		if _, err := ParseRecord(bad); err == nil {
			t.Errorf("ParseRecord(%q) accepted", bad)
		}
	}
}

// sector is the hand-assembled boot sector from the qemu tests: "OK\n" to COM1, then halt.
func sector() []byte {
	b := make([]byte, 512)
	copy(b, []byte{0xBA, 0xF8, 0x03, 0xB0, 'O', 0xEE, 0xB0, 'K', 0xEE, 0xB0, 0x0A, 0xEE, 0xFA, 0xF4, 0xEB, 0xFD})
	b[510], b[511] = 0x55, 0xAA
	return b
}

func freePort(t *testing.T) int {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

// TestDebugBootSectorInQEMU attaches GDB to QEMU paused at reset, stops at 0x7c00 and reads
// the machine the way the inspector does.
func TestDebugBootSectorInQEMU(t *testing.T) {
	for _, tool := range []string{"qemu-system-x86_64", "gdb"} {
		if _, err := toolchain.LookPath(tool); err != nil {
			t.Skip(tool + " is not installed")
		}
	}
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "boot.bin"), sector(), 0o644); err != nil {
		t.Fatal(err)
	}
	port := freePort(t)
	c, err := config.Parse(fmt.Sprintf("[project]\nname = \"x\"\n[qemu]\nmemory = \"16M\"\nboot_image = \"boot.bin\"\n"+
		"extra_args = [\"-display\", \"none\"]\n[qemu.debug]\ngdb_port = %d\n", port))
	if err != nil {
		t.Fatal(err)
	}
	inst, err := qemu.Launch(context.Background(), c.Qemu, qemu.Options{Root: root, Debug: true})
	if err != nil {
		t.Fatal(err)
	}
	defer inst.Kill()

	stops := make(chan Stop, 8)
	var mu sync.Mutex
	var console strings.Builder
	s, err := Start(Options{
		OnConsole: func(text string) { mu.Lock(); console.WriteString(text); mu.Unlock() },
		OnExec: func(r Record) {
			if r.Class == "stopped" {
				stops <- StopOf(r)
			}
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	wait := func(what string) Stop {
		t.Helper()
		select {
		case st := <-stops:
			return st
		case <-time.After(15 * time.Second):
			mu.Lock()
			defer mu.Unlock()
			t.Fatalf("no stop: %s\n%s", what, console.String())
		}
		return Stop{}
	}

	if err := s.Attach(ctx, "i8086", "", fmt.Sprintf("localhost:%d", port)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Break(ctx, "*0x7c00"); err != nil {
		t.Fatal(err)
	}
	if err := s.Continue(ctx); err != nil {
		t.Fatal(err)
	}
	// The attach itself may report a stop at the reset vector first.
	st := wait("the breakpoint")
	for st.Addr != 0x7c00 {
		st = wait("the breakpoint at 0x7c00")
	}

	regs, err := s.Registers(ctx)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, r := range regs {
		got[r.Name] = r.Value
	}
	if pc := got["eip"] + got["pc"] + got["rip"]; !strings.Contains(pc, "7c00") {
		t.Errorf("program counter: %v", got)
	}
	mem, err := s.ReadMemory(ctx, 0x7c00, 512)
	if err != nil || !bytes.Equal(mem, sector()) {
		t.Errorf("memory at 0x7c00: % x, %v", mem[:min(16, len(mem))], err)
	}
	// GDB decodes with QEMU's x86-64 register description whatever architecture is set, so real
	// mode code is decoded from memory by package inspect; GDB's listing still gives addresses.
	ins, err := s.Disassemble(ctx, 0x7c00, 0x7c10)
	if err != nil || len(ins) < 3 || ins[0].Addr != 0x7c00 {
		t.Errorf("disassembly: %+v, %v", ins, err)
	}
	if real := inspect.Disassemble(mem[:16], 0x7c00, inspect.ModeOf("i8086"), true); real[0].Text != "mov dx, 0x3f8" {
		t.Errorf("real-mode decode of memory: %+v", real[0])
	}

	// One instruction: mov dx, 0x3f8 is three bytes long.
	if err := s.StepInstruction(ctx); err != nil {
		t.Fatal(err)
	}
	if st := wait("the step"); st.Addr != 0x7c03 {
		t.Errorf("after one step: %+v", st)
	}
	if frames, err := s.Frames(ctx); err != nil || len(frames) == 0 || frames[0].Addr != 0x7c03 {
		t.Errorf("frames: %+v, %v", frames, err)
	}
	if out, err := s.Console(ctx, "info registers dx"); err != nil || !strings.Contains(out, "0x3f8") {
		t.Errorf("console command: %q, %v", out, err)
	}
}
