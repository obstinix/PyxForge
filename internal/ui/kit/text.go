package kit

import (
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/widget"
	"github.com/obstinix/PyxForge/internal/ui/theme"
)

// Text is a single line of themed text.
type Text struct {
	widget.BaseWidget
	Text     string
	Role     Role
	Face     Face
	TextSize float32 // 0 means the face's default size
	Upper    bool    // uppercase, for panel titles and section labels only
	Align    fyne.TextAlign
}

// NewText returns themed text.
func NewText(s string, f Face, r Role) *Text {
	t := &Text{Text: s, Face: f, Role: r}
	t.ExtendBaseWidget(t)
	return t
}

// Title returns an uppercase Syne panel title (DESIGN_SYSTEM.md §7).
func Title(s string) *Text {
	t := NewText(s, Display, Secondary)
	t.Upper = true
	t.TextSize = theme.TextCaption + 1
	return t
}

// SetText changes the text and redraws it.
func (t *Text) SetText(s string) {
	t.Text = s
	t.Refresh()
}

func (t *Text) CreateRenderer() fyne.WidgetRenderer {
	r := &textRenderer{t: t, c: canvas.NewText("", nil)}
	r.Refresh()
	return r
}

type textRenderer struct {
	t *Text
	c *canvas.Text
}

func (r *textRenderer) Refresh() {
	tk := theme.Current()
	s := r.t.Text
	if r.t.Upper {
		s = strings.ToUpper(s)
	}
	r.c.Text = s
	r.c.Color = r.t.Role.color(tk)
	r.c.FontSource = r.t.Face.font()
	r.c.TextStyle = fyne.TextStyle{Monospace: r.t.Face == Mono}
	r.c.TextSize = r.t.TextSize
	if r.c.TextSize == 0 {
		r.c.TextSize = r.t.Face.size()
	}
	r.c.Alignment = r.t.Align
	r.c.Refresh()
}

func (r *textRenderer) Layout(s fyne.Size)           { r.c.Resize(s) }
func (r *textRenderer) MinSize() fyne.Size           { return r.c.MinSize() }
func (r *textRenderer) Objects() []fyne.CanvasObject { return []fyne.CanvasObject{r.c} }
func (r *textRenderer) Destroy()                     {}
