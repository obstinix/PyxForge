package shell

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"github.com/obstinix/PyxForge/internal/config"
	"github.com/obstinix/PyxForge/internal/gdb"
	"github.com/obstinix/PyxForge/internal/inspect"
	"github.com/obstinix/PyxForge/internal/neovim"
	"github.com/obstinix/PyxForge/internal/qemu"
	"github.com/obstinix/PyxForge/internal/ui/icons"
	"github.com/obstinix/PyxForge/internal/ui/kit"
	"github.com/obstinix/PyxForge/internal/ui/notifications"
	"github.com/obstinix/PyxForge/internal/ui/theme"
)

// machine runs the project in QEMU and, for a debug session, attaches GDB to it. It owns the
// QEMU and GDB tabs and feeds the inspector's machine views. Its state changes on the UI thread
// only; QEMU and GDB report through s.dispatch.
type machine struct {
	s         *Shell
	qemuTab   *container.TabItem
	gdbTab    *container.TabItem
	qemuOut   *outputView
	gdbOut    *outputView
	state     *kit.Text
	run       *kit.IconButton
	debug     *kit.IconButton
	pause     *kit.IconButton
	step      *kit.IconButton
	stop      *kit.IconButton
	monitor   *widget.Entry
	gdbInput  *widget.Entry
	controls  *fyne.Container   // the bar holding the buttons, laid out again as they show and hide
	setupView fyne.CanvasObject // Set Up Machine…, in place of the output while there is no machine
	prev      *qemu.Instance    // the QEMU last stopped, which a new launch waits for

	cfg      *config.Config
	inst     *qemu.Instance
	dbg      *gdb.Session
	arch     string
	mode     inspect.Mode
	debugOn  bool // this session has GDB
	paused   bool
	pc       uint64
	starting bool
	gen      int // the session; events from older sessions are ignored
	views    *machineViews

	// Editor breakpoints in this GDB session: "file:line" (relative to the project) → GDB's
	// number. A change made while the machine runs waits for the next stop (bpDirty, bpWant):
	// GDB sets breakpoints in a remote target only while it is stopped.
	bpNums  map[string]string
	bpWant  []neovim.Breakpoint
	bpDirty bool

	// Launches run in the background and hand QEMU and GDB to the UI thread through
	// s.dispatch, which drops work once the shell has shut down. Until a process is handed
	// over it is held here, so that shutdown stops it all the same.
	life     context.Context // ends at shutdown, cancelling launches in progress
	endLife  context.CancelFunc
	heldMu   sync.Mutex
	held     map[int]func() // how to stop each process not yet handed over
	nextHeld int
	down     bool // shutdown has run
}

func (s *Shell) buildMachine() (*container.TabItem, *container.TabItem) {
	m := &machine{s: s, views: s.mviews, held: map[int]func(){}} // the inspector, and its views, are built first
	m.life, m.endLife = context.WithCancel(context.Background())
	s.mach = m
	m.qemuOut = newOutputView(s, 5000)
	m.gdbOut = newOutputView(s, 5000)
	m.state = kit.NewText("QEMU is not running", kit.Body, kit.Secondary)
	m.state.TextSize = theme.TextCaption + 1
	m.run = kit.NewIconButton(icons.Play, "Run in QEMU (Ctrl+Shift+R)", func() { m.start(false) })
	m.debug = kit.NewIconButton(icons.Bug, "Debug in QEMU (Ctrl+Shift+D)", func() { m.start(true) })
	m.pause = kit.NewIconButton(icons.Pause, "Pause", m.togglePause)
	m.step = kit.NewIconButton(icons.StepForward, "Step instruction (Ctrl+Shift+F11)", m.stepInstruction)
	m.stop = kit.NewIconButton(icons.Square, "Stop QEMU", m.halt)
	m.controls = container.NewHBox(m.state, layout.NewSpacer(), m.run, m.debug, m.pause, m.step, m.stop)
	bar := kit.Row(theme.TabBarHeight, m.controls)
	top := container.NewVBox(container.New(layout.NewCustomPaddedLayout(0, 0, theme.Space2, theme.Space1), bar), kit.NewRule(false))

	m.monitor = widget.NewEntry()
	m.monitor.SetPlaceHolder("QEMU monitor command, such as info registers or x /16xb 0x7c00")
	m.monitor.OnSubmitted = m.runMonitor
	// In a scroll: the dock may be shorter than the message, which must not spill over the bar.
	m.setupView = container.NewVScroll(placeholderAction(icons.CPU, "No machine set up",
		"Run and Debug in QEMU boot this project from the [qemu] table of "+config.FileName+".",
		"Set Up Machine…", m.setup))
	body := container.NewStack(m.qemuOut.Widget(), m.setupView)
	m.qemuTab = container.NewTabItem("QEMU", container.NewBorder(top, container.NewPadded(m.monitor), nil, nil, body))
	m.showSetup()

	m.gdbInput = widget.NewEntry()
	m.gdbInput.SetPlaceHolder("GDB command, such as info registers, x/8i $pc or break *0x7c10")
	m.gdbInput.OnSubmitted = m.runGDB
	m.gdbTab = container.NewTabItem("GDB", container.NewBorder(nil, container.NewPadded(m.gdbInput), nil, nil, m.gdbOut.Widget()))
	m.sync()
	return m.qemuTab, m.gdbTab
}

// sync shows the controls for the current state: Run and Debug while nothing runs; Pause or
// Continue, Step and Stop while QEMU runs.
func (m *machine) sync() {
	running := m.inst != nil || m.starting
	show := func(b *kit.IconButton, on bool) {
		if on {
			b.Show()
		} else {
			b.Hide()
		}
	}
	show(m.run, !running)
	show(m.debug, !running)
	show(m.pause, running)
	show(m.step, running && m.debugOn)
	show(m.stop, running)
	if m.paused {
		m.pause.Icon, m.pause.Label = icons.Play, "Continue (Ctrl+Shift+F5)"
	} else {
		m.pause.Icon, m.pause.Label = icons.Pause, "Pause"
	}
	// While GDB attaches, QEMU must not be resumed behind its back.
	if m.inst != nil && (m.dbg != nil || !m.debugOn) {
		m.pause.Enable()
	} else {
		m.pause.Disable()
	}
	if m.debugOn && m.paused && m.dbg != nil {
		m.step.Enable()
	} else {
		m.step.Disable()
	}
	m.pause.Refresh()
	m.controls.Refresh()
}

func (m *machine) setState(text string) {
	m.state.SetText(text)
	m.s.statusMachine.SetText(text)
	if visible := m.inst != nil || m.starting; visible != m.s.statusMachine.Visible() {
		if visible {
			m.s.statusMachine.Show()
		} else {
			m.s.statusMachine.Hide()
		}
		m.s.statusMachine.bar.Refresh()
	}
	m.sync()
}

func (m *machine) showTab() {
	m.s.showDockTab(slices.Index(m.s.dock.Items, m.qemuTab))
}

// start builds the project, then boots it; with debug, QEMU starts paused and GDB attaches.
// A running machine is stopped first.
func (m *machine) start(debug bool) {
	if m.starting {
		return
	}
	if m.inst != nil {
		m.halt()
	}
	c, err := config.Load(m.s.root)
	file := filepath.Join(m.s.root, config.FileName)
	switch {
	case errors.Is(err, config.ErrNotFound) || err == nil && c.Qemu == nil:
		m.showTab()
		m.showSetup()
		m.s.choose("No machine set up", "This project has no [qemu] table in "+config.FileName+" saying what to boot. Set one up: the file to boot, the QEMU that boots it, and the port GDB connects to.",
			choice{label: "Set Up Machine…", primary: true, run: m.setupForm})
		return
	case err != nil:
		m.s.choose("Cannot run: "+config.FileName+" has a problem", err.Error(),
			choice{label: "Open " + config.FileName, primary: true, run: func() { m.s.OpenFile(file) }})
		return
	}
	m.cfg = c
	m.showTab()
	m.qemuOut.Clear()
	m.starting = true
	if len(c.Profiles) == 0 {
		m.launch(debug)
		return
	}
	if m.s.buildp.running {
		m.starting = false
		m.s.Notify(notifications.Info, "A build is running", "Run again when it ends.")
		m.sync()
		return
	}
	m.setState("Building…")
	m.s.buildp.after = func(ok bool) {
		if !ok {
			m.starting = false
			m.setState("The build failed; not booting")
			return
		}
		m.launch(debug)
	}
	m.s.buildp.start()
	if !m.s.buildp.running { // the build could not start; start explains why
		m.s.buildp.after = nil
		m.starting = false
		m.setState("QEMU is not running")
	}
}

func (m *machine) launch(debug bool) {
	m.setState("Starting QEMU…")
	cfg, root, out, prev := m.cfg, m.s.root, m.qemuOut, m.prev
	go func() {
		if prev != nil { // a QEMU being stopped may still hold the GDB and QMP ports
			select {
			case <-prev.Done():
			case <-time.After(5 * time.Second):
			case <-m.life.Done():
			}
		}
		ctx, cancel := context.WithTimeout(m.life, 20*time.Second)
		defer cancel()
		opts := qemu.Options{Root: root, Debug: debug, OnOutput: out.Add}
		if cfg.Qemu.Snapshots && cfg.Qemu.BootImage != "" {
			// Boot through the overlay that holds machine snapshots; without it, boot the image.
			dir, err := qemu.OverlayDir(root)
			if err == nil {
				var made string
				opts.Overlay, made, err = qemu.PrepareOverlay(ctx, filepath.Join(root, filepath.FromSlash(cfg.Qemu.BootImage)), dir)
				if err == nil && made != "" {
					out.Add("==> New snapshot overlay: " + made + "; earlier machine snapshots no longer apply")
				}
			}
			if err != nil {
				out.Add("==> Machine snapshots unavailable: " + err.Error())
			}
		}
		inst, err := qemu.Launch(ctx, cfg.Qemu, opts)
		release := func() {}
		if err == nil {
			var ok bool
			if release, ok = m.hold(inst.Stop); !ok {
				return // the shell shut down meanwhile; hold stopped QEMU
			}
		}
		m.s.dispatch(func() { release(); m.launched(inst, err, debug) })
	}()
}

// hold keeps stop until the UI thread takes the process over (release), so that a shutdown
// in between still stops it. After shutdown it stops the process at once and reports false.
func (m *machine) hold(stop func()) (release func(), ok bool) {
	m.heldMu.Lock()
	if m.down {
		m.heldMu.Unlock()
		stop()
		return nil, false
	}
	id := m.nextHeld
	m.nextHeld++
	m.held[id] = stop
	m.heldMu.Unlock()
	return func() {
		m.heldMu.Lock()
		delete(m.held, id)
		m.heldMu.Unlock()
	}, true
}

func (m *machine) launched(inst *qemu.Instance, err error, debug bool) {
	m.starting = false
	if err != nil {
		m.qemuOut.AddNow(strings.Split(err.Error(), "\n")...)
		m.setState("QEMU did not start")
		m.s.Notify(notifications.Error, "QEMU did not start", firstLine(err.Error()))
		return
	}
	m.gen++
	gen := m.gen
	m.inst, m.debugOn, m.paused, m.pc = inst, debug && m.cfg.Qemu.Debug.Enabled, false, 0
	g := m.cfg.GdbFor("")
	m.arch, m.mode = g.Architecture, inspect.ModeOf(g.Architecture)
	m.qemuOut.AddNow(commandLines(filepath.Base(inst.Path), inst.Args)...)
	m.s.logf("QEMU started (pid %d)", inst.Pid())
	go func() {
		<-inst.Done()
		m.s.dispatch(func() { m.exited(gen, inst) })
	}()
	if inst.QMP != nil {
		go func() {
			for ev := range inst.QMP.Events() {
				m.s.dispatch(func() { m.qmpEvent(gen, ev) })
			}
		}()
	}
	m.s.snaps.refresh()
	if !m.debugOn {
		m.setState("Running")
		return
	}
	m.paused = true
	m.setState("Paused at reset; attaching GDB…")
	m.attach(gen, g)
}

// attach starts GDB, connects it to QEMU's stub, stops at the first instruction of the
// program (0x7c00 for a boot sector, the entry point of an ELF kernel) and continues there.
func (m *machine) attach(gen int, g config.Gdb) {
	q := m.cfg.Qemu
	target := fmt.Sprintf("localhost:%d", q.Debug.GdbPort)
	symbols, stopAt := "", "*0x7c00"
	if q.Kernel != "" {
		k := filepath.Join(m.s.root, filepath.FromSlash(q.Kernel))
		if f, err := os.Open(k); err == nil {
			if e, err := inspect.ReadELF(f); err == nil {
				symbols, stopAt = k, fmt.Sprintf("*0x%x", e.Entry)
			} else {
				stopAt = ""
			}
			f.Close()
		}
	}
	out := m.gdbOut
	m.gdbOut.Clear()
	breaks := m.lineBreaks(m.s.currentBreakpoints())
	go func() {
		var set map[string]string
		var failed []string
		sess, err := gdb.Start(gdb.Options{
			Executable: g.Executable,
			OnConsole: func(text string) {
				for _, l := range strings.Split(strings.TrimRight(text, "\n"), "\n") {
					out.Add(l)
				}
			},
			OnExec: func(r gdb.Record) { m.s.dispatch(func() { m.execEvent(gen, r) }) },
		})
		release := func() {}
		if err == nil {
			var ok bool
			if release, ok = m.hold(sess.Close); !ok {
				return // the shell shut down meanwhile; hold closed GDB
			}
			ctx, cancel := context.WithTimeout(m.life, 20*time.Second)
			err = sess.Attach(ctx, g.Architecture, symbols, target)
			if err == nil && stopAt != "" {
				_, err = sess.Break(ctx, stopAt)
			}
			if err == nil {
				set, failed = setLineBreaks(ctx, sess, breaks)
				err = sess.Continue(ctx)
			}
			cancel()
			if err != nil {
				sess.Close()
			}
		}
		m.s.dispatch(func() { release(); m.attached(gen, sess, err, stopAt, set, failed) })
	}()
}

func (m *machine) attached(gen int, sess *gdb.Session, err error, stopAt string, set map[string]string, failed []string) {
	if gen != m.gen {
		if sess != nil && err == nil {
			go sess.Close()
		}
		return
	}
	if err != nil {
		m.gdbOut.AddNow("GDB could not attach: " + err.Error())
		m.setState("Running without GDB")
		m.s.Notify(notifications.Error, "GDB could not attach", err.Error())
		m.debugOn = false
		return
	}
	m.dbg = sess
	m.bpNums, m.bpDirty = set, false
	m.reportBreaks(set, failed)
	go func() {
		<-sess.Done()
		m.s.dispatch(func() { m.gdbGone(gen, sess) })
	}()
	m.s.logf("GDB attached; stopping at %s", stopAt)
	m.setState("Running to " + strings.TrimPrefix(stopAt, "*") + "…")
}

// gdbGone handles GDB exiting on its own (killed, crashed): QEMU keeps running without it.
// A GDB that PyxForge closed is no longer m.dbg, so its exit is not reported.
func (m *machine) gdbGone(gen int, sess *gdb.Session) {
	if gen != m.gen || m.dbg != sess {
		return
	}
	m.dbg, m.debugOn, m.paused = nil, false, false
	m.gdbOut.AddNow("==> GDB exited")
	m.setState("GDB exited; QEMU is still running")
	m.s.Notify(notifications.Error, "GDB exited", "QEMU keeps running. Debug in QEMU starts a new session.")
	m.views.running()
}

// execEvent handles GDB's *stopped and *running records.
func (m *machine) execEvent(gen int, r gdb.Record) {
	if gen != m.gen {
		return
	}
	switch r.Class {
	case "running":
		m.paused = false
		m.setState("Running")
		m.views.running()
	case "stopped":
		st := gdb.StopOf(r)
		if st.Reason == "exited" || st.Reason == "exited-normally" {
			return
		}
		m.paused, m.pc = true, st.Addr
		if m.bpDirty {
			m.applyBreakpoints()
		}
		reason := strings.ReplaceAll(st.Reason, "-", " ")
		if reason == "" {
			reason = "paused"
		}
		m.setState(fmt.Sprintf("Paused at 0x%x · %s", st.Addr, reason))
		m.views.refresh()
	}
}

// qmpEvent follows pauses and resets QEMU reports itself (a run without GDB, or the monitor).
func (m *machine) qmpEvent(gen int, ev qemu.Event) {
	if gen != m.gen || m.dbg != nil {
		return
	}
	switch ev.Name {
	case "STOP":
		m.paused = true
		m.setState("Paused")
	case "RESUME":
		m.paused = false
		m.setState("Running")
	case "RESET":
		m.qemuOut.AddNow("==> The machine was reset")
	}
}

func (m *machine) exited(gen int, inst *qemu.Instance) {
	if m.inst == inst {
		m.inst = nil
	}
	if gen != m.gen {
		return
	}
	m.closeGDB()
	m.paused, m.debugOn = false, false
	msg := "QEMU exited"
	if err := inst.Err(); err != nil {
		msg += ": " + err.Error()
	}
	m.qemuOut.AddNow("==> " + msg)
	m.setState(msg)
	m.views.running()
	m.s.logf("%s", msg)
}

func (m *machine) closeGDB() {
	if m.dbg != nil {
		d := m.dbg
		m.dbg = nil
		go d.Close()
	}
}

// halt stops QEMU and GDB.
func (m *machine) halt() {
	inst := m.inst
	m.gen++ // what the old session reports from now on is ignored
	m.inst, m.paused, m.debugOn, m.starting = nil, false, false, false
	m.closeGDB()
	if inst != nil {
		m.prev = inst
		go inst.Stop()
		m.qemuOut.AddNow("==> Stopped")
		m.s.logf("QEMU stopped")
	}
	m.setState("QEMU is not running")
	m.views.running()
	m.s.snaps.show() // machine states are listed only while QEMU runs
}

// shutdown stops everything before the window closes, waiting for QEMU to end.
func (m *machine) shutdown() {
	if m == nil {
		return
	}
	m.heldMu.Lock()
	m.down = true
	held := slices.Collect(maps.Values(m.held))
	clear(m.held)
	m.heldMu.Unlock()
	m.endLife()
	for _, stop := range held {
		stop()
	}
	inst, d := m.inst, m.dbg
	m.inst, m.dbg = nil, nil
	m.gen++
	if d != nil {
		d.Close()
	}
	if inst != nil {
		inst.Stop()
	}
}

func (m *machine) togglePause() {
	switch {
	case m.inst == nil:
	case m.dbg != nil && m.paused:
		m.gdbCall("continue", (*gdb.Session).Continue)
	case m.dbg != nil:
		m.gdbCall("pause", (*gdb.Session).Interrupt)
	case m.inst.QMP != nil:
		cmd := "stop"
		if m.paused {
			cmd = "cont"
		}
		q := m.inst.QMP
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := q.Execute(ctx, cmd, nil, nil); err != nil {
				m.s.dispatch(func() { m.s.Notify(notifications.Error, "QEMU did not "+cmd, err.Error()) })
			}
		}()
	}
}

func (m *machine) cont() {
	if m.dbg != nil && m.paused {
		m.gdbCall("continue", (*gdb.Session).Continue)
	} else if m.dbg == nil && m.paused {
		m.togglePause()
	}
}

func (m *machine) stepInstruction() {
	if m.dbg != nil && m.paused {
		m.gdbCall("step", (*gdb.Session).StepInstruction)
	}
}

func (m *machine) nextInstruction() {
	if m.dbg != nil && m.paused {
		m.gdbCall("step over", (*gdb.Session).NextInstruction)
	}
}

// gdbCall runs a GDB command off the UI thread; failures are shown, results arrive as events.
func (m *machine) gdbCall(what string, f func(*gdb.Session, context.Context) error) {
	d := m.dbg
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := f(d, ctx); err != nil {
			m.s.dispatch(func() { m.gdbOut.AddNow("GDB " + what + ": " + err.Error()) })
		}
	}()
}

func (m *machine) runMonitor(line string) {
	line = strings.TrimSpace(line)
	if line == "" {
		return
	}
	m.monitor.SetText("")
	if m.inst == nil || m.inst.QMP == nil {
		m.qemuOut.AddNow("QEMU is not running: Run or Debug starts it.")
		return
	}
	q := m.inst.QMP
	m.qemuOut.AddNow("(qemu) " + line)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		out, err := q.HMP(ctx, line)
		if err != nil {
			out = err.Error()
		}
		m.s.dispatch(func() {
			m.qemuOut.AddNow(strings.Split(strings.TrimRight(strings.ReplaceAll(out, "\r\n", "\n"), "\n"), "\n")...)
		})
	}()
}

func (m *machine) runGDB(line string) {
	line = strings.TrimSpace(line)
	if line == "" {
		return
	}
	m.gdbInput.SetText("")
	if m.dbg == nil {
		m.gdbOut.AddNow("No debug session: Debug (Ctrl+Shift+D) starts one.")
		return
	}
	d := m.dbg
	m.gdbOut.AddNow("(gdb) " + line)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		if _, err := d.Console(ctx, line); err != nil { // its output streams in through OnConsole
			m.s.dispatch(func() { m.gdbOut.AddNow(err.Error()) })
		}
		m.s.dispatch(func() {
			if m.dbg == d && m.paused {
				m.views.refresh() // the command may have changed registers or memory
			}
		})
	}()
}

// addBreakpoint asks for a location and sets a breakpoint there.
func (m *machine) addBreakpoint() {
	if m.dbg == nil {
		m.s.Notify(notifications.Info, "No debug session", "Debug (Ctrl+Shift+D) starts QEMU with GDB attached.")
		return
	}
	entry := m.s.newDialogEntry()
	entry.SetPlaceHolder("*0x7c10, a symbol, or file:line")
	d := dialog.NewForm("Add Breakpoint", "Add", "Cancel", []*widget.FormItem{widget.NewFormItem("Location", entry)},
		func(ok bool) {
			loc := strings.TrimSpace(entry.Text)
			if !ok || loc == "" || m.dbg == nil {
				return
			}
			dbg := m.dbg
			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				bp, err := dbg.Break(ctx, loc)
				m.s.dispatch(func() {
					if err != nil {
						m.s.Notify(notifications.Error, "No breakpoint", err.Error())
						return
					}
					m.gdbOut.AddNow(fmt.Sprintf("Breakpoint %s at %s", bp.Number, bp.Addr))
				})
			}()
		}, m.s.win)
	d.Resize(fyne.NewSize(380, d.MinSize().Height))
	m.s.submitForm(d, entry)
	m.s.win.Canvas().Focus(entry)
	m.s.lastDialog = d
}

// showSetup shows Set Up Machine… in the QEMU tab while the project has no machine and the tab
// has nothing else to show.
func (m *machine) showSetup() {
	ok, problem := machineSetUp(m.s.root)
	if ok || problem != nil || m.inst != nil || m.starting || len(m.qemuOut.Lines()) > 0 {
		m.setupView.Hide()
		m.monitor.Show()
	} else {
		m.setupView.Show()
		m.monitor.Hide() // no QEMU to send commands to
	}
}

// commandLines lays out a command line one option to a line, so a long one is read whole
// instead of being cut at the panel's edge: "==> qemu-system-i386", then "      -m 16M".
func commandLines(program string, args []string) []string {
	lines := []string{"==> " + program}
	for _, a := range args {
		if strings.HasPrefix(a, "-") || len(lines) == 1 {
			lines = append(lines, "      "+a)
			continue
		}
		lines[len(lines)-1] += " " + a
	}
	return lines
}

// lineBreak is an editor breakpoint as GDB is given it.
type lineBreak struct {
	key  string // "file:line", the file relative to the project
	file string // slash-separated, relative to the project: GDB matches the end of a source name
	line int
}

// lineBreaks are the editor's breakpoints in files of the project; others are no part of it.
func (m *machine) lineBreaks(list []neovim.Breakpoint) []lineBreak {
	var out []lineBreak
	for _, b := range list {
		rel, err := filepath.Rel(resolved(m.s.root), resolved(b.File)) // one spelling for both
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			continue
		}
		for _, l := range b.Lines {
			f := filepath.ToSlash(rel)
			out = append(out, lineBreak{key: fmt.Sprintf("%s:%d", f, l), file: f, line: l})
		}
	}
	return out
}

// setLineBreaks sets breakpoints in a stopped session and returns the numbers of those set and
// why the others were not.
func setLineBreaks(ctx context.Context, sess *gdb.Session, breaks []lineBreak) (map[string]string, []string) {
	set := map[string]string{}
	var failed []string
	for _, b := range breaks {
		bp, err := sess.BreakAt(ctx, b.file, b.line)
		if err != nil {
			failed = append(failed, b.key+": "+err.Error())
			continue
		}
		set[b.key] = bp.Number
	}
	return set, failed
}

// reportBreaks says in the GDB tab which editor breakpoints are set and which are not: a line
// of a program built without debug information (a flat boot sector) has no address GDB knows.
func (m *machine) reportBreaks(set map[string]string, failed []string) {
	keys := slices.Sorted(maps.Keys(set))
	for _, k := range keys {
		m.gdbOut.AddNow(fmt.Sprintf("Breakpoint %s at %s (from the editor)", set[k], k))
	}
	for _, f := range failed {
		m.gdbOut.AddNow("Breakpoint not set at " + f)
	}
	if len(failed) > 0 {
		m.s.logf("Editor breakpoints not set in GDB: %s", strings.Join(failed, "; "))
	}
}

// syncBreakpoints passes a change of the editor's breakpoints to the running session.
func (m *machine) syncBreakpoints(list []neovim.Breakpoint) {
	if m == nil || m.dbg == nil {
		return
	}
	m.bpWant = list
	if !m.paused {
		m.bpDirty = true
		return
	}
	m.applyBreakpoints()
}

// applyBreakpoints makes the session's editor breakpoints those of bpWant: it deletes the ones
// cleared and sets the new ones, with the machine stopped.
func (m *machine) applyBreakpoints() {
	m.bpDirty = false
	dbg := m.dbg
	want := m.lineBreaks(m.bpWant)
	wanted := map[string]bool{}
	var add []lineBreak
	for _, b := range want {
		wanted[b.key] = true
		if _, ok := m.bpNums[b.key]; !ok {
			add = append(add, b)
		}
	}
	var del []string
	for k, n := range m.bpNums {
		if !wanted[k] {
			del = append(del, n)
			delete(m.bpNums, k)
		}
	}
	if len(add) == 0 && len(del) == 0 {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		for _, n := range del {
			_ = dbg.Delete(ctx, n)
		}
		set, failed := setLineBreaks(ctx, dbg, add)
		m.s.dispatch(func() {
			if m.dbg != dbg {
				return
			}
			if m.bpNums == nil {
				m.bpNums = map[string]string{}
			}
			maps.Copy(m.bpNums, set)
			m.reportBreaks(set, failed)
		})
	}()
}
