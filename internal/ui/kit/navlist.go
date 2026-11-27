package kit

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/widget"
	"github.com/obstinix/PyxForge/internal/ui/icons"
	"github.com/obstinix/PyxForge/internal/ui/theme"
)

// NavItem is one entry of a NavList, or a section heading when Heading is set.
type NavItem struct {
	ID      string
	Label   string
	Icon    icons.Name
	Heading bool // a section heading ("Options"): not selectable
}

// navRowHeight is an item's height; headings are as tall, with more space above them.
const navRowHeight float32 = 32

// NavList is a vertical list of pages under section headings, as in Obsidian's settings: an
// icon and a label per item, the selected one on a Raised rounded fill. With keyboard focus,
// Up and Down move a cursor (hover wash and focus ring on that row) and Enter or Space selects
// it; a click selects at once.
type NavList struct {
	widget.BaseWidget
	Items      []NavItem
	OnSelected func(id string)

	selected, cursor int
	hovered          int
	focused          bool
}

var (
	_ fyne.Tappable  = (*NavList)(nil)
	_ fyne.Focusable = (*NavList)(nil)
)

// NewNavList returns a list; the first selectable item is selected.
func NewNavList(items []NavItem, selected func(id string)) *NavList {
	l := &NavList{Items: items, OnSelected: selected, selected: -1, hovered: -1}
	for i, it := range items {
		if !it.Heading {
			l.selected, l.cursor = i, i
			break
		}
	}
	l.ExtendBaseWidget(l)
	return l
}

// Selected is the selected item's ID.
func (l *NavList) Selected() string {
	if l.selected < 0 {
		return ""
	}
	return l.Items[l.selected].ID
}

// Select selects the item with id and reports it.
func (l *NavList) Select(id string) {
	for i, it := range l.Items {
		if it.ID == id && !it.Heading {
			l.selected, l.cursor = i, i
			l.Refresh()
			if l.OnSelected != nil {
				l.OnSelected(id)
			}
			return
		}
	}
}

// rowAt is the index of the row under y, or -1.
func (l *NavList) rowAt(y float32) int {
	i := int(y / navRowHeight)
	if i < 0 || i >= len(l.Items) || l.Items[i].Heading {
		return -1
	}
	return i
}

func (l *NavList) Tapped(e *fyne.PointEvent) {
	if i := l.rowAt(e.Position.Y); i >= 0 {
		l.Select(l.Items[i].ID)
	}
}

// moveCursor moves the keyboard cursor to the next selectable item in direction dir.
func (l *NavList) moveCursor(dir int) {
	for i := l.cursor + dir; i >= 0 && i < len(l.Items); i += dir {
		if !l.Items[i].Heading {
			l.cursor = i
			l.Refresh()
			return
		}
	}
}

func (l *NavList) FocusGained() { l.focused = true; l.cursor = max(l.selected, 0); l.Refresh() }
func (l *NavList) FocusLost()   { l.focused = false; l.Refresh() }
func (l *NavList) TypedRune(r rune) {
	if r == ' ' {
		l.Select(l.Items[l.cursor].ID)
	}
}
func (l *NavList) TypedKey(e *fyne.KeyEvent) {
	switch e.Name {
	case fyne.KeyUp:
		l.moveCursor(-1)
	case fyne.KeyDown:
		l.moveCursor(1)
	case fyne.KeyReturn, fyne.KeyEnter:
		l.Select(l.Items[l.cursor].ID)
	}
}

// Cursor is the item the keyboard cursor is on; for tests.
func (l *NavList) Cursor() string { return l.Items[l.cursor].ID }

func (l *NavList) CreateRenderer() fyne.WidgetRenderer {
	r := &navListRenderer{l: l, fill: canvas.NewRectangle(nil), wash: canvas.NewRectangle(nil), ring: canvas.NewRectangle(nil)}
	for _, it := range l.Items {
		row := navRow{label: NewText(it.Label, Body, Secondary)}
		if it.Heading {
			row.label.Face, row.label.Role = Label, Tertiary
		} else if it.Icon != "" {
			row.icon = NewIcon(it.Icon, Secondary)
		}
		r.rows = append(r.rows, row)
	}
	r.Refresh()
	return r
}

type navRow struct {
	icon  *Icon
	label *Text
}

type navListRenderer struct {
	l                *NavList
	fill, wash, ring *canvas.Rectangle
	rows             []navRow
}

func (r *navListRenderer) Refresh() {
	t := theme.Current()
	l := r.l
	r.fill.FillColor, r.fill.CornerRadius = t.Surface.Raised, theme.RadiusControl
	r.fill.StrokeColor, r.fill.StrokeWidth = t.Border.Hairline, 1
	r.fill.Hidden = l.selected < 0
	r.wash.FillColor, r.wash.CornerRadius = t.State.Hover, theme.RadiusControl
	r.wash.Hidden = !(l.focused && l.cursor != l.selected)
	styleRing(r.ring, t)
	r.ring.Hidden = !l.focused
	for i, row := range r.rows {
		if l.Items[i].Heading {
			// Only primary text sits on bare glass (TestGlassContrast): on Liquid Glass the
			// list's labels, heading included, are primary; the selected item keeps its fill.
			row.label.Role = Tertiary
			if theme.GlassOn() {
				row.label.Role = Primary
			}
			row.label.Refresh()
			continue
		}
		role := Secondary
		if i == l.selected || theme.GlassOn() {
			role = Primary
		}
		row.label.Role = role
		row.label.Refresh()
		if row.icon != nil {
			row.icon.Role = role
			row.icon.Refresh()
		}
	}
	r.fill.Refresh()
	r.wash.Refresh()
	r.ring.Refresh()
	r.Layout(l.Size())
}

func (r *navListRenderer) Layout(s fyne.Size) {
	at := func(rect *canvas.Rectangle, i int) {
		if i < 0 {
			return
		}
		rect.Move(fyne.NewPos(0, float32(i)*navRowHeight+2))
		rect.Resize(fyne.NewSize(s.Width, navRowHeight-4))
	}
	at(r.fill, r.l.selected)
	at(r.wash, r.l.cursor)
	at(r.ring, r.l.cursor)
	for i, row := range r.rows {
		y := float32(i) * navRowHeight
		x := theme.Space3
		if row.icon != nil {
			row.icon.Move(fyne.NewPos(x, y+(navRowHeight-theme.IconSize)/2))
			row.icon.Resize(fyne.NewSquareSize(theme.IconSize))
			x += theme.IconSize + theme.Space3
		}
		m := row.label.MinSize()
		if r.l.Items[i].Heading {
			x = theme.Space3
			y += navRowHeight - m.Height - theme.Space1 // headings sit low, near their items
		} else {
			y += (navRowHeight - m.Height) / 2
		}
		row.label.Move(fyne.NewPos(x, y))
		row.label.Resize(m)
	}
}

func (r *navListRenderer) MinSize() fyne.Size {
	w := float32(0)
	for _, row := range r.rows {
		w = max(w, row.label.MinSize().Width+theme.IconSize+3*theme.Space3)
	}
	return fyne.NewSize(w, float32(len(r.rows))*navRowHeight)
}

func (r *navListRenderer) Objects() []fyne.CanvasObject {
	objs := []fyne.CanvasObject{r.fill, r.wash}
	for _, row := range r.rows {
		if row.icon != nil {
			objs = append(objs, row.icon)
		}
		objs = append(objs, row.label)
	}
	return append(objs, r.ring)
}

func (r *navListRenderer) Destroy() {}
