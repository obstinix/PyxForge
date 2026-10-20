package kit

import (
	"slices"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	fynetheme "fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/obstinix/PyxForge/internal/ui/icons"
)

// Tabs is a row of tabs over the selected tab's content: the editor's, the dock's and the
// inspector's, in place of Fyne's AppTabs and DocTabs, with the same methods.
//
// It draws the selected tab's label and 2 px underline in the accent only while SetAccent says
// its region is active, and in the text colour otherwise (DESIGN_SYSTEM.md §12, decision A1).
// With Fyne's tabs that takes a container.ThemeOverride, and every ThemeOverride.Refresh gives
// the objects under it a new theme scope, for which Fyne parses and caches the whole font set
// again, about 13 MB that is never freed (Fyne 2.8.1). Tabs reads the accent from its own state
// instead, so moving the accent between regions costs nothing.
type Tabs struct {
	widget.BaseWidget

	Items []*container.TabItem

	// OnSelected runs when a tab is tapped, even the selected one (so a tap also activates the
	// tabs' region), and when another tab becomes the selected one in code: selected, the first
	// tab appended, or the next one after the selected tab is removed.
	OnSelected func(*container.TabItem)
	// CloseIntercept, when set, runs instead of removing a tab whose close button is tapped.
	CloseIntercept func(*container.TabItem)
	// OnClosed runs after a tab's close button removed it (not when CloseIntercept is set).
	OnClosed func(*container.TabItem)

	closable bool
	accent   bool
	current  int
	buttons  map[*container.TabItem]*tabButton
}

// NewTabs returns tabs without close buttons, like Fyne's AppTabs.
func NewTabs(items ...*container.TabItem) *Tabs { return newTabs(false, items) }

// NewDocTabs returns tabs with close buttons, like Fyne's DocTabs.
func NewDocTabs(items ...*container.TabItem) *Tabs { return newTabs(true, items) }

func newTabs(closable bool, items []*container.TabItem) *Tabs {
	t := &Tabs{Items: items, closable: closable, current: -1, buttons: map[*container.TabItem]*tabButton{}}
	if len(items) > 0 {
		t.current = 0
	}
	t.ExtendBaseWidget(t)
	return t
}

// SetAccent draws the selected tab in the accent (its region is active) or in the text colour.
func (t *Tabs) SetAccent(on bool) {
	if t.accent == on {
		return
	}
	t.accent = on
	t.Refresh()
}

// Accent reports whether the selected tab is drawn in the accent.
func (t *Tabs) Accent() bool { return t.accent }

// Append adds a tab at the end. The first tab of empty tabs becomes the selected one.
func (t *Tabs) Append(item *container.TabItem) {
	t.Items = append(t.Items, item)
	if t.current < 0 {
		t.current = 0
		t.Refresh()
		t.selected(item)
		return
	}
	t.Refresh()
}

// Remove takes a tab away. The selected tab stays selected; if it is the one removed, the tab
// now in its place (or the last) is selected.
func (t *Tabs) Remove(item *container.TabItem) {
	i := slices.Index(t.Items, item)
	if i < 0 {
		return
	}
	was := t.Selected()
	t.Items = slices.Concat(t.Items[:i], t.Items[i+1:]) // a new array: callers may be ranging over the old one
	delete(t.buttons, item)
	switch {
	case len(t.Items) == 0:
		t.current = -1
	case was == item:
		t.current = min(i, len(t.Items)-1)
		t.Refresh()
		t.selected(t.Items[t.current])
		return
	default:
		t.current = slices.Index(t.Items, was)
	}
	t.Refresh()
}

// Select makes item the selected tab.
func (t *Tabs) Select(item *container.TabItem) {
	if i := slices.Index(t.Items, item); i >= 0 {
		t.SelectIndex(i)
	}
}

// SelectIndex makes the tab at index i the selected one.
func (t *Tabs) SelectIndex(i int) {
	if i == t.current || i < 0 || i >= len(t.Items) {
		return
	}
	t.current = i
	t.Refresh()
	t.selected(t.Items[i])
}

// Selected returns the selected tab, or nil.
func (t *Tabs) Selected() *container.TabItem {
	if t.current < 0 || t.current >= len(t.Items) {
		return nil
	}
	return t.Items[t.current]
}

// SelectedIndex returns the selected tab's index, or -1.
func (t *Tabs) SelectedIndex() int { return t.current }

func (t *Tabs) selected(item *container.TabItem) {
	if t.OnSelected != nil {
		t.OnSelected(item)
	}
}

// close is what a tab's close button does.
func (t *Tabs) close(item *container.TabItem) {
	if t.CloseIntercept != nil {
		t.CloseIntercept(item)
		return
	}
	t.Remove(item)
	if t.OnClosed != nil {
		t.OnClosed(item)
	}
}

func (t *Tabs) button(item *container.TabItem) *tabButton {
	b := t.buttons[item]
	if b == nil {
		b = &tabButton{tabs: t, item: item}
		b.ExtendBaseWidget(b)
		t.buttons[item] = b
	}
	return b
}

func (t *Tabs) CreateRenderer() fyne.WidgetRenderer {
	t.ExtendBaseWidget(t)
	r := &tabsRenderer{t: t, row: &fyne.Container{Layout: &tabRowLayout{}}, divider: canvas.NewRectangle(nil),
		indicator: canvas.NewRectangle(nil)}
	r.scroll = container.NewHScroll(r.row)
	r.Refresh()
	return r
}

type tabsRenderer struct {
	t         *Tabs
	row       *fyne.Container // the tab buttons, then the indicator
	scroll    *container.Scroll
	divider   *canvas.Rectangle
	indicator *canvas.Rectangle
	content   fyne.CanvasObject
}

// indicatorHeight is the selected tab's underline (DESIGN_SYSTEM.md §8: a 2 px active marker).
const indicatorHeight = 2

func (r *tabsRenderer) Refresh() {
	t := r.t
	th, v := t.Theme(), fyne.CurrentApp().Settings().ThemeVariant()
	objs := make([]fyne.CanvasObject, 0, len(t.Items)+1)
	for _, it := range t.Items {
		b := t.button(it)
		b.Refresh()
		objs = append(objs, b)
	}
	r.indicator.FillColor = th.Color(fynetheme.ColorNameForeground, v)
	if t.accent {
		r.indicator.FillColor = th.Color(fynetheme.ColorNamePrimary, v)
	}
	r.indicator.Hidden = t.Selected() == nil
	r.row.Objects = append(objs, r.indicator)
	r.row.Layout.(*tabRowLayout).selected = t.current
	r.row.Refresh()
	r.divider.FillColor = th.Color(fynetheme.ColorNameShadow, v)
	r.divider.Refresh()

	var content fyne.CanvasObject
	if sel := t.Selected(); sel != nil {
		content = sel.Content
	}
	if content != r.content {
		if r.content != nil {
			r.content.Hide()
		}
		r.content = content
		if content != nil {
			content.Show()
			r.Layout(t.Size())
		}
	}
	r.scrollToSelected()
}

func (r *tabsRenderer) scrollToSelected() {
	sel := r.t.Selected()
	if sel == nil {
		return
	}
	b := r.t.button(sel)
	left, right := b.Position().X, b.Position().X+b.Size().Width
	visible := r.scroll.Size().Width
	switch {
	case left < r.scroll.Offset.X:
		r.scroll.Offset.X = left
	case right > r.scroll.Offset.X+visible && visible > 0:
		r.scroll.Offset.X = right - visible
	default:
		return
	}
	r.scroll.Refresh()
}

func (r *tabsRenderer) barHeight() float32 { return r.row.MinSize().Height }

func (r *tabsRenderer) Layout(s fyne.Size) {
	th := r.t.Theme()
	bar := r.barHeight()
	r.scroll.Move(fyne.NewPos(0, 0))
	r.scroll.Resize(fyne.NewSize(s.Width, bar))
	sep := th.Size(fynetheme.SizeNameSeparatorThickness)
	r.divider.Move(fyne.NewPos(0, bar))
	r.divider.Resize(fyne.NewSize(s.Width, sep))
	if r.content != nil {
		top := bar + th.Size(fynetheme.SizeNamePadding)
		r.content.Move(fyne.NewPos(0, top))
		r.content.Resize(fyne.NewSize(s.Width, max(s.Height-top, 0)))
	}
	r.scrollToSelected()
}

func (r *tabsRenderer) MinSize() fyne.Size {
	th := r.t.Theme()
	content := fyne.NewSize(0, 0)
	for _, it := range r.t.Items {
		if it.Content != nil {
			content = content.Max(it.Content.MinSize())
		}
	}
	return fyne.NewSize(max(content.Width, r.scroll.MinSize().Width),
		r.barHeight()+th.Size(fynetheme.SizeNamePadding)+content.Height)
}

func (r *tabsRenderer) Objects() []fyne.CanvasObject {
	objs := []fyne.CanvasObject{r.scroll, r.divider}
	if r.content != nil {
		objs = append(objs, r.content)
	}
	return objs
}

func (r *tabsRenderer) Destroy() {}

// tabRowLayout lays the buttons out left to right at their natural width and puts the last
// object, the indicator, under the selected button.
type tabRowLayout struct{ selected int }

func (l *tabRowLayout) Layout(objs []fyne.CanvasObject, s fyne.Size) {
	if len(objs) == 0 {
		return
	}
	buttons, indicator := objs[:len(objs)-1], objs[len(objs)-1]
	x := float32(0)
	for i, b := range buttons {
		w := b.MinSize().Width
		b.Move(fyne.NewPos(x, 0))
		b.Resize(fyne.NewSize(w, s.Height))
		if i == l.selected {
			indicator.Move(fyne.NewPos(x, s.Height-indicatorHeight))
			indicator.Resize(fyne.NewSize(w, indicatorHeight))
		}
		x += w
	}
}

func (l *tabRowLayout) MinSize(objs []fyne.CanvasObject) fyne.Size {
	var w, h float32
	for _, b := range objs[:max(len(objs)-1, 0)] {
		m := b.MinSize()
		w += m.Width
		h = max(h, m.Height)
	}
	return fyne.NewSize(w, h)
}

// tabButton is one tab: its label and, for closable tabs, a close mark, shown while the tab is
// selected or under the pointer. The mark is drawn by the button itself rather than being a
// button of its own, so moving the pointer onto it does not take the hover away from the tab.
type tabButton struct {
	widget.BaseWidget
	tabs      *Tabs
	item      *container.TabItem
	hovered   bool
	overClose bool
}

var (
	_ fyne.Tappable     = (*tabButton)(nil)
	_ desktop.Hoverable = (*tabButton)(nil)
	_ desktop.Mouseable = (*tabButton)(nil)
)

// closeRect is where the close mark is, in the button's coordinates.
func (b *tabButton) closeRect() (fyne.Position, fyne.Size) {
	th := b.Theme()
	d := th.Size(fynetheme.SizeNameInlineIcon)
	pad := th.Size(fynetheme.SizeNameInnerPadding)
	s := b.Size()
	return fyne.NewPos(s.Width-d-pad/2, (s.Height-d)/2), fyne.NewSquareSize(d)
}

func (b *tabButton) onClose(p fyne.Position) bool {
	if !b.tabs.closable {
		return false
	}
	pos, size := b.closeRect()
	return p.X >= pos.X && p.X <= pos.X+size.Width && p.Y >= pos.Y && p.Y <= pos.Y+size.Height
}

func (b *tabButton) Tapped(e *fyne.PointEvent) {
	if b.onClose(e.Position) {
		b.tabs.close(b.item)
		return
	}
	if b.tabs.Selected() == b.item {
		b.tabs.selected(b.item)
		return
	}
	b.tabs.Select(b.item)
}

// MouseDown closes a closable tab on a middle click, as other editors do.
func (b *tabButton) MouseDown(e *desktop.MouseEvent) {
	if e.Button == desktop.MouseButtonTertiary && b.tabs.closable {
		b.tabs.close(b.item)
	}
}
func (b *tabButton) MouseUp(*desktop.MouseEvent) {}

func (b *tabButton) MouseIn(e *desktop.MouseEvent) {
	b.hovered, b.overClose = true, b.onClose(e.Position)
	b.Refresh()
}

func (b *tabButton) MouseMoved(e *desktop.MouseEvent) {
	if over := b.onClose(e.Position); over != b.overClose {
		b.overClose = over
		b.Refresh()
	}
}

func (b *tabButton) MouseOut() {
	b.hovered, b.overClose = false, false
	b.Refresh()
}

func (b *tabButton) CreateRenderer() fyne.WidgetRenderer {
	r := &tabButtonRenderer{b: b, bg: canvas.NewRectangle(nil), label: canvas.NewText("", nil)}
	r.label.TextStyle.Bold = true
	if b.tabs.closable {
		r.closeBg = canvas.NewRectangle(nil)
		r.close = NewIcon(icons.X, Secondary)
	}
	r.Refresh()
	return r
}

type tabButtonRenderer struct {
	b       *tabButton
	bg      *canvas.Rectangle
	label   *canvas.Text
	closeBg *canvas.Rectangle
	close   *Icon
}

func (r *tabButtonRenderer) Refresh() {
	b := r.b
	th, v := b.Theme(), fyne.CurrentApp().Settings().ThemeVariant()
	r.bg.FillColor = th.Color(fynetheme.ColorNameHover, v)
	r.bg.CornerRadius = th.Size(fynetheme.SizeNameSelectionRadius)
	r.bg.Hidden = !b.hovered
	r.bg.Refresh()
	selected := b.tabs.Selected() == b.item
	r.label.Text = b.item.Text
	r.label.TextSize = th.Size(fynetheme.SizeNameText)
	r.label.Color = th.Color(fynetheme.ColorNameForeground, v)
	if selected && b.tabs.accent {
		r.label.Color = th.Color(fynetheme.ColorNamePrimary, v)
	}
	r.label.Refresh()
	if r.close != nil {
		show := b.hovered || selected
		r.close.Hidden = !show
		r.close.IconSize = th.Size(fynetheme.SizeNameInlineIcon) * 0.75
		r.close.Role = Secondary
		if b.overClose {
			r.close.Role = Primary
		}
		r.close.Refresh()
		r.closeBg.FillColor = th.Color(fynetheme.ColorNameHover, v)
		r.closeBg.CornerRadius = th.Size(fynetheme.SizeNameSelectionRadius)
		r.closeBg.Hidden = !b.overClose
		r.closeBg.Refresh()
	}
}

func (r *tabButtonRenderer) Layout(s fyne.Size) {
	pad := r.b.Theme().Size(fynetheme.SizeNameInnerPadding)
	r.bg.Resize(s)
	text := r.label.MinSize()
	r.label.Move(fyne.NewPos(pad, (s.Height-text.Height)/2))
	r.label.Resize(text)
	if r.close != nil {
		pos, size := r.b.closeRect()
		r.closeBg.Move(pos)
		r.closeBg.Resize(size)
		r.close.Move(pos)
		r.close.Resize(size)
	}
}

func (r *tabButtonRenderer) MinSize() fyne.Size {
	th := r.b.Theme()
	pad := th.Size(fynetheme.SizeNameInnerPadding)
	text := r.label.MinSize()
	w, h := text.Width+2*pad, text.Height+2*pad
	if r.close != nil {
		w += th.Size(fynetheme.SizeNameInlineIcon) + pad/2
	}
	return fyne.NewSize(w, h)
}

func (r *tabButtonRenderer) Objects() []fyne.CanvasObject {
	objs := []fyne.CanvasObject{r.bg, r.label}
	if r.close != nil {
		objs = append(objs, r.closeBg, r.close)
	}
	return objs
}

func (r *tabButtonRenderer) Destroy() {}
