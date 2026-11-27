package shell

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"github.com/obstinix/PyxForge/internal/pty"
	"github.com/obstinix/PyxForge/internal/ui/icons"
	"github.com/obstinix/PyxForge/internal/ui/kit"
	"github.com/obstinix/PyxForge/internal/ui/notifications"
	"github.com/obstinix/PyxForge/internal/ui/theme"
)

// termSearch is the bar above the terminal's panes that searches the active session's output
// (Ctrl+Shift+F): every match highlighted, Enter and Shift+Enter (or the arrows) step through
// them, Esc closes it.
type termSearch struct {
	t     *terminalHost
	box   *fyne.Container
	field *termSearchEntry
	count *kit.Text
	match *widget.Check
}

// termSearchEntry is the bar's field: Esc closes the bar, Enter goes to the next match and
// Shift+Enter to the previous.
type termSearchEntry struct {
	widget.Entry
	bar *termSearch
}

func (e *termSearchEntry) TypedKey(ev *fyne.KeyEvent) {
	switch ev.Name {
	case fyne.KeyEscape:
		e.bar.close()
		return
	case fyne.KeyReturn, fyne.KeyEnter:
		step := 1
		if d, ok := fyne.CurrentApp().Driver().(interface{ CurrentKeyModifiers() fyne.KeyModifier }); ok &&
			d.CurrentKeyModifiers()&fyne.KeyModifierShift != 0 {
			step = -1
		}
		e.bar.step(step)
		return
	}
	e.Entry.TypedKey(ev)
}

func newTermSearch(t *terminalHost) *termSearch {
	b := &termSearch{t: t}
	b.field = &termSearchEntry{bar: b}
	b.field.ExtendBaseWidget(b.field)
	b.field.SetPlaceHolder("Find in terminal output")
	b.field.OnChanged = func(string) { b.run() }
	b.count = kit.NewText("", kit.Body, kit.Tertiary)
	b.match = widget.NewCheck("Match case", func(bool) { b.run() })
	prev := kit.NewIconButton(icons.ChevronUp, "Previous Match (Shift+Enter)", func() { b.step(-1) })
	next := kit.NewIconButton(icons.ChevronDown, "Next Match (Enter)", func() { b.step(1) })
	closeBtn := kit.NewIconButton(icons.X, "Close Find (Esc)", b.close)
	for _, btn := range []*kit.IconButton{prev, next, closeBtn} {
		btn.Side = 28
	}
	row := container.NewBorder(nil, nil, nil, container.NewHBox(b.count, prev, next, b.match, closeBtn), b.field)
	b.box = container.NewVBox(container.New(layout.NewCustomPaddedLayout(theme.Space1, theme.Space1, theme.Space2, theme.Space2), row), kit.NewRule(false))
	b.box.Hide()
	return b
}

// open shows the bar with the cursor in its field.
func (b *termSearch) open() {
	t := b.t
	t.s.showDockItem(t.tab)
	if t.current == nil {
		return
	}
	b.box.Show()
	t.frame.Refresh()
	t.s.win.Canvas().Focus(b.field)
	if b.field.Text != "" {
		b.run()
	}
}

// close hides the bar, clears the highlights and returns to the terminal.
func (b *termSearch) close() {
	b.box.Hide()
	for _, p := range b.t.panes {
		p.view.SetSearch("", false)
	}
	b.t.frame.Refresh()
	b.t.focus()
}

// Visible reports whether the bar shows.
func (b *termSearch) Visible() bool { return b.box.Visible() }

func (b *termSearch) run() {
	n := b.t.view.SetSearch(b.field.Text, b.match.Checked)
	switch {
	case b.field.Text == "":
		b.count.SetText("")
	case n == 0:
		b.count.SetText("No results")
	default:
		b.count.SetText(fmt.Sprintf("%d of %d", n, n))
	}
}

func (b *termSearch) step(dir int) {
	at, n := b.t.view.NextMatch(dir)
	if n > 0 {
		b.count.SetText(fmt.Sprintf("%d of %d", at, n))
	}
}

// newShellOf starts a session running a particular shell (the rail's kind menu), leaving the
// default in Settings as it is.
func (t *terminalHost) newShellOf(sh pty.Shell) {
	if !t.s.editorEnabled {
		return
	}
	ts := &termSession{name: fmt.Sprintf("%s %d", shellLabel(sh), t.next), kind: kindShell, shell: &sh}
	if err := t.launch(ts); err != nil {
		t.s.Notify(notifications.Error, "The shell did not start", err.Error())
		return
	}
	t.next++
	t.sessions = append(t.sessions, ts)
	t.showSession(ts)
}

// shellLabel is a short name for a shell: pwsh, bash, cmd.
func shellLabel(sh pty.Shell) string {
	if k := sh.Kind(); k != "" {
		return k
	}
	return "Shell"
}
