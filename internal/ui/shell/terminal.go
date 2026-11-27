package shell

import (
	"fmt"
	"os"
	"slices"
	"strings"
	"sync/atomic"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"github.com/obstinix/PyxForge/internal/pty"
	"github.com/obstinix/PyxForge/internal/shellint"
	"github.com/obstinix/PyxForge/internal/term"
	"github.com/obstinix/PyxForge/internal/ui/commandpalette"
	"github.com/obstinix/PyxForge/internal/ui/icons"
	"github.com/obstinix/PyxForge/internal/ui/kit"
	"github.com/obstinix/PyxForge/internal/ui/notifications"
	"github.com/obstinix/PyxForge/internal/ui/termview"
	"github.com/obstinix/PyxForge/internal/vt"
)

// terminalHost runs the Terminal panel (D17, D24): each session is a program on a
// pseudo-terminal of its own (internal/term: ConPTY on Windows, a Unix pty elsewhere), the
// user's shell or a program Run Current File started, or the machine's QEMU serial port or
// GDB console. The session rail on the right lists them; one or two panes, side by side, show
// sessions; the others keep running while hidden, their output kept. Shells PyxForge can
// integrate (shellint) mark their commands, which the panes draw as blocks.
type terminalHost struct {
	s        *Shell
	tab      *container.TabItem
	body     *fyne.Container // the panes while a session shows; otherwise a message
	frame    fyne.CanvasObject
	rail     *sessionRail
	find     *termSearch
	panes    []*termPane
	active   int            // the pane keys and the rail act on
	view     *termview.View // the active pane's view
	current  *termSession   // the active pane's session
	sessions []*termSession
	next     int // the number for the next shell's default name
	split    float32
}

// sessionKind is what a session runs.
type sessionKind int

const (
	kindShell sessionKind = iota
	kindRun
	kindDebug
	kindSerial // the machine's serial port (QEMU -serial stdio)
	kindGDB    // the debugger's console
)

func (k sessionKind) icon() icons.Name {
	switch k {
	case kindRun:
		return icons.Play
	case kindDebug:
		return icons.Bug
	case kindSerial:
		return icons.Server
	case kindGDB:
		return icons.Binary
	}
	return icons.Terminal
}

func (k sessionKind) label() string {
	return [...]string{"Shell", "Run", "Debug", "QEMU serial", "GDB"}[k]
}

// termSession is one shell, a program Run Current File started, or the machine's serial port
// or GDB console.
type termSession struct {
	name   string
	kind   sessionKind
	t      *term.Session // a program on a pseudo-terminal; nil for serial and GDB
	screen *vt.Screen    // what it shows: t.Screen, or the serial or GDB console's own
	done   bool          // the program exited
	status int           // its exit status, once done
	start  time.Time
	end    time.Time
	argv   []string // the program, for a session that runs one instead of the shell
	dir    string   // and its folder

	// Serial and GDB sessions are read only until Send input is on; send then takes what
	// is typed.
	send      func([]byte)
	sendInput bool

	shell      *pty.Shell // the shell, when not the one Settings chose
	blocks     bool       // the shell loaded PyxForge's integration: its commands are blocks
	redraw     atomic.Bool
	plainShell bool // a shell that cannot be integrated (cmd.exe) while blocks are on
}

// Screen is what the session shows.
func (ts *termSession) Screen() *vt.Screen {
	if ts.t != nil {
		return ts.t.Screen
	}
	return ts.screen
}

func (ts *termSession) running() bool { return !ts.done }

// write types into the session's program, when it takes input.
func (ts *termSession) write(b []byte) {
	switch {
	case ts.t != nil && !ts.done:
		_ = ts.t.Write(b)
	case ts.send != nil && ts.sendInput:
		ts.send(b)
	}
}

// termPane is one of the panel's (at most two) side-by-side views and the session it shows.
type termPane struct {
	h       *terminalHost
	view    *termview.View
	session *termSession
	box     *fyne.Container // a line about the session (exited, read only) above the view
}

// prefTerminalBlocks turns command blocks on and off (Settings > Terminal > Command blocks).
const prefTerminalBlocks = "terminal.blocks"

func (s *Shell) blocksOn() bool {
	return s.app.Preferences().BoolWithFallback(prefTerminalBlocks, true)
}

func (s *Shell) buildTerminal() *container.TabItem {
	t := &terminalHost{s: s, next: 1, split: 0.5}
	s.term = t
	t.rail = newSessionRail(t)
	t.find = newTermSearch(t)
	t.body = container.NewStack(placeholderAction(icons.Terminal, "No terminal session",
		"A shell starts in your home folder when no project is open, otherwise in the project folder.", "Open Terminal", t.newSession))
	t.addPane()
	t.frame = container.New(&terminalLayout{t: t}, container.NewBorder(t.find.box, nil, nil, nil, t.body), kit.NewRule(true), t.rail.object())
	t.tab = container.NewTabItem("Terminal", t.frame)
	return t.tab
}

// addPane makes a pane with its own view.
func (t *terminalHost) addPane() *termPane {
	s := t.s
	p := &termPane{h: t, view: termview.New()}
	v := p.view
	v.SetTextSize(s.textSize(prefTerminalTextSize))
	v.ShellTakes = s.takesFromTerminal
	v.OnShortcut = func(sc fyne.Shortcut) { s.runChord(sc) }
	v.Dispatch = s.dispatch
	v.OnFocus = func(on bool) {
		if on {
			t.activate(p)
			s.activate(regionPanel)
		}
	}
	v.Input = func(b []byte) {
		if p.session != nil {
			p.session.write(b)
		}
	}
	v.OnResize = func(cols, rows int) {
		switch {
		case p.session == nil:
		case p.session.t != nil:
			p.session.t.Resize(cols, rows)
		default: // a serial or GDB console: its screen takes the pane's size
			p.session.screen.Resize(cols, rows)
		}
	}
	v.ResolveLink = s.resolveTerminalLink
	v.OnOpenLink = func(path string, line, col int) { s.openAt(path, max(line-1, 0), max(col-1, 0)) }
	v.OnSendToProblems = func(b term.Block, out string) { s.sendBlockToProblems(b, out) }
	p.box = container.NewBorder(nil, nil, nil, nil, v)
	t.panes = append(t.panes, p)
	if len(t.panes) == 1 {
		t.view = v
	}
	return p
}

// activate makes a pane the one keys and the rail act on.
func (t *terminalHost) activate(p *termPane) {
	i := slices.Index(t.panes, p)
	if i < 0 || i == t.active && t.view == p.view {
		return
	}
	t.active, t.view, t.current = i, p.view, p.session
	t.rail.refresh()
}

func (t *terminalHost) pane() *termPane { return t.panes[t.active] }

// placeholderAction is a placeholder with one button under its text.
func placeholderAction(icon icons.Name, title, detail, label string, run func()) fyne.CanvasObject {
	p := placeholder(icon, title, detail).(*fyne.Container)
	b := kit.NewButton(label, run)
	b.Variant = kit.ButtonCTA
	p.Objects[0].(*fyne.Container).Add(container.NewCenter(b))
	return p
}

func (t *terminalHost) show(o fyne.CanvasObject) {
	t.body.Objects = []fyne.CanvasObject{o}
	t.body.Refresh()
}

// start opens the first session when the panel is first shown. Sessions that exited are left
// for the user to restart. A shell made without the editor (tests) starts no programs.
func (t *terminalHost) start() {
	if len(t.sessions) > 0 || !t.s.editorEnabled {
		return
	}
	t.newSession()
}

// launch starts a program in a terminal the view's size: the user's shell in the workspace
// root (the home folder without one) when argv is nil, with PyxForge's shell integration when
// command blocks are on and the shell can have it.
func (t *terminalHost) launch(ts *termSession) error {
	argv, dir := ts.argv, ts.dir
	var env []string
	ts.blocks, ts.plainShell = false, false
	if argv == nil {
		sh := t.s.terminalShell()
		if ts.shell != nil {
			sh = *ts.shell
		}
		argv, dir = sh.Argv, t.s.processFolder()
		if t.s.blocksOn() {
			if l, err := shellint.Prepare(sh, shellint.Dir()); err == nil {
				argv, env, ts.blocks = l.Argv, append(os.Environ(), l.Env...), true
			} else if err == shellint.ErrUnsupported {
				ts.plainShell = true
			} else {
				t.s.logf("Terminal: no command blocks: %v", err)
			}
		}
	}
	cols, rows := t.view.GridSize()
	if cols < 2 {
		cols, rows = 100, 12
	}
	var self *term.Session
	sess, err := term.Start(pty.Options{Argv: argv, Dir: dir, Env: env, Cols: cols, Rows: rows}, term.Hooks{
		// From the session's read goroutine: the panes are the UI thread's, so redrawing the
		// ones showing it waits for that thread, once however much output arrives meanwhile.
		Changed: func() {
			if ts.redraw.CompareAndSwap(false, true) {
				t.s.dispatch(func() {
					ts.redraw.Store(false)
					for _, p := range t.panes {
						if p.session == ts {
							p.view.Refresh()
						}
					}
				})
			}
		},
		Exited: func(status int) {
			t.s.dispatch(func() { t.closed(ts, self, status) })
		},
	})
	if err != nil {
		return err
	}
	self = sess
	ts.t, ts.done, ts.status, ts.start, ts.end = sess, false, 0, time.Now(), time.Time{}
	return nil
}

// newSession starts another shell and shows it in the active pane.
func (t *terminalHost) newSession() {
	if !t.s.editorEnabled {
		return
	}
	ts := &termSession{name: fmt.Sprintf("Shell %d", t.next), kind: kindShell}
	if err := t.launch(ts); err != nil {
		t.show(placeholderAction(icons.Terminal, "The shell did not start", err.Error(), "Try Again", t.newSession))
		t.s.logf("Terminal: the shell did not start: %v", err)
		return
	}
	t.next++
	t.sessions = append(t.sessions, ts)
	t.showSession(ts)
	t.s.logf("Terminal: %s started in %s", ts.name, t.s.processFolder())
}

// run starts a program in a session of its own and shows it, so it can read what is typed;
// Restart runs it again. The session is named after it.
// typeAt and typed, when set, are lines typed into the program at its prompt (Plan.Type).
func (t *terminalHost) run(name string, argv []string, dir, typeAfter, typeAt string, typed []string) {
	kind := kindRun
	if strings.HasPrefix(name, "Debug ") {
		kind = kindDebug
	}
	ts := &termSession{name: t.freeName(name), kind: kind, argv: argv, dir: dir}
	if err := t.launch(ts); err != nil {
		t.s.Notify(notifications.Error, "Cannot run "+name, err.Error())
		return
	}
	t.sessions = append(t.sessions, ts)
	t.s.showDockItem(t.tab)
	t.showSession(ts)
	t.s.logf("Terminal: %s: %s in %s", ts.name, strings.Join(argv, " "), dir)
	if len(typed) > 0 {
		sess := ts.t
		go func() {
			if n := sess.TypeAtPrompt(typeAfter, typeAt, typed, 30*time.Second); n < len(typed) {
				t.s.dispatch(func() {
					t.s.logf("Terminal: %s: typed %d of %d commands; the rest: %s", ts.name, n, len(typed), strings.Join(typed[n:], "; "))
				})
			}
		}()
	}
}

// console is a serial or GDB session: a screen PyxForge writes lines into, read only until
// Send input is on, when what is typed goes to send. It is made, or found, by name.
func (t *terminalHost) console(kind sessionKind, name string, send func([]byte)) *termSession {
	for _, ts := range t.sessions {
		if ts.kind == kind {
			ts.send, ts.done, ts.end = send, false, time.Time{}
			return ts
		}
	}
	ts := &termSession{name: t.freeName(name), kind: kind, screen: vt.New(100, 12), send: send, start: time.Now()}
	t.sessions = append(t.sessions, ts)
	t.rail.refresh()
	return ts
}

// consoleLine adds lines to a serial or GDB session.
func (t *terminalHost) consoleLine(ts *termSession, lines ...string) {
	for _, l := range lines {
		ts.screen.Write([]byte(strings.ReplaceAll(l, "\n", "\r\n") + "\r\n"))
	}
	for _, p := range t.panes {
		if p.session == ts {
			p.view.Changed()
		}
	}
}

// consoleEnded marks a serial or GDB session as over (QEMU or GDB exited).
func (t *terminalHost) consoleEnded(ts *termSession, status int) {
	if ts == nil || ts.done {
		return
	}
	ts.done, ts.status, ts.end, ts.send = true, status, time.Now(), nil
	t.sync()
}

// showSession shows ts in the active pane. When the other pane shows it, the two panes swap
// sessions, so a session is never on view twice.
func (t *terminalHost) showSession(ts *termSession) {
	p := t.pane()
	for i, o := range t.panes {
		if i != t.active && o.session == ts {
			o.session = p.session
			if o.session != nil {
				o.view.BlocksOn = o.session.blocks
				o.view.SetScreen(o.session.Screen())
			}
			break
		}
	}
	p.session, t.current = ts, ts
	p.view.BlocksOn = ts.blocks
	p.view.SetScreen(ts.Screen())
	t.sync()
	t.focus()
}

// freeName is name, or name with a number after it, so no two sessions share a name.
func (t *terminalHost) freeName(name string) string {
	taken := func(n string) bool {
		return slices.ContainsFunc(t.sessions, func(o *termSession) bool { return o.name == n })
	}
	for i, n := 2, name; ; i++ {
		if !taken(n) {
			return n
		}
		n = fmt.Sprintf("%s (%d)", name, i)
	}
}

// selectSession shows a session, or its exit status if it ended.
func (t *terminalHost) selectSession(ts *termSession) {
	if ts == nil {
		return
	}
	for _, p := range t.panes { // already on view in a pane: make that pane active
		if p.session == ts {
			t.activate(p)
			t.sync()
			t.focus()
			return
		}
	}
	t.showSession(ts)
}

// cycle shows the next (1) or previous (-1) session in the active pane.
func (t *terminalHost) cycle(step int) {
	n := len(t.sessions)
	if n < 2 {
		return
	}
	i := slices.Index(t.sessions, t.current)
	t.showSession(t.sessions[((i+step)%n+n)%n])
}

// restart replaces the active session's program with a new one, keeping its name: the shell,
// or the program it ran, again. Serial and GDB sessions restart with the machine.
func (t *terminalHost) restart() { t.restartSession(t.current) }

func (t *terminalHost) restartSession(ts *termSession) {
	if ts == nil {
		t.newSession()
		return
	}
	if ts.kind == kindSerial || ts.kind == kindGDB {
		t.s.Notify(notifications.Info, ts.name+" restarts with the machine", "Run or Debug starts QEMU again.")
		return
	}
	old := ts.t
	if err := t.launch(ts); err != nil {
		if ts.argv != nil {
			t.s.Notify(notifications.Error, "Cannot run "+ts.name, err.Error())
		} else {
			t.show(placeholderAction(icons.Terminal, "The shell did not start", err.Error(), "Try Again", t.restart))
		}
		return
	}
	if old != nil {
		old.Close() // ends the old program if it still runs
	}
	for _, p := range t.panes {
		if p.session == ts {
			p.view.BlocksOn = ts.blocks
			p.view.SetScreen(ts.Screen())
		}
	}
	t.selectSession(ts)
}

// clear empties the active session's scrollback and has its shell clear the screen.
func (t *terminalHost) clear() {
	if ts := t.current; ts != nil && ts.t != nil {
		ts.t.Clear()
		t.view.SetScreen(ts.t.Screen)
	}
	t.focus()
}

// closeCurrent ends the active session's program and removes it.
func (t *terminalHost) closeCurrent() { t.closeSession(t.current) }

// closeSession ends a session's program and removes it; a pane that showed it shows another,
// and a second pane left with nothing to show closes.
func (t *terminalHost) closeSession(ts *termSession) {
	if ts == nil {
		return
	}
	i := slices.Index(t.sessions, ts)
	if i < 0 {
		return
	}
	t.sessions = slices.Delete(t.sessions, i, i+1)
	if ts.t != nil {
		ts.t.Close()
	}
	for pi := len(t.panes) - 1; pi >= 0; pi-- {
		p := t.panes[pi]
		if p.session != ts {
			continue
		}
		p.session = nil
		if pi > 0 {
			t.closePane(pi)
			continue
		}
		if other := t.unshown(); other != nil {
			p.session = other
			p.view.BlocksOn = other.blocks
			p.view.SetScreen(other.Screen())
		} else if len(t.panes) > 1 { // the second pane's session moves over
			q := t.panes[1]
			p.session = q.session
			p.view.BlocksOn = q.view.BlocksOn
			p.view.SetScreen(q.session.Screen())
			t.closePane(1)
		} else {
			p.view.SetScreen(nil)
		}
	}
	t.active = min(t.active, len(t.panes)-1)
	t.view, t.current = t.pane().view, t.pane().session
	t.s.logf("Terminal: %s closed", ts.name)
	t.sync()
	t.focus()
}

// closeExited removes every session whose program ended.
func (t *terminalHost) closeExited() {
	for _, ts := range slices.Clone(t.sessions) {
		if ts.done {
			t.closeSession(ts)
		}
	}
}

// unshown is a session no pane shows, the most recent first.
func (t *terminalHost) unshown() *termSession {
	for i := len(t.sessions) - 1; i >= 0; i-- {
		ts := t.sessions[i]
		if !slices.ContainsFunc(t.panes, func(p *termPane) bool { return p.session == ts }) {
			return ts
		}
	}
	return nil
}

// splitRight shows a second pane to the right of the first: the given session, another one
// no pane shows, or a new shell. There are at most two.
func (t *terminalHost) splitRight(ts *termSession) {
	if len(t.panes) >= 2 {
		t.s.Notify(notifications.Info, "The terminal is split already", "It shows two sessions at most; close one first.")
		return
	}
	if len(t.sessions) == 0 {
		t.newSession()
	}
	p := t.addPane()
	t.active, t.view = len(t.panes)-1, p.view
	switch {
	case ts != nil && ts != t.panes[0].session:
		t.showSession(ts)
	case t.unshown() != nil:
		t.showSession(t.unshown())
	default:
		t.newSession()
	}
	if p.session == nil { // the shell did not start
		t.closePane(1)
	}
	t.sync()
}

// closePane removes the second pane; its session keeps running, listed in the rail.
func (t *terminalHost) closePane(i int) {
	if i <= 0 || i >= len(t.panes) {
		return
	}
	t.panes = slices.Delete(t.panes, i, i+1)
	t.active = min(t.active, len(t.panes)-1)
	t.view, t.current = t.pane().view, t.pane().session
	t.sync()
}

// unsplit closes the second pane.
func (t *terminalHost) unsplit() { t.closePane(1) }

// renameCurrent asks for a new name for the active session.
func (t *terminalHost) renameCurrent() { t.renameSession(t.current) }

func (t *terminalHost) renameSession(ts *termSession) {
	if ts == nil {
		return
	}
	entry := t.s.newDialogEntry()
	entry.SetText(ts.name)
	d := escForm("Rename Terminal", "Rename", "Cancel", []*widget.FormItem{widget.NewFormItem("Name", entry)},
		func(ok bool) {
			if ok {
				t.rename(ts, entry.Text)
			}
		}, t.s.win)
	d.Resize(fyne.NewSize(360, d.MinSize().Height))
	t.s.submitForm(d, entry)
	t.s.win.Canvas().Focus(entry)
	t.s.lastDialog = d
}

// rename gives a session a name no other session has.
func (t *terminalHost) rename(ts *termSession, name string) {
	name = strings.TrimSpace(name)
	if name == "" || name == ts.name {
		return
	}
	for _, o := range t.sessions {
		if o != ts && o.name == name {
			t.s.Notify(notifications.Info, "Name in use", "Another terminal is called "+name+".")
			return
		}
	}
	ts.name = name
	t.sync()
}

// chooseSession lists the sessions in the palette.
func (t *terminalHost) chooseSession() {
	if len(t.sessions) == 0 {
		t.s.showDockItem(t.tab)
		return
	}
	var items []commandpalette.Item
	for _, ts := range t.sessions {
		detail := ts.kind.label() + ", " + sessionState(ts)
		if ts == t.current {
			detail += ", current"
		}
		items = append(items, commandpalette.Item{Title: ts.name, Detail: detail, Icon: ts.kind.icon(), Run: func() {
			t.s.showDockItem(t.tab)
			t.selectSession(ts)
		}})
	}
	t.s.palette.Show("Switch to a terminal", items)
}

// sessionState says how a session is: running, or how it ended and after how long.
func sessionState(ts *termSession) string {
	if !ts.done {
		return "running"
	}
	s := fmt.Sprintf("exit %d", ts.status)
	if !ts.start.IsZero() && !ts.end.IsZero() {
		s += " · " + termview.FormatDuration(ts.end.Sub(ts.start))
	}
	return s
}

// sync shows the panes, each its session with a line above it when there is something to say
// (it ended; it is read only; its shell has no command blocks), and the rail.
func (t *terminalHost) sync() {
	t.rail.refresh()
	if t.current == nil && len(t.sessions) == 0 {
		t.show(placeholderAction(icons.Terminal, "No terminal session", "New Terminal starts a shell in "+t.s.root+".",
			"New Terminal", t.newSession))
		return
	}
	var shown []fyne.CanvasObject
	for i, p := range t.panes {
		ts := p.session
		p.view.SetCursorShown(ts != nil && ts.running() && (ts.t != nil || ts.sendInput))
		var top fyne.CanvasObject
		switch {
		case ts == nil:
		case ts.done:
			// The output stays, under a line saying how it ended.
			top = container.NewPadded(exitStrip(exitDescription(ts.who(), ts.status)+".", ts.status != 0, func() { t.restartSession(ts) }))
		case ts.kind == kindSerial || ts.kind == kindGDB:
			top = container.NewPadded(t.inputStrip(ts))
		case ts.plainShell && !t.s.noBlocksNoted:
			top = container.NewPadded(t.noBlocksStrip())
		}
		p.box.Objects = []fyne.CanvasObject{p.view}
		if top != nil {
			p.box.Objects = []fyne.CanvasObject{p.view, top}
			p.box.Layout = layoutBorderTop(top)
		} else {
			p.box.Layout = layoutBorderTop(nil)
		}
		p.box.Refresh()
		if i > 0 {
			shown = append(shown, kit.NewSash(true, func(dx float32) { t.dragSplit(dx) }))
		}
		shown = append(shown, p.box)
	}
	if len(t.panes) == 1 {
		t.show(t.panes[0].box)
	} else {
		t.show(container.New(&splitLayout{t: t}, shown...))
	}
}

// dragSplit moves the line between the two panes.
func (t *terminalHost) dragSplit(dx float32) {
	if w := t.body.Size().Width; w > 0 {
		t.split = min(max(t.split+dx/w, 0.2), 0.8)
		t.body.Refresh()
	}
}

// inputStrip is the line above a serial or GDB session: read only, or sending what is typed.
func (t *terminalHost) inputStrip(ts *termSession) fyne.CanvasObject {
	note := kit.NewText("Read only: Send input types into "+ts.kind.label()+".", kit.Body, kit.Secondary)
	if ts.sendInput {
		note.SetText("What you type goes to " + ts.kind.label() + ".")
	}
	sw := kit.NewSwitch(ts.sendInput, func(on bool) {
		ts.sendInput = on
		t.sync()
		t.focus()
	})
	sw.Label = "Send input"
	return container.NewBorder(nil, kit.NewRule(false), kit.NewIcon(ts.kind.icon(), kit.Secondary),
		container.NewHBox(kit.NewText("Send input", kit.Body, kit.Secondary), sw), note)
}

// noBlocksStrip is the one line saying why a cmd.exe session has no command blocks.
func (t *terminalHost) noBlocksStrip() fyne.CanvasObject {
	note := kit.NewText("Command blocks need PowerShell, bash, zsh or fish.", kit.Body, kit.Secondary)
	open := kit.NewButton("Terminal settings", func() { t.s.openSettingsAt("terminal") })
	open.Variant = kit.ButtonGhost
	dismiss := kit.NewIconButton(icons.X, "Dismiss", func() { t.s.noBlocksNoted = true; t.sync() })
	dismiss.Side = 24
	return container.NewBorder(nil, kit.NewRule(false), kit.NewIcon(icons.Info, kit.Secondary),
		container.NewHBox(open, dismiss), note)
}

// exitStrip is the line above an ended session's output: how it ended, and Restart. The note
// wraps between the icon and the button, so a long status never pushes Restart out of a narrow
// panel (PXF-DEF-006).
func exitStrip(text string, failed bool, restart func()) fyne.CanvasObject {
	role := kit.Secondary
	if failed {
		role = kit.Warning
	}
	note := widget.NewLabel(text)
	note.Wrapping = fyne.TextWrapWord
	return container.NewBorder(nil, kit.NewRule(false), kit.NewIcon(icons.Terminal, role),
		container.NewVBox(kit.NewButton("Restart", restart)), note)
}

// closed records a program's exit; its output stays on view under a line saying how it ended
// (PXF-DEF-015). The exit of a program a restart or a close replaced is not the session's.
func (t *terminalHost) closed(ts *termSession, sess *term.Session, status int) {
	if ts.t != sess || ts.done || !slices.Contains(t.sessions, ts) {
		return
	}
	ts.done, ts.status, ts.end = true, status, time.Now()
	t.s.logf("Terminal: %s: %s", ts.name, exitDescription(ts.who(), status))
	t.sync()
}

// exitDescription says how a session's shell or program (who) ended, naming the Windows
// statuses people meet.
func exitDescription(who string, status int) string {
	switch {
	case status == 0:
		return who + " exited"
	case status == -1:
		return who + " ended; its exit status is not known"
	case uint32(status) == 0xC000013A:
		return who + " was ended by Ctrl+C, Ctrl+Break or its console closing (status 0xC000013A)"
	case uint32(status) == 0xC0000005:
		return who + " crashed: access violation (status 0xC0000005)"
	case uint32(status) == 0xC00000FD:
		return who + " crashed: stack overflow (status 0xC00000FD)"
	case status < 0:
		return fmt.Sprintf("%s ended with status %d (0x%08X)", who, status, uint32(status))
	}
	return fmt.Sprintf("%s exited with status %d", who, status)
}

// who names what a session runs, for its exit: "The shell", or the program's file.
func (ts *termSession) who() string {
	switch ts.kind {
	case kindSerial:
		return "QEMU"
	case kindGDB:
		return "GDB"
	}
	if file, ok := strings.CutPrefix(ts.name, "Debug "); ok && ts.argv != nil {
		return "The debugger for " + file
	}
	if ts.argv != nil {
		return strings.TrimPrefix(ts.name, "Run ")
	}
	return "The shell"
}

// focus gives the active pane keyboard focus when its session takes input.
func (t *terminalHost) focus() {
	ts := t.current
	if ts == nil || ts.done {
		return
	}
	if c := t.s.win.Canvas(); c != nil {
		c.Focus(t.view)
	}
}

// syncTheme redraws the terminal in the current theme: its colours, ANSI ones included, are
// the theme's, read as it draws.
func (t *terminalHost) syncTheme() {
	if t == nil {
		return
	}
	for _, p := range t.panes {
		p.view.Refresh()
	}
	t.rail.refresh()
}

// setTextSize sets every pane's text size.
func (t *terminalHost) setTextSize(size float32) {
	for _, p := range t.panes {
		p.view.SetTextSize(size)
	}
}

// stop ends every session's program and waits, at most stopWait, for them to go: they hold
// the workspace open as their folder.
func (t *terminalHost) stop() {
	if t == nil {
		return
	}
	for _, ts := range t.sessions {
		if ts.t != nil {
			ts.t.Close()
		}
	}
	limit := time.After(stopWait)
	for _, ts := range t.sessions {
		if ts.t == nil {
			continue
		}
		select {
		case <-ts.t.Ended():
		case <-limit:
			return
		}
	}
}

// stopWait is how long stop waits for the terminals' programs: Close kills one that is still
// running 2 s after its terminal closed.
const stopWait = 3 * time.Second

// prefTerminalShell is the program new terminal sessions run (Settings > Terminal > Shell, D24):
// its path, or "" for the preferred shell found on this machine.
const prefTerminalShell = "terminal.shell"

// terminalShell is the shell new sessions start: the one chosen in Settings, or the preferred
// one (pwsh, then Windows PowerShell, then cmd.exe on Windows; $SHELL elsewhere).
func (s *Shell) terminalShell() pty.Shell {
	return pty.ShellFor(s.app.Preferences().String(prefTerminalShell), pty.Shells())
}

// Layouts

// terminalLayout puts the rail on the right: 180 px, or 40 px of icons when the panel is
// narrower than 700 px.
type terminalLayout struct{ t *terminalHost }

const (
	railWide, railNarrow = 180, 40
	railBreakpoint       = 700
)

func (l *terminalLayout) Layout(objs []fyne.CanvasObject, s fyne.Size) {
	w := float32(railWide)
	narrow := s.Width < railBreakpoint
	if narrow {
		w = railNarrow
	}
	l.t.rail.setNarrow(narrow)
	objs[0].Move(fyne.NewPos(0, 0))
	objs[0].Resize(fyne.NewSize(s.Width-w-1, s.Height))
	objs[1].Move(fyne.NewPos(s.Width-w-1, 0))
	objs[1].Resize(fyne.NewSize(1, s.Height))
	objs[2].Move(fyne.NewPos(s.Width-w, 0))
	objs[2].Resize(fyne.NewSize(w, s.Height))
}

func (l *terminalLayout) MinSize(objs []fyne.CanvasObject) fyne.Size {
	return fyne.NewSize(railNarrow+120, max(objs[0].MinSize().Height, objs[2].MinSize().Height))
}

// splitLayout puts two panes side by side with a sash between them.
type splitLayout struct{ t *terminalHost }

func (l *splitLayout) Layout(objs []fyne.CanvasObject, s fyne.Size) {
	left := float32(int(s.Width * l.t.split))
	objs[0].Move(fyne.NewPos(0, 0))
	objs[0].Resize(fyne.NewSize(left, s.Height))
	objs[1].Move(fyne.NewPos(left, 0))
	objs[1].Resize(fyne.NewSize(1, s.Height))
	objs[2].Move(fyne.NewPos(left+1, 0))
	objs[2].Resize(fyne.NewSize(s.Width-left-1, s.Height))
}

func (l *splitLayout) MinSize(objs []fyne.CanvasObject) fyne.Size {
	return fyne.NewSize(objs[0].MinSize().Width+objs[2].MinSize().Width+1, objs[0].MinSize().Height)
}

// borderTop lays out a pane: its line (if any) on top, the view under it.
type borderTop struct{ top fyne.CanvasObject }

func layoutBorderTop(top fyne.CanvasObject) fyne.Layout { return &borderTop{top: top} }

func (l *borderTop) Layout(objs []fyne.CanvasObject, s fyne.Size) {
	y := float32(0)
	if l.top != nil {
		h := l.top.MinSize().Height
		l.top.Move(fyne.NewPos(0, 0))
		l.top.Resize(fyne.NewSize(s.Width, h))
		y = h
	}
	objs[0].Move(fyne.NewPos(0, y))
	objs[0].Resize(fyne.NewSize(s.Width, s.Height-y))
}

func (l *borderTop) MinSize(objs []fyne.CanvasObject) fyne.Size {
	ms := objs[0].MinSize()
	if l.top != nil {
		ms.Height += l.top.MinSize().Height
		ms.Width = max(ms.Width, l.top.MinSize().Width)
	}
	return ms
}
