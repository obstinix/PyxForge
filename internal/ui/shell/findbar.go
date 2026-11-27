package shell

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"github.com/obstinix/PyxForge/internal/ui/icons"
	"github.com/obstinix/PyxForge/internal/ui/kit"
	"github.com/obstinix/PyxForge/internal/ui/notifications"
	"github.com/obstinix/PyxForge/internal/ui/theme"
)

// findBar is the editor's Find and Replace bar (Ctrl+F, Ctrl+H), above the editor's tabs. It
// searches the file in the selected tab for literal text, as Neovim's own search does, so the
// matches are highlighted and n and N go on from them.
type findBar struct {
	s           *Shell
	box         *fyne.Container
	find, repl  *findEntry
	replaceRow  *fyne.Container
	count       *kit.Text
	case_, word *widget.Check
}

// findEntry is a field of the find bar: Esc closes the bar and returns to the editor; Enter
// runs the field's action (OnSubmitted).
type findEntry struct {
	widget.Entry
	bar *findBar
}

func (e *findEntry) TypedKey(ev *fyne.KeyEvent) {
	if ev.Name == fyne.KeyEscape {
		e.bar.close()
		return
	}
	e.Entry.TypedKey(ev)
}

func (s *Shell) buildFindBar() *findBar {
	b := &findBar{s: s}
	b.find = &findEntry{bar: b}
	b.find.ExtendBaseWidget(b.find)
	b.find.SetPlaceHolder("Find")
	b.find.OnChanged = func(string) { b.search(false, true) }
	b.find.OnSubmitted = func(string) { b.search(false, false) }
	b.repl = &findEntry{bar: b}
	b.repl.ExtendBaseWidget(b.repl)
	b.repl.SetPlaceHolder("Replace with")
	b.repl.OnSubmitted = func(string) { b.replace() }
	b.count = kit.NewText("", kit.Body, kit.Tertiary)
	rerun := func(bool) { b.search(false, true) }
	b.case_ = widget.NewCheck("Match case", rerun)
	b.word = widget.NewCheck("Whole word", rerun)
	prev := kit.NewIconButton(icons.ChevronUp, "Previous Match", func() { b.search(true, false) })
	next := kit.NewIconButton(icons.ChevronDown, "Next Match (Enter)", func() { b.search(false, false) })
	closeBtn := kit.NewIconButton(icons.X, "Close Find (Esc)", b.close)
	for _, btn := range []*kit.IconButton{prev, next, closeBtn} {
		btn.Side = 28
	}
	findRow := container.NewBorder(nil, nil, nil,
		container.NewHBox(b.count, prev, next, b.case_, b.word, closeBtn), b.find)
	b.replaceRow = container.NewBorder(nil, nil, nil, container.NewHBox(
		kit.NewButton("Replace", b.replace), kit.NewButton("Replace All", b.replaceAll)), b.repl)
	b.box = container.NewVBox(container.NewPadded(container.NewVBox(findRow, b.replaceRow)), kit.NewRule(false))
	b.box.Hide()
	return b
}

func (b *findBar) options() FindOptions {
	return FindOptions{MatchCase: b.case_.Checked, WholeWord: b.word.Checked}
}

// open shows the bar, with the replace row for Ctrl+H, and puts the cursor in the find field.
func (b *findBar) open(replace bool) {
	if b.s.activeDoc() == nil {
		b.s.Notify(notifications.Info, "Nothing to search", "Find works in a file open in the editor.")
		return
	}
	if replace {
		b.replaceRow.Show()
	} else {
		b.replaceRow.Hide()
	}
	b.box.Show()
	b.s.editorPane.Refresh()
	b.s.activate(regionEditor)
	b.s.win.Canvas().Focus(b.find)
	b.find.TypedShortcut(&fyne.ShortcutSelectAll{}) // typing replaces the last search
	if b.find.Text != "" {
		b.search(false, true)
	}
}

// close hides the bar, clears the highlighting and returns to the editor.
func (b *findBar) close() {
	b.box.Hide()
	b.s.editorPane.Refresh()
	if d := b.s.activeDoc(); d != nil {
		b.s.nat.find(d, "", b.options(), false, false)
	}
	b.s.FocusEditor()
}

// search finds the next (back: previous) match; with here a match at the cursor counts, as
// while typing.
func (b *findBar) search(back, here bool) {
	if d := b.s.activeDoc(); d != nil {
		b.show(b.s.nat.find(d, b.find.Text, b.options(), back, here), nil, b.find.Text)
		return
	}
}

func (b *findBar) replace() {
	if d := b.s.activeDoc(); d != nil && b.find.Text != "" {
		b.show(b.s.nat.replace(d, b.find.Text, b.repl.Text, b.options()), nil, b.find.Text)
		return
	}
}

func (b *findBar) replaceAll() {
	if d := b.s.activeDoc(); d != nil && b.find.Text != "" {
		b.count.SetText(fmt.Sprintf("Replaced %d", b.s.nat.replaceAll(d, b.find.Text, b.repl.Text, b.options())))
		return
	}
}

// show puts a search's result in the bar.
func (b *findBar) show(r FindResult, err error, text string) {
	switch {
	case err != nil:
		b.count.SetText("")
		b.s.Notify(notifications.Error, "Find failed", err.Error())
	case text == "":
		b.count.SetText("")
	case r.Total == 0:
		b.count.SetText("No results")
	default:
		b.count.SetText(fmt.Sprintf("%d of %d", r.Current, r.Total))
	}
	b.count.TextSize = theme.TextSmall
}
