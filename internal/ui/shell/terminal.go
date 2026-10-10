package shell

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"github.com/obstinix/PyxForge/internal/neovim"
	"github.com/obstinix/PyxForge/internal/ui/commandpalette"
	"github.com/obstinix/PyxForge/internal/ui/editor"
	"github.com/obstinix/PyxForge/internal/ui/icons"
	"github.com/obstinix/PyxForge/internal/ui/kit"
	"github.com/obstinix/PyxForge/internal/ui/notifications"
	"github.com/obstinix/PyxForge/internal/ui/theme"
)

// terminalHost runs the Terminal panel: a second embedded Neovim, separate from the editor's,
// in which each terminal session is a :terminal buffer running the user's shell ('shell') in
// the workspace root. Neovim supplies the terminal emulator and the pseudo-terminal (ConPTY on
// Windows); the editor view draws whichever session is selected, so the panel gets the
// editor's keys, mouse, theme and clipboard. Each session is its own process with its own
// output; the others keep running while hidden.
type terminalHost struct {
	s        *Shell
	tab      *container.TabItem
	body     *fyne.Container // the view while the selected shell runs; otherwise a message
	frame    fyne.CanvasObject
	bar      *fyne.Container
	pick     *widget.Select
	sess     *neovim.Session
	view     *editor.View
	sessions []*termSession
	current  *termSession
	next     int  // the number for the next session's default name
	failed   bool // Neovim could not start or stopped; the panel waits for the user
}

// termSession is one shell.
type termSession struct {
	name   string
	buf    int  // its terminal buffer in the panel's Neovim
	done   bool // the shell exited
	status int  // its exit status, once done
}

// panelLua strips the editor chrome a terminal window does not need, and removes Neovim's own
// "close the buffer when the shell exits with 0" rule (0.10+), so the panel can report every
// exit and keep the session until it is closed.
const panelLua = `vim.o.laststatus = 0
vim.o.showmode = false
vim.o.ruler = false
vim.o.showcmd = false
pcall(function() vim.o.cmdheight = 0 end)
for _, g in ipairs({ "nvim.terminal", "nvim_terminal" }) do
  pcall(vim.api.nvim_del_augroup_by_name, g)
end`

func (s *Shell) buildTerminal() *container.TabItem {
	t := &terminalHost{s: s, next: 1}
	s.term = t
	t.pick = widget.NewSelect(nil, func(name string) { t.selectByName(name) })
	t.pick.PlaceHolder = "No terminal"
	newBtn := kit.NewIconButton(icons.Plus, "New Terminal (Ctrl+Shift+`)", t.newSession)
	rename := kit.NewIconButton(icons.FileText, "Rename Terminal", t.renameCurrent)
	restart := kit.NewIconButton(icons.Refresh, "Restart Terminal", t.restart)
	closeBtn := kit.NewIconButton(icons.X, "Close Terminal", t.closeCurrent)
	t.bar = container.NewHBox(t.pick, layout.NewSpacer(), newBtn, rename, restart, closeBtn)
	top := container.NewVBox(container.New(layout.NewCustomPaddedLayout(0, 0, theme.Space2, theme.Space1),
		kit.Row(theme.TabBarHeight, t.bar)), kit.NewRule(false))
	t.body = container.NewStack(placeholderAction(icons.Terminal, "No terminal session",
		"A shell starts in the project folder when this tab opens.", "Open Terminal", t.newSession))
	t.frame = container.NewBorder(top, nil, nil, nil, t.body)
	t.tab = container.NewTabItem("Terminal", t.frame)
	return t.tab
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

// start opens the first session when the panel is first shown. Sessions that exited and a
// Neovim that failed are left for the user to restart.
func (t *terminalHost) start() {
	if len(t.sessions) > 0 || t.failed || !t.s.editorEnabled {
		return
	}
	t.newSession()
}

// ensure starts the panel's Neovim if it is not running.
func (t *terminalHost) ensure() bool {
	if !t.s.editorEnabled {
		return false
	}
	if t.sess != nil {
		return true
	}
	if err := t.launch(); err != nil {
		t.failed = true
		t.show(placeholderAction(icons.Terminal, "Terminal unavailable", err.Error()+
			". The terminal runs inside Neovim; pyxforge doctor shows how to install it.", "Try Again", t.retry))
		t.s.logf("Terminal could not start: %v", err)
		return false
	}
	t.failed = false
	return true
}

func (t *terminalHost) retry() {
	t.failed = false
	if len(t.sessions) == 0 {
		t.newSession()
	} else {
		t.restart()
	}
}

// newSession starts another shell and selects it.
func (t *terminalHost) newSession() {
	if !t.ensure() {
		return
	}
	buf, err := t.sess.NewTerminal()
	if err != nil {
		t.show(placeholderAction(icons.Terminal, "The shell did not start", err.Error(), "Try Again", t.newSession))
		t.s.logf("Terminal: the shell did not start: %v", err)
		return
	}
	ts := &termSession{name: fmt.Sprintf("Shell %d", t.next), buf: buf}
	t.next++
	t.sessions = append(t.sessions, ts)
	t.current = ts
	t.sync()
	t.focus()
	t.s.logf("Terminal: %s started in %s", ts.name, t.s.root)
}

// selectSession shows a session's shell, or its exit status if it ended.
func (t *terminalHost) selectSession(ts *termSession) {
	if ts == nil || t.sess == nil {
		return
	}
	if ts != t.current {
		t.current = ts
		if err := t.sess.ShowTerminal(ts.buf); err != nil {
			t.s.logf("Terminal: cannot show %s: %v", ts.name, err)
		}
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

// restart replaces the selected session's shell with a new one, keeping its name.
func (t *terminalHost) restart() {
	if t.current == nil {
		t.newSession()
		return
	}
	if !t.ensure() {
		return
	}
	if err := t.sess.ShowTerminal(t.current.buf); err != nil {
		t.s.logf("Terminal: %v", err)
	}
	buf, err := t.sess.Terminal() // replaces the shown buffer, ending its job if it still runs
	if err != nil {
		t.show(placeholderAction(icons.Terminal, "The shell did not start", err.Error(), "Try Again", t.restart))
		return
	}
	t.current.buf, t.current.done, t.current.status = buf, false, 0
	t.sync()
	t.focus()
}

// closeCurrent ends the selected session's shell and removes it.
func (t *terminalHost) closeCurrent() {
	ts := t.current
	if ts == nil {
		return
	}
	i := slices.Index(t.sessions, ts)
	t.sessions = slices.Delete(t.sessions, i, i+1)
	t.current = nil
	if t.sess != nil {
		if len(t.sessions) > 0 {
			t.selectSession(t.sessions[min(i, len(t.sessions)-1)])
		}
		sess, buf := t.sess, ts.buf
		go func() { _ = sess.CloseBuffer(buf, false, true) }() // ends the shell
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
	entry := widget.NewEntry()
	entry.SetText(ts.name)
	d := dialog.NewForm("Rename Terminal", "Rename", "Cancel", []*widget.FormItem{widget.NewFormItem("Name", entry)},
		func(ok bool) {
			if ok {
				t.rename(ts, entry.Text)
			}
		}, t.s.win)
	d.Resize(fyne.NewSize(360, d.MinSize().Height))
	d.Show()
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
		t.s.showDockTab(slices.Index(t.s.dock.Items, t.tab))
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
			t.s.showDockTab(slices.Index(t.s.dock.Items, t.tab))
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
	switch {
	case t.failed:
	case t.current == nil:
		t.show(placeholderAction(icons.Terminal, "No terminal session", "New Terminal starts a shell in "+t.s.root+".",
			"New Terminal", t.newSession))
	case t.current.done:
		title := "The shell exited"
		if t.current.status != 0 {
			title = fmt.Sprintf("The shell exited with status %d", t.current.status)
		}
		t.show(placeholderAction(icons.Terminal, title, "Restart opens a new shell in "+t.s.root+".", "Restart", t.restart))
	default:
		t.show(t.view)
	}
}

// launch starts the Neovim that hosts the terminals, with PyxForge's configuration and theme.
func (t *terminalHost) launch() error {
	s := t.s
	t.view = editor.NewView()
	t.view.IsShellChord = IsShellChord
	t.view.OnShortcut = s.runChord
	t.view.Dispatch = s.dispatch
	t.view.OnFocus = func(on bool) {
		if on {
			s.activate(regionPanel)
		}
	}
	var self *neovim.Session
	o := s.nvimOptions()
	o.Width, o.Height = 100, 12
	o.OnFlush = t.view.FlushHook
	o.OnEvent = func(e neovim.Event) {
		if e.Kind == "TermClose" {
			s.dispatch(func() { t.closed(e.Buffer, e.Status) })
		}
	}
	o.OnExit = func(err error) {
		s.dispatch(func() {
			if t.sess == nil || t.sess != self {
				return // stopped on purpose, or an older Neovim
			}
			t.sess, t.failed = nil, true
			t.sessions, t.current = nil, nil
			t.sync()
			t.show(placeholderAction(icons.Terminal, "Terminal stopped", "Its Neovim exited"+errSuffix(err)+"; its shells ended with it.",
				"Restart", t.retry))
			s.logf("Terminal: Neovim exited%s", errSuffix(err))
		})
	}
	sess, err := neovim.Start(context.Background(), o)
	if err != nil {
		return err
	}
	self = sess
	t.sess = sess
	t.view.Attach(sess)
	if err := sess.ExecLua(panelLua, nil); err != nil {
		s.logf("Terminal: panel options not applied: %v", err)
	}
	t.syncTheme(true)
	return nil
}

// closed records a shell's exit. Buffers of closed or restarted sessions are not sessions any
// more, so their events are ignored.
func (t *terminalHost) closed(buf, status int) {
	for _, ts := range t.sessions {
		if ts.buf == buf && !ts.done {
			ts.done, ts.status = true, status
			t.s.logf("Terminal: %s exited with status %d", ts.name, status)
			t.sync()
			return
		}
	}
}

// focus gives the terminal keyboard focus when the selected shell runs.
func (t *terminalHost) focus() {
	if t.sess == nil || t.current == nil || t.current.done {
		return
	}
	if c := t.s.win.Canvas(); c != nil {
		c.Focus(t.view)
	}
}

// syncTheme gives the terminal the current theme. ANSI colours reach shells started after the
// change; the default foreground and background follow at once.
func (t *terminalHost) syncTheme(wait bool) {
	if t.sess == nil {
		return
	}
	tk := theme.Current()
	bg, groups := editor.Colorscheme(tk)
	colors := editor.TerminalColors(tk)
	sess, s := t.sess, t.s
	apply := func() {
		err := sess.SetTerminalColors(colors)
		if err == nil {
			err = sess.SetColorscheme("pyxforge", bg, groups)
		}
		if err != nil {
			s.dispatch(func() { s.logf("Terminal colours not applied: %v", err) })
		}
	}
	if wait {
		apply()
		return
	}
	go apply()
}

// stop ends every shell and the terminal's Neovim.
func (t *terminalHost) stop() {
	if t == nil || t.sess == nil {
		return
	}
	sess := t.sess
	t.sess = nil
	_ = sess.Close()
}
