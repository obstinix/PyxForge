package shell

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"github.com/obstinix/PyxForge/internal/pty"
	"github.com/obstinix/PyxForge/internal/term"
	"github.com/obstinix/PyxForge/internal/ui/commandpalette"
	"github.com/obstinix/PyxForge/internal/ui/icons"
	"github.com/obstinix/PyxForge/internal/ui/kit"
	"github.com/obstinix/PyxForge/internal/ui/notifications"
	"github.com/obstinix/PyxForge/internal/ui/termview"
	"github.com/obstinix/PyxForge/internal/ui/theme"
)

// terminalHost runs the Terminal panel (D17): each session is a program on a pseudo-terminal
// of its own (internal/term: ConPTY on Windows, a Unix pty elsewhere), the user's shell in the
// workspace root or a program Run Current File started. One view draws whichever session is
// selected; the others keep running while hidden, their output kept.
type terminalHost struct {
	s        *Shell
	tab      *container.TabItem
	body     *fyne.Container // the view while a session shows; otherwise a message
	frame    fyne.CanvasObject
	bar      *fyne.Container
	pick     *sessionPicker
	view     *termview.View
	sessions []*termSession
	current  *termSession
	next     int // the number for the next session's default name
}

// termSession is one shell, or one program Run Current File started.
type termSession struct {
	name   string
	t      *term.Session
	done   bool     // the program exited
	status int      // its exit status, once done
	argv   []string // the program, for a session that runs one instead of the shell
	dir    string   // and its folder
}

func (s *Shell) buildTerminal() *container.TabItem {
	t := &terminalHost{s: s, next: 1}
	s.term = t
	t.pick = newSessionPicker(func(name string) { t.selectByName(name) })
	t.pick.PlaceHolder = "No terminal"
	newBtn := kit.NewIconButton(icons.Plus, "New Terminal (Ctrl+Shift+`)", t.newSession)
	rename := kit.NewIconButton(icons.FileText, "Rename Terminal", t.renameCurrent)
	restart := kit.NewIconButton(icons.Refresh, "Restart Terminal", t.restart)
	closeBtn := kit.NewIconButton(icons.X, "Close Terminal", t.closeCurrent)
	// The picker takes the bar's free width, so "Run hello.c (exited)" or "Debug count.c" is
	// read whole at the default layout (#28); it still shrinks to its placeholder's width in a
	// narrow panel, where hovering it shows the whole name.
	t.bar = container.NewBorder(nil, nil, nil, container.NewHBox(newBtn, rename, restart, closeBtn), t.pick)
	top := container.NewVBox(container.New(layout.NewCustomPaddedLayout(0, 0, theme.Space2, theme.Space1),
		kit.Row(theme.TabBarHeight, t.bar)), kit.NewRule(false))
	t.body = container.NewStack(placeholderAction(icons.Terminal, "No terminal session",
		"A shell starts in your home folder when no project is open, otherwise in the project folder.", "Open Terminal", t.newSession))
	t.frame = container.NewBorder(top, nil, nil, nil, t.body)
	t.tab = container.NewTabItem("Terminal", t.frame)
	t.view = termview.New()
	t.view.SetTextSize(s.textSize(prefTerminalTextSize))
	t.view.ShellTakes = s.takesFromTerminal
	t.view.OnShortcut = func(sc fyne.Shortcut) { s.runChord(sc) }
	t.view.Dispatch = s.dispatch
	t.view.OnFocus = func(on bool) {
		if on {
			s.activate(regionPanel)
		}
	}
	t.view.Input = func(b []byte) {
		if ts := t.current; ts != nil && !ts.done {
			_ = ts.t.Write(b)
		}
	}
	t.view.OnResize = func(cols, rows int) {
		if ts := t.current; ts != nil {
			ts.t.Resize(cols, rows)
		}
	}
	return t.tab
}

// sessionPicker is the terminal panel's session Select, with the selected session's whole name
// as its tooltip: a narrow panel cuts it short.
type sessionPicker struct {
	widget.Select
}

func newSessionPicker(changed func(string)) *sessionPicker {
	p := &sessionPicker{}
	p.OnChanged = changed
	p.ExtendBaseWidget(p)
	return p
}

func (p *sessionPicker) MouseIn(e *desktop.MouseEvent) {
	p.Select.MouseIn(e)
	kit.Hover(p, p.Selected)
}

func (p *sessionPicker) MouseOut() {
	p.Select.MouseOut()
	kit.Unhover(p)
}

func (p *sessionPicker) Tapped(e *fyne.PointEvent) {
	kit.Unhover(p) // the list opening answers what the tooltip would
	p.Select.Tapped(e)
}

// placeholderAction is a placeholder with one button under its text.
func placeholderAction(icon icons.Name, title, detail, label string, run func()) fyne.CanvasObject {
	p := placeholder(icon, title, detail).(*fyne.Container)
	b := widget.NewButton(label, run)
	b.Importance = widget.HighImportance
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
// root (the home folder without one) when argv is nil.
func (t *terminalHost) launch(ts *termSession) error {
	argv, dir := ts.argv, ts.dir
	if argv == nil {
		argv, dir = pty.DefaultShell(), t.s.processFolder()
	}
	cols, rows := t.view.GridSize()
	if cols < 2 {
		cols, rows = 100, 12
	}
	var self *term.Session
	sess, err := term.Start(pty.Options{Argv: argv, Dir: dir, Cols: cols, Rows: rows}, term.Hooks{
		Changed: t.view.Changed,
		Exited: func(status int) {
			t.s.dispatch(func() { t.closed(ts, self, status) })
		},
	})
	if err != nil {
		return err
	}
	self = sess
	ts.t, ts.done, ts.status = sess, false, 0
	return nil
}

// newSession starts another shell and selects it.
func (t *terminalHost) newSession() {
	if !t.s.editorEnabled {
		return
	}
	ts := &termSession{name: fmt.Sprintf("Shell %d", t.next)}
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

// run starts a program in a session of its own and selects it, so it can read what is typed;
// Restart runs it again. The session is named after it.
// typeAt and typed, when set, are lines typed into the program at its prompt (Plan.Type).
func (t *terminalHost) run(name string, argv []string, dir, typeAfter, typeAt string, typed []string) {
	ts := &termSession{name: t.freeName(name), argv: argv, dir: dir}
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

// showSession makes ts the session on view.
func (t *terminalHost) showSession(ts *termSession) {
	t.current = ts
	t.view.SetScreen(ts.t.Screen)
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

// selectSession shows a session's program, or its exit status if it ended.
func (t *terminalHost) selectSession(ts *termSession) {
	if ts == nil {
		return
	}
	if ts != t.current {
		t.showSession(ts)
		return
	}
	t.sync()
	t.focus()
}

func (t *terminalHost) selectByName(name string) {
	for _, ts := range t.sessions {
		if ts.name == name && ts != t.current {
			t.selectSession(ts)
			return
		}
	}
}

// cycle selects the next (1) or previous (-1) session.
func (t *terminalHost) cycle(step int) {
	n := len(t.sessions)
	if n < 2 {
		return
	}
	i := slices.Index(t.sessions, t.current)
	t.selectSession(t.sessions[((i+step)%n+n)%n])
}

// restart replaces the selected session's program with a new one, keeping its name: the
// shell, or the program it ran, again.
func (t *terminalHost) restart() {
	ts := t.current
	if ts == nil {
		t.newSession()
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
	old.Close() // ends the old program if it still runs
	t.showSession(ts)
}

// clear empties the selected session's scrollback and has its shell clear the screen.
func (t *terminalHost) clear() {
	if ts := t.current; ts != nil {
		ts.t.Clear()
		t.view.SetScreen(ts.t.Screen)
	}
	t.focus()
}

// closeCurrent ends the selected session's program and removes it.
func (t *terminalHost) closeCurrent() {
	ts := t.current
	if ts == nil {
		return
	}
	i := slices.Index(t.sessions, ts)
	t.sessions = slices.Delete(t.sessions, i, i+1)
	t.current = nil
	ts.t.Close()
	if len(t.sessions) > 0 {
		t.selectSession(t.sessions[min(i, len(t.sessions)-1)])
	} else {
		t.view.SetScreen(nil)
	}
	t.s.logf("Terminal: %s closed", ts.name)
	t.sync()
}

// renameCurrent asks for a new name for the selected session.
func (t *terminalHost) renameCurrent() {
	ts := t.current
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
		detail := "running"
		if ts.done {
			detail = fmt.Sprintf("exited (%d)", ts.status)
		}
		if ts == t.current {
			detail += ", current"
		}
		items = append(items, commandpalette.Item{Title: ts.name, Detail: detail, Icon: icons.Terminal, Run: func() {
			t.s.showDockItem(t.tab)
			t.selectSession(ts)
		}})
	}
	t.s.palette.Show("Switch to a terminal", items)
}

// sync shows the selected session: its shell, or why it is not running, and the session list.
func (t *terminalHost) sync() {
	names := make([]string, len(t.sessions))
	for i, ts := range t.sessions {
		names[i] = ts.name
		if ts.done {
			names[i] += " (exited)"
		}
	}
	t.pick.Options = names
	t.pick.Selected = ""
	if t.current != nil {
		t.pick.Selected = names[slices.Index(t.sessions, t.current)]
	}
	t.pick.Refresh()
	t.bar.Refresh()
	t.view.SetCursorShown(t.current != nil && !t.current.done)
	switch {
	case t.current == nil:
		t.show(placeholderAction(icons.Terminal, "No terminal session", "New Terminal starts a shell in "+t.s.root+".",
			"New Terminal", t.newSession))
	case t.current.done:
		// The output stays, under a line saying how the shell ended.
		strip := exitStrip(exitDescription(t.current.who(), t.current.status)+".", t.current.status != 0, t.restart)
		t.show(container.NewBorder(container.NewPadded(strip), nil, nil, nil, t.view))
	default:
		t.show(t.view)
	}
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
		container.NewVBox(widget.NewButton("Restart", restart)), note)
}

// closed records a program's exit; its output stays on view under a line saying how it ended
// (PXF-DEF-015). The exit of a program a restart or a close replaced is not the session's.
func (t *terminalHost) closed(ts *termSession, sess *term.Session, status int) {
	if ts.t != sess || ts.done || !slices.Contains(t.sessions, ts) {
		return
	}
	ts.done, ts.status = true, status
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
	if file, ok := strings.CutPrefix(ts.name, "Debug "); ok && ts.argv != nil {
		return "The debugger for " + file
	}
	if ts.argv != nil {
		return strings.TrimPrefix(ts.name, "Run ")
	}
	return "The shell"
}

// focus gives the terminal keyboard focus when the selected program runs.
func (t *terminalHost) focus() {
	if t.current == nil || t.current.done {
		return
	}
	if c := t.s.win.Canvas(); c != nil {
		c.Focus(t.view)
	}
}

// syncTheme redraws the terminal in the current theme: its colours, ANSI ones included, are
// the theme's, read as it draws.
func (t *terminalHost) syncTheme() {
	if t != nil && t.view != nil {
		t.view.Refresh()
	}
}

// stop ends every session's program and waits, at most stopWait, for them to go: they hold
// the workspace open as their folder.
func (t *terminalHost) stop() {
	if t == nil {
		return
	}
	for _, ts := range t.sessions {
		ts.t.Close()
	}
	limit := time.After(stopWait)
	for _, ts := range t.sessions {
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
