package shell

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"github.com/obstinix/PyxForge/internal/pty"
	"github.com/obstinix/PyxForge/internal/ui/icons"
	"github.com/obstinix/PyxForge/internal/ui/kit"
	"github.com/obstinix/PyxForge/internal/ui/theme"
)

// sessionRail is the terminal panel's list of sessions, on its right (identity prompt
// Phase 4a): a + button with a chevron for the kind of session, then one row per session with
// its kind's icon, its name, a status dot and, once it ended, its exit code and duration. A
// row's right-click menu renames, restarts, splits, closes and closes every exited session. In
// a panel narrower than 700 px it is 40 px of icons.
type sessionRail struct {
	t      *terminalHost
	box    *fyne.Container
	list   *fyne.Container
	head   *fyne.Container
	newBtn *kit.IconButton
	kinds  *kit.IconButton
	narrow bool
	rows   []*railRow
}

func newSessionRail(t *terminalHost) *sessionRail {
	r := &sessionRail{t: t}
	r.newBtn = kit.NewIconButton(icons.Plus, "New Terminal (Ctrl+Shift+`)", t.newSession)
	r.kinds = kit.NewIconButton(icons.ChevronDown, "New Session of a Kind…", r.kindMenu)
	for _, b := range []*kit.IconButton{r.newBtn, r.kinds} {
		b.Side = 28
	}
	title := kit.NewText("SESSIONS", kit.Label, kit.Tertiary)
	r.head = container.NewBorder(nil, nil, container.NewCenter(title), container.NewHBox(r.newBtn, r.kinds))
	r.list = container.NewVBox()
	bg := kit.NewSurface(kit.Raised, nil)
	r.box = container.NewStack(bg, container.New(layout.NewCustomPaddedLayout(theme.Space1, theme.Space1, theme.Space2, theme.Space1),
		container.NewBorder(r.head, nil, nil, nil, container.NewVScroll(r.list))))
	return r
}

// CreateRenderer-free: the rail is a container.
func (r *sessionRail) object() fyne.CanvasObject { return r.box }

// kindMenu offers the kinds of session: each shell found on the machine, and the machine's
// serial port and GDB console when they exist.
func (r *sessionRail) kindMenu() {
	t := r.t
	var items []*fyne.MenuItem
	for _, sh := range pty.Shells() {
		items = append(items, fyne.NewMenuItem(sh.Name, func() { t.newShellOf(sh) }))
	}
	for _, ts := range t.sessions {
		if ts.kind == kindSerial || ts.kind == kindGDB {
			items = append(items, fyne.NewMenuItem(ts.name, func() { t.selectSession(ts) }))
		}
	}
	c := t.s.win.Canvas()
	at := fyne.CurrentApp().Driver().AbsolutePositionForObject(r.kinds).AddXY(0, r.kinds.Size().Height)
	widget.ShowPopUpMenuAtPosition(fyne.NewMenu("", items...), c, at)
}

// setNarrow switches between the full rail and its icons.
func (r *sessionRail) setNarrow(n bool) {
	if n == r.narrow {
		return
	}
	r.narrow = n
	r.refresh()
}

// refresh rebuilds the rows from the sessions.
func (r *sessionRail) refresh() {
	if r == nil || r.list == nil {
		return
	}
	r.head.Objects[0].(*fyne.Container).Hidden = r.narrow // the title
	r.kinds.Hidden = r.narrow
	r.rows = r.rows[:0]
	var objs []fyne.CanvasObject
	for _, ts := range r.t.sessions {
		row := newRailRow(r, ts)
		r.rows = append(r.rows, row)
		objs = append(objs, row)
	}
	r.list.Objects = objs
	r.list.Refresh()
	r.head.Refresh()
}

// railRow is one session in the rail.
type railRow struct {
	widget.BaseWidget
	r       *sessionRail
	ts      *termSession
	hovered bool
}

func newRailRow(r *sessionRail, ts *termSession) *railRow {
	row := &railRow{r: r, ts: ts}
	row.ExtendBaseWidget(row)
	return row
}

func (row *railRow) Tapped(*fyne.PointEvent) { row.r.t.selectSession(row.ts) }

// TappedSecondary opens the session's menu.
func (row *railRow) TappedSecondary(e *fyne.PointEvent) {
	t := row.r.t
	ts := row.ts
	split := fyne.NewMenuItem("Split Right", func() { t.splitRight(ts) })
	split.Disabled = len(t.panes) >= 2
	m := fyne.NewMenu("",
		fyne.NewMenuItem("Rename…", func() { t.renameSession(ts) }),
		fyne.NewMenuItem("Restart", func() { t.restartSession(ts) }),
		split,
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("Close", func() { t.closeSession(ts) }),
		fyne.NewMenuItem("Close Exited", t.closeExited),
	)
	widget.ShowPopUpMenuAtPosition(m, t.s.win.Canvas(), e.AbsolutePosition)
}

func (row *railRow) MouseIn(*desktop.MouseEvent) {
	row.hovered = true
	row.Refresh()
	kit.Hover(row, row.ts.name+" · "+row.ts.kind.label()+" · "+sessionState(row.ts))
}
func (row *railRow) MouseMoved(*desktop.MouseEvent) {}
func (row *railRow) MouseOut() {
	row.hovered = false
	row.Refresh()
	kit.Unhover(row)
}

// shown reports whether a pane shows the row's session, and whether it is the active pane's.
func (row *railRow) shown() (bool, bool) {
	t := row.r.t
	for i, p := range t.panes {
		if p.session == row.ts {
			return true, i == t.active
		}
	}
	return false, false
}

// statusDot is a session's dot: the accent while it runs, success for exit 0, error for
// another status, tertiary for a console no longer attached.
func statusDot(t theme.Tokens, ts *termSession) color.NRGBA {
	switch {
	case !ts.done:
		return t.Accent.Primary
	case ts.kind == kindSerial || ts.kind == kindGDB:
		return t.Text.Tertiary
	case ts.status == 0:
		return t.Status.Success
	}
	return t.Status.Error
}

func (row *railRow) CreateRenderer() fyne.WidgetRenderer {
	rr := &railRowRenderer{row: row, bg: canvas.NewRectangle(color.Transparent), dot: canvas.NewCircle(color.Transparent),
		icon: kit.NewIcon(row.ts.kind.icon(), kit.Secondary), name: kit.NewText("", kit.Body, kit.Secondary),
		state: kit.NewText("", kit.Label, kit.Tertiary), mark: canvas.NewRectangle(color.Transparent)}
	rr.Refresh()
	return rr
}

type railRowRenderer struct {
	row         *railRow
	bg, mark    *canvas.Rectangle
	dot         *canvas.Circle
	icon        *kit.Icon
	name, state *kit.Text
}

const railRowH = 40

func (rr *railRowRenderer) Refresh() {
	t := theme.Current()
	row := rr.row
	shown, active := row.shown()
	rr.bg.FillColor = color.Transparent
	switch {
	case active:
		rr.bg.FillColor = t.State.Selected
	case row.hovered:
		rr.bg.FillColor = t.State.Hover
	}
	rr.bg.CornerRadius = theme.RadiusControl
	rr.mark.FillColor = color.Transparent
	if shown {
		rr.mark.FillColor = t.Accent.Primary
	}
	rr.dot.FillColor = statusDot(t, row.ts)
	role := kit.Secondary
	if active {
		role = kit.Primary
	}
	rr.icon.Role = role
	rr.name.Role = role
	rr.name.SetText(row.ts.name)
	rr.state.SetText("")
	if row.ts.done {
		rr.state.SetText(sessionState(row.ts))
	}
	rr.name.Hidden, rr.state.Hidden = row.r.narrow, row.r.narrow || !row.ts.done
	for _, o := range []fyne.CanvasObject{rr.bg, rr.mark, rr.dot, rr.icon, rr.name, rr.state} {
		o.Refresh()
	}
	rr.Layout(row.Size())
}

func (rr *railRowRenderer) Layout(s fyne.Size) {
	rr.bg.Move(fyne.NewPos(0, 2))
	rr.bg.Resize(fyne.NewSize(s.Width, s.Height-4))
	rr.mark.Move(fyne.NewPos(0, 10))
	rr.mark.Resize(fyne.NewSize(2, s.Height-20))
	is := theme.IconSize
	rr.icon.Move(fyne.NewPos(theme.Space2, (s.Height-is)/2))
	rr.icon.Resize(fyne.NewSize(is, is))
	d := float32(7)
	if rr.row.r.narrow {
		rr.dot.Move(fyne.NewPos(theme.Space2+is-2, (s.Height-is)/2-2))
	} else {
		rr.dot.Move(fyne.NewPos(s.Width-theme.Space2-d, (s.Height-d)/2))
	}
	rr.dot.Resize(fyne.NewSize(d, d))
	x := theme.Space2 + is + theme.Space2
	w := max(s.Width-x-theme.Space2-d-theme.Space1, 0)
	nh := rr.name.MinSize().Height
	if rr.row.ts.done {
		sh := rr.state.MinSize().Height
		top := (s.Height - nh - sh) / 2
		rr.name.Move(fyne.NewPos(x, top))
		rr.state.Move(fyne.NewPos(x, top+nh))
		rr.state.Resize(fyne.NewSize(w, sh))
	} else {
		rr.name.Move(fyne.NewPos(x, (s.Height-nh)/2))
	}
	rr.name.Resize(fyne.NewSize(w, nh))
}

func (rr *railRowRenderer) MinSize() fyne.Size {
	return fyne.NewSize(railNarrow-theme.Space1-theme.Space1, railRowH)
}

func (rr *railRowRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{rr.bg, rr.mark, rr.icon, rr.name, rr.state, rr.dot}
}

func (rr *railRowRenderer) Destroy() {}
