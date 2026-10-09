package shell

import (
	"context"
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"github.com/obstinix/PyxForge/internal/neovim"
	"github.com/obstinix/PyxForge/internal/ui/editor"
	"github.com/obstinix/PyxForge/internal/ui/icons"
	"github.com/obstinix/PyxForge/internal/ui/theme"
)

// terminalHost runs the Terminal panel: a second embedded Neovim, separate from the editor's,
// whose one window is a :terminal running the user's shell ('shell') in the workspace root.
// Neovim supplies the terminal emulator and the pseudo-terminal (ConPTY on Windows), and the
// editor view draws it, so the panel gets the editor's keys, mouse, theme and clipboard.
type terminalHost struct {
	s    *Shell
	tab  *container.TabItem
	body *fyne.Container // the view while a shell runs; otherwise a message saying why not
	sess *neovim.Session
	view *editor.View
	buf  int  // the running shell's terminal buffer
	done bool // the shell exited; the panel waits for Restart
}

// panelLua strips the editor chrome a single terminal window does not need.
const panelLua = `vim.o.laststatus = 0
vim.o.showmode = false
vim.o.ruler = false
vim.o.showcmd = false
pcall(function() vim.o.cmdheight = 0 end)`

func (s *Shell) buildTerminal() *container.TabItem {
	t := &terminalHost{s: s}
	t.body = container.NewStack(placeholderAction(icons.Terminal, "No terminal session",
		"A shell starts in the project folder when this tab opens.", "Open Terminal", t.restart))
	t.tab = container.NewTabItem("Terminal", t.body)
	s.term = t
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

// start launches the terminal the first time the panel is shown. A shell that has exited is
// not restarted on its own: the panel keeps its exit status until the user asks.
func (t *terminalHost) start() {
	if t.sess != nil || t.done || !t.s.editorEnabled {
		return
	}
	t.restart()
}

// restart runs a new shell, starting the terminal's Neovim first if it is not running.
func (t *terminalHost) restart() {
	if !t.s.editorEnabled {
		return
	}
	if t.sess == nil {
		if err := t.launch(); err != nil {
			t.show(placeholderAction(icons.Terminal, "Terminal unavailable", err.Error()+
				". The terminal runs inside Neovim; pyxforge doctor shows how to install it.", "Try Again", t.restart))
			t.s.logf("Terminal could not start: %v", err)
			return
		}
	}
	buf, err := t.sess.Terminal()
	if err != nil {
		t.show(placeholderAction(icons.Terminal, "The shell did not start", err.Error(), "Try Again", t.restart))
		t.s.logf("Terminal: the shell did not start: %v", err)
		return
	}
	t.buf, t.done = buf, false
	t.show(t.view)
	t.focus()
}

// launch starts the Neovim that hosts the terminal, with PyxForge's configuration and theme.
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
				return // stopped on purpose, or an older session
			}
			t.sess, t.done = nil, true
			t.show(placeholderAction(icons.Terminal, "Terminal stopped", "Its Neovim exited"+errSuffix(err)+".",
				"Restart", t.restart))
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
	s.logf("Terminal started in %s", s.root)
	return nil
}

// closed handles a shell exiting. Events for terminals replaced by Restart are ignored.
func (t *terminalHost) closed(buf, status int) {
	if buf != t.buf || t.done {
		return
	}
	t.done = true
	title := "The shell exited"
	if status != 0 {
		title = fmt.Sprintf("The shell exited with status %d", status)
	}
	t.show(placeholderAction(icons.Terminal, title, "Restart opens a new shell in "+t.s.root+".", "Restart", t.restart))
	t.s.logf("Terminal: shell exited with status %d", status)
}

// focus gives the terminal keyboard focus when its shell is running.
func (t *terminalHost) focus() {
	if t.sess == nil || t.done {
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

// stop ends the terminal's shell and Neovim.
func (t *terminalHost) stop() {
	if t == nil || t.sess == nil {
		return
	}
	sess := t.sess
	t.sess = nil
	_ = sess.Close()
}
