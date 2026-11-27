package shell

import (
	"fmt"
	"time"

	"fyne.io/fyne/v2"
	"github.com/obstinix/PyxForge/internal/vt"
)

// Review renders of the Terminal (tools/snapshot): sessions whose screens hold what an
// integrated shell writes, so the renders show blocks without depending on the machine's shell.

// demoScreen writes integrated commands onto a screen, each taking the given time.
func demoScreen(cmds []demoCmd) *vt.Screen {
	sc := vt.New(110, 20)
	clock := time.Date(2026, 11, 27, 10, 0, 0, 0, time.UTC)
	old := vt.Clock
	defer func() { vt.Clock = old }()
	vt.Clock = func() time.Time { return clock }
	fmt.Fprint(sc, "\x1b]7;file://host/home/dev/boot\x07")
	for _, c := range cmds {
		fmt.Fprintf(sc, "\x1b]133;A\x07\x1b[32m~/boot\x1b[0m $ \x1b]133;B\x07%s\r\n\x1b]133;C\x07", c.cmd)
		fmt.Fprint(sc, c.out)
		clock = clock.Add(c.took)
		if c.exit >= 0 {
			fmt.Fprintf(sc, "\x1b]133;D;%d\x07", c.exit)
		}
	}
	if len(cmds) > 0 && cmds[len(cmds)-1].exit >= 0 {
		fmt.Fprint(sc, "\x1b]133;A\x07\x1b[32m~/boot\x1b[0m $ \x1b]133;B\x07")
	}
	return sc
}

type demoCmd struct {
	cmd, out string
	exit     int
	took     time.Duration
}

var demoBuild = []demoCmd{
	{"nasm -f bin boot.asm -o boot.img", "", 0, 400 * time.Millisecond},
	{"ls -l boot.img", "-rw-r--r-- 1 dev dev 512 Nov 27 10:00 boot.img\r\n", 0, 100 * time.Millisecond},
	{"make -j8", "nasm -f bin boot.asm -o boot.img\r\ncc -O2 -c loader.c -o loader.o\r\n" +
		"loader.c:14:9: \x1b[31merror:\x1b[0m implicit declaration of function 'serial_puts'\r\n" +
		"loader.c:21:5: \x1b[35mwarning:\x1b[0m unused variable 'port'\r\nmake: *** [Makefile:9: loader.o] Error 1\r\n", 2, 2400 * time.Millisecond},
	{"git status --short", " M boot.asm\r\n M loader.c\r\n?? notes.md\r\n", 0, 200 * time.Millisecond},
}

// demoSession adds a finished-looking shell session with a demo screen.
func (t *terminalHost) demoSession(name string, cmds []demoCmd) *termSession {
	ts := &termSession{name: name, kind: kindShell, screen: demoScreen(cmds), blocks: true, start: time.Now()}
	ts.send, ts.sendInput = func([]byte) {}, true // takes keys, so its cursor shows
	t.sessions = append(t.sessions, ts)
	return ts
}

// ShowTerminalDemo opens the Terminal with demo sessions: one with a successful command, a
// failed one and a collapsed one, the pointer over the failed one; split shows two sessions
// side by side. For review renders.
func (s *Shell) ShowTerminalDemo(split bool) {
	t := s.term
	a := t.demoSession("Shell 1", demoBuild)
	t.demoSession("Run hello.c", []demoCmd{{"./hello", "Hello, PyxForge\r\n", 0, 50 * time.Millisecond}}).kind = kindRun
	serial := t.console(kindSerial, "QEMU serial", nil)
	t.consoleLine(serial, "SeaBIOS (version 1.16.3)", "Booting from Hard Disk...", "PyxForge boot sector: hello over COM1")
	failedRun := &termSession{name: "Run count.c", kind: kindRun, screen: vt.New(80, 5), done: true, status: 1,
		start: time.Now().Add(-1200 * time.Millisecond), end: time.Now()}
	t.sessions = append(t.sessions, failedRun)
	s.bench.dockH = 360 // room for the blocks
	s.showDockItem(t.tab)
	s.content.Refresh()
	t.showSession(a)
	if split {
		t.splitRight(serial)
		t.activate(t.panes[0])
		t.sync()
	}
	v := t.panes[0].view
	v.Refresh()
	bs := v.Blocks()
	if len(bs) >= 2 {
		v.ToggleCollapse(bs[1]) // ls: one summary line
	}
	// The pointer over make's output, so its header shows.
	cell := v.CellSize()
	for _, b := range v.Blocks() {
		if b.Exit == 2 {
			row := b.PromptLine - v.TopLine() + 1
			v.HoverAt(fyne.NewPos(200, 12+float32(max(row, 0))*cell.Height+cell.Height/2))
		}
	}
}
