package kit

import (
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"github.com/obstinix/PyxForge/internal/ui/theme"
)

// Tooltips (PXF-DEF-018). Fyne 2.8 has none, so a hovered IconButton shows its Label after a
// moment, in a layer of the window's content: a canvas overlay would take the keyboard focus
// manager, and typing in the editor while the pointer rests on a button would go nowhere.

// TooltipDelay is how long the pointer rests on a control before its tooltip shows.
const TooltipDelay = 600 * time.Millisecond

// Tooltips is a window's tooltip layer.
type Tooltips struct {
	layer *fyne.Container
	tip   *Glass
	text  *Text
	// Dispatch runs a delayed show on the UI thread; nil means fyne.Do. Delay 0 shows at once
	// (tests).
	Dispatch func(func())
	Delay    time.Duration

	mu    sync.Mutex
	owner fyne.CanvasObject // the control whose tooltip is pending or showing
}

// active is the window's tooltips, used by the controls that have one.
var active *Tooltips

// NewTooltips returns a tooltip layer and makes it the one controls use.
func NewTooltips() *Tooltips {
	t := &Tooltips{Delay: TooltipDelay, text: NewText("", Body, Primary)}
	t.tip = NewGlass(t.text)
	t.tip.Padding = theme.Space1
	t.tip.Hide()
	t.layer = container.NewWithoutLayout(t.tip)
	active = t
	return t
}

// Layer is the object to stack above the workspace, filling it.
func (t *Tooltips) Layer() fyne.CanvasObject { return t.layer }

// Text is the tooltip showing, or "".
func (t *Tooltips) Text() string {
	if !t.tip.Visible() {
		return ""
	}
	return t.text.Text
}

// hover schedules o's tooltip.
func (t *Tooltips) hover(o fyne.CanvasObject, label string) {
	t.mu.Lock()
	t.owner = o
	t.mu.Unlock()
	if t.Delay == 0 {
		t.show(o, label)
		return
	}
	time.AfterFunc(t.Delay, func() {
		do := t.Dispatch
		if do == nil {
			do = fyne.Do
		}
		do(func() { t.show(o, label) })
	})
}

// leave hides o's tooltip, or cancels it.
func (t *Tooltips) leave(o fyne.CanvasObject) {
	t.mu.Lock()
	mine := t.owner == o
	if mine {
		t.owner = nil
	}
	t.mu.Unlock()
	if mine {
		t.tip.Hide()
	}
}

// show puts the tooltip under o, inside the layer, if the pointer is still on o.
func (t *Tooltips) show(o fyne.CanvasObject, label string) {
	t.mu.Lock()
	still := t.owner == o
	t.mu.Unlock()
	if !still || !o.Visible() {
		return
	}
	d := fyne.CurrentApp().Driver()
	at := d.AbsolutePositionForObject(o).Subtract(d.AbsolutePositionForObject(t.layer))
	t.text.SetText(label)
	size := t.tip.MinSize()
	x := min(at.X, t.layer.Size().Width-size.Width-theme.Space1)
	y := at.Y + o.Size().Height + theme.Space1
	if y+size.Height > t.layer.Size().Height { // no room below: above
		y = at.Y - size.Height - theme.Space1
	}
	t.tip.Move(fyne.NewPos(max(x, 0), max(y, 0)))
	t.tip.Resize(size)
	t.tip.Show()
	t.layer.Refresh()
}
