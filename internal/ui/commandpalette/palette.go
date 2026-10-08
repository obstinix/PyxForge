// Package commandpalette is the keyboard-first overlay that finds and runs commands, files and
// other items by fuzzy match.
package commandpalette

import (
	"sort"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"github.com/obstinix/PyxForge/internal/command"
	"github.com/obstinix/PyxForge/internal/ui/icons"
	"github.com/obstinix/PyxForge/internal/ui/kit"
	"github.com/obstinix/PyxForge/internal/ui/theme"
)

// Item is one row: a command, a file, a theme.
type Item struct {
	Title  string
	Detail string // category or directory, shown dimmed
	Keys   string // keybinding, shown in mono
	Icon   icons.Name
	Run    func()
}

// FromCommands turns registry commands into palette items.
func FromCommands(cs []command.Command) []Item {
	out := make([]Item, len(cs))
	for i, c := range cs {
		out[i] = Item{Title: c.Title, Detail: c.Category, Keys: c.Keys, Run: c.Run}
	}
	return out
}

const maxRows = 10

// Palette is a glass overlay attached to one canvas.
type Palette struct {
	canvas  fyne.Canvas
	entry   *entry
	list    *widget.List
	items   []Item // the source set for the current mode
	shown   []Item // items matching the current query, best first
	sel     int
	overlay *fyne.Container
	glass   *kit.Glass
	empty   fyne.CanvasObject // shown instead of the list when nothing matches
}

// New builds a palette for a canvas. It is not visible until Show.
func New(c fyne.Canvas) *Palette {
	p := &Palette{canvas: c}
	p.entry = newEntry(p)
	p.entry.OnChanged = func(string) { p.filter() }
	p.entry.OnSubmitted = func(string) { p.run(p.sel) }
	p.list = widget.NewList(
		func() int { return len(p.shown) },
		p.createRow,
		p.updateRow,
	)
	p.list.HideSeparators = true
	p.list.OnSelected = func(id widget.ListItemID) { p.sel = id }
	none := kit.NewText("No matches", kit.Body, kit.Tertiary)
	p.empty = kit.Row(theme.PaletteRowHeight, none)
	body := container.NewBorder(p.entry, nil, nil, nil, container.NewStack(p.list, p.empty))
	p.glass = kit.NewGlass(body)
	p.overlay = container.New(&placement{p: p}, newDismissArea(p.Hide), p.glass)
	return p
}

// Show opens the palette over the canvas with the given items and placeholder.
func (p *Palette) Show(placeholder string, items []Item) {
	p.items = items
	p.entry.SetPlaceHolder(placeholder)
	p.entry.SetText("")
	p.filter()
	if !p.Visible() {
		p.canvas.Overlays().Add(p.overlay)
	}
	p.canvas.Focus(p.entry)
}

// Hide closes the palette.
func (p *Palette) Hide() { p.canvas.Overlays().Remove(p.overlay) }

// Visible reports whether the palette is open.
func (p *Palette) Visible() bool {
	for _, o := range p.canvas.Overlays().List() {
		if o == p.overlay {
			return true
		}
	}
	return false
}

// Query and Results expose the current state for tests.
func (p *Palette) Query() string   { return p.entry.Text }
func (p *Palette) Results() []Item { return p.shown }

func (p *Palette) filter() {
	q := p.entry.Text
	if q == "" {
		p.shown = p.items
	} else {
		type hit struct {
			it    Item
			score int
		}
		var hits []hit
		for _, it := range p.items {
			best, ok := command.Match(q, it.Title)
			if s, ok2 := command.Match(q, it.Detail+" "+it.Title); ok2 && (!ok || s > best) {
				best, ok = s, true
			}
			if ok {
				hits = append(hits, hit{it, best})
			}
		}
		sort.SliceStable(hits, func(i, j int) bool { return hits[i].score > hits[j].score })
		p.shown = make([]Item, len(hits))
		for i, h := range hits {
			p.shown[i] = h.it
		}
	}
	if len(p.shown) == 0 {
		p.list.Hide()
		p.empty.Show()
	} else {
		p.empty.Hide()
		p.list.Show()
	}
	p.list.Refresh()
	p.sel = 0
	p.move(0)
	p.overlay.Refresh()
}

func (p *Palette) move(delta int) {
	if len(p.shown) == 0 {
		p.sel = 0
		p.list.UnselectAll()
		return
	}
	p.sel = (p.sel + delta + len(p.shown)) % len(p.shown)
	p.list.Select(p.sel)
	p.list.ScrollTo(p.sel)
}

func (p *Palette) run(i int) {
	if i < 0 || i >= len(p.shown) {
		return
	}
	it := p.shown[i]
	p.Hide()
	if it.Run != nil {
		it.Run()
	}
}

func (p *Palette) createRow() fyne.CanvasObject {
	icon := kit.NewIcon(icons.Command, kit.Secondary)
	title := kit.NewText("", kit.Body, kit.Primary)
	detail := kit.NewText("", kit.Body, kit.Tertiary)
	keys := kit.NewText("", kit.Mono, kit.Tertiary)
	keys.TextSize = theme.TextCaption
	return kit.Row(theme.PaletteRowHeight, container.NewHBox(icon, title, detail, layout.NewSpacer(), keys))
}

func (p *Palette) updateRow(id widget.ListItemID, o fyne.CanvasObject) {
	if id >= len(p.shown) {
		return
	}
	it := p.shown[id]
	row := o.(*fyne.Container).Objects[0].(*fyne.Container).Objects
	icon := row[0].(*kit.Icon)
	if it.Icon == "" {
		icon.Hide()
	} else {
		icon.Name = it.Icon
		icon.Show()
		icon.Refresh()
	}
	row[1].(*kit.Text).SetText(it.Title)
	row[2].(*kit.Text).SetText(it.Detail)
	row[4].(*kit.Text).SetText(it.Keys)
	o.(*fyne.Container).Refresh() // re-lay out: the texts just changed width
}

// entry routes navigation keys to the palette and everything else to the text field.
type entry struct {
	widget.Entry
	p *Palette
}

func newEntry(p *Palette) *entry {
	e := &entry{p: p}
	e.ExtendBaseWidget(e)
	return e
}

func (e *entry) TypedKey(k *fyne.KeyEvent) {
	switch k.Name {
	case fyne.KeyDown:
		e.p.move(1)
	case fyne.KeyUp:
		e.p.move(-1)
	case fyne.KeyEscape:
		e.p.Hide()
	default:
		e.Entry.TypedKey(k)
	}
}

// placement puts the glass panel near the top centre and lets the dismiss area fill the canvas.
type placement struct{ p *Palette }

func (l *placement) Layout(objs []fyne.CanvasObject, s fyne.Size) {
	objs[0].Resize(s)
	w := min(theme.PaletteWidth, s.Width-2*theme.Space6)
	rows := max(1, min(len(l.p.shown), maxRows)) // one row for the empty state
	// Fyne lists put theme padding (4 px) between rows.
	h := l.p.entry.MinSize().Height + float32(rows)*(theme.PaletteRowHeight+theme.Space1) + 3*theme.Space2
	objs[1].Resize(fyne.NewSize(w, h))
	objs[1].Move(fyne.NewPos((s.Width-w)/2, s.Height*0.12))
}

func (l *placement) MinSize([]fyne.CanvasObject) fyne.Size { return fyne.Size{} }

// dismissArea is an invisible full-canvas target: clicking outside the palette closes it.
type dismissArea struct {
	widget.BaseWidget
	onTap func()
}

func newDismissArea(onTap func()) *dismissArea {
	d := &dismissArea{onTap: onTap}
	d.ExtendBaseWidget(d)
	return d
}

func (d *dismissArea) Tapped(*fyne.PointEvent) { d.onTap() }

func (d *dismissArea) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(layout.NewSpacer())
}
