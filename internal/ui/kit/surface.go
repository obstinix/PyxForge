package kit

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/widget"
	"github.com/obstinix/PyxForge/internal/ui/theme"
)

// Surface paints one of the four surface levels behind its content. Panels are flush regions
// separated by Rules, never boxes inside boxes.
type Surface struct {
	widget.BaseWidget
	Role    SurfaceRole
	Content fyne.CanvasObject
}

// NewSurface wraps content in a surface.
func NewSurface(r SurfaceRole, content fyne.CanvasObject) *Surface {
	s := &Surface{Role: r, Content: content}
	s.ExtendBaseWidget(s)
	return s
}

func (s *Surface) CreateRenderer() fyne.WidgetRenderer {
	r := &surfaceRenderer{s: s, bg: canvas.NewRectangle(nil)}
	r.Refresh()
	return r
}

type surfaceRenderer struct {
	s  *Surface
	bg *canvas.Rectangle
}

func (r *surfaceRenderer) Refresh() {
	r.bg.FillColor = r.s.Role.color(theme.Current())
	r.bg.Refresh()
	if r.s.Content != nil {
		r.s.Content.Refresh()
	}
}

func (r *surfaceRenderer) Layout(sz fyne.Size) {
	r.bg.Resize(sz)
	if r.s.Content != nil {
		r.s.Content.Resize(sz)
	}
}

func (r *surfaceRenderer) MinSize() fyne.Size {
	if r.s.Content == nil {
		return fyne.Size{}
	}
	return r.s.Content.MinSize()
}

func (r *surfaceRenderer) Objects() []fyne.CanvasObject {
	if r.s.Content == nil {
		return []fyne.CanvasObject{r.bg}
	}
	return []fyne.CanvasObject{r.bg, r.s.Content}
}

func (r *surfaceRenderer) Destroy() {}

// Rule is a 1 px hairline divider.
type Rule struct {
	widget.BaseWidget
	Vertical bool
	Strong   bool
}

// NewRule returns a horizontal or vertical hairline.
func NewRule(vertical bool) *Rule {
	r := &Rule{Vertical: vertical}
	r.ExtendBaseWidget(r)
	return r
}

func (r *Rule) CreateRenderer() fyne.WidgetRenderer {
	rr := &ruleRenderer{r: r, line: canvas.NewRectangle(nil)}
	rr.Refresh()
	return rr
}

type ruleRenderer struct {
	r    *Rule
	line *canvas.Rectangle
}

func (rr *ruleRenderer) Refresh() {
	t := theme.Current()
	rr.line.FillColor = t.Border.Hairline
	if rr.r.Strong {
		rr.line.FillColor = t.Border.Strong
	}
	rr.line.Refresh()
}

func (rr *ruleRenderer) Layout(s fyne.Size) { rr.line.Resize(s) }

func (rr *ruleRenderer) MinSize() fyne.Size {
	if rr.r.Vertical {
		return fyne.NewSize(1, 0)
	}
	return fyne.NewSize(0, 1)
}

func (rr *ruleRenderer) Objects() []fyne.CanvasObject { return []fyne.CanvasObject{rr.line} }
func (rr *ruleRenderer) Destroy()                     {}
