package shell

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
	"github.com/obstinix/PyxForge/internal/ui/kit"
	"github.com/obstinix/PyxForge/internal/ui/theme"
)

// settingsView is Settings → Appearance (D2: theme, accent, System, previews).
func (s *Shell) settingsView() fyne.CanvasObject {
	var cards []fyne.CanvasObject
	cards = append(cards, newThemeCard(s, theme.SystemID, "System", "follows the OS"))
	for _, p := range theme.Palettes {
		pol := "dark"
		if p.Polarity == theme.Light {
			pol = "light"
		}
		cards = append(cards, newThemeCard(s, p.ID, p.Name, pol))
	}
	var accents []fyne.CanvasObject
	for _, a := range theme.Accents {
		accents = append(accents, newAccentCard(s, a))
	}
	glass := widget.NewCheck("Glass overlays", s.setGlass)
	glass.SetChecked(s.sel.Glass)
	s.onAppearance(func() {
		if glass.Checked != s.sel.Glass {
			glass.SetChecked(s.sel.Glass)
		}
	})

	section := func(title string) fyne.CanvasObject {
		t := kit.NewText(title, kit.Label, kit.Tertiary)
		t.Upper = true
		return container.NewVBox(widget.NewSeparator(), t)
	}
	display := kit.NewText("Panel title in Syne", kit.Display, kit.Primary)
	display.Upper = true
	display.TextSize = theme.TextLarge
	body := container.NewVBox(
		kit.NewText("Appearance", kit.Heading, kit.Primary),
		kit.NewText("Changes apply immediately and are saved on this computer.", kit.Body, kit.Secondary),
		section("Theme"),
		container.NewGridWrap(fyne.NewSize(240, 168), cards...),
		kit.NewText("System follows the operating system: light uses Ink & Paper, dark uses Smoked Kraft.", kit.Body, kit.Tertiary),
		section("Accent"),
		container.NewHBox(accents...),
		section("Overlays"),
		glass,
		kit.NewText("Palette, dialogs, notifications and the floating inspector blur what is behind them.", kit.Body, kit.Tertiary),
		section("Type"),
		display,
		kit.NewText("Interface text in Geist, 13 px. Labels, menus, tree rows.", kit.Body, kit.Primary),
		kit.NewText("mov ax, 0x7c00    ; JetBrains Mono for code and data", kit.Mono, kit.Primary),
	)
	return container.NewVScroll(container.NewPadded(container.NewPadded(body)))
}

// themeCard previews one palette with the current accent, drawn from its real tokens.
type themeCard struct {
	widget.BaseWidget
	s           *Shell
	id          string
	name, note  string
	hovered     bool
	frame       *canvas.Rectangle
	preview     *fyne.Container
	title, desc *kit.Text
}

func newThemeCard(s *Shell, id, name, note string) *themeCard {
	c := &themeCard{s: s, id: id, name: name, note: note, frame: canvas.NewRectangle(nil),
		preview: container.NewWithoutLayout(), title: kit.NewText(name, kit.Strong, kit.Primary),
		desc: kit.NewText(note, kit.Label, kit.Tertiary)}
	c.ExtendBaseWidget(c)
	s.onAppearance(c.Refresh)
	return c
}

func (c *themeCard) Tapped(*fyne.PointEvent) {
	sel := c.s.Selection()
	sel.PaletteID = c.id
	c.s.SetSelection(sel)
}
func (c *themeCard) MouseIn(*desktop.MouseEvent)    { c.hovered = true; c.Refresh() }
func (c *themeCard) MouseMoved(*desktop.MouseEvent) {}
func (c *themeCard) MouseOut()                      { c.hovered = false; c.Refresh() }

func (c *themeCard) CreateRenderer() fyne.WidgetRenderer {
	r := &themeCardRenderer{c: c}
	r.Refresh()
	return r
}

type themeCardRenderer struct{ c *themeCard }

const previewH = 112

func (r *themeCardRenderer) Refresh() {
	c := r.c
	cur := theme.Current()
	selected := c.s.Selection().PaletteID == c.id
	c.frame.CornerRadius = theme.RadiusPanel
	c.frame.FillColor = cur.Surface.Raised
	c.frame.StrokeColor, c.frame.StrokeWidth = cur.Border.Hairline, 1
	if c.hovered {
		c.frame.StrokeColor = cur.Border.Strong
	}
	if selected {
		c.frame.StrokeColor, c.frame.StrokeWidth = cur.Accent.Primary, 2
	}
	c.frame.Refresh()

	accent, _ := theme.AccentByID(c.s.Selection().AccentID)
	if c.id == theme.SystemID {
		c.preview.Objects = append(miniature(theme.Resolve(theme.SmokedKraft, accent), 0, 112),
			miniature(theme.Resolve(theme.InkPaper, accent), 112, 112)...)
	} else {
		p, _ := theme.PaletteByID(c.id)
		c.preview.Objects = miniature(theme.Resolve(p, accent), 0, 224)
	}
	c.preview.Refresh()
	c.title.Refresh()
	c.desc.Refresh()
}

func (r *themeCardRenderer) Layout(s fyne.Size) {
	c := r.c
	c.frame.Resize(s)
	c.preview.Move(fyne.NewPos(theme.Space2, theme.Space2))
	c.preview.Resize(fyne.NewSize(224, previewH))
	c.title.Move(fyne.NewPos(theme.Space2, theme.Space2+previewH+theme.Space2))
	c.title.Resize(c.title.MinSize())
	c.desc.Move(fyne.NewPos(theme.Space2, c.title.Position().Y+c.title.MinSize().Height))
	c.desc.Resize(c.desc.MinSize())
}

func (r *themeCardRenderer) MinSize() fyne.Size { return fyne.NewSize(240, 168) }
func (r *themeCardRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.c.frame, r.c.preview, r.c.title, r.c.desc}
}
func (r *themeCardRenderer) Destroy() {}

// miniature draws a workspace from tokens: rail, explorer with a selected row, editor with
// syntax-coloured lines and a selection band, a terminal strip and a glass overlay chip. It is
// laid out on a 224 px wide grid and scaled horizontally to w.
func miniature(t theme.Tokens, x, w float32) []fyne.CanvasObject {
	k := w / 224
	rect := func(c color.Color, px, py, pw, ph float32) fyne.CanvasObject {
		r := canvas.NewRectangle(c)
		r.Move(fyne.NewPos(x+px*k, py))
		r.Resize(fyne.NewSize(max(pw*k, 1), ph))
		return r
	}
	w = 224 // the grid every element below is placed on
	sx := t.Syntax
	editorX := float32(62)
	out := []fyne.CanvasObject{
		rect(t.Surface.Base, 0, 0, w, previewH),
		rect(t.Surface.Raised, 0, 0, 12, previewH),            // rail
		rect(t.Border.Hairline, 12, 0, 1, previewH),           // rail rule
		rect(t.Border.Hairline, editorX-1, 0, 1, previewH-26), // explorer rule
		rect(t.Text.Secondary, 18, 10, 30, 3),                 // explorer rows
		rect(theme.Over(t.State.Selected, t.Surface.Base), 13, 18, editorX-14, 9),
		rect(t.Accent.Primary, 13, 18, 2, 9), // selected-row marker
		rect(t.Text.Primary, 18, 21, 26, 3),
		rect(t.Text.Secondary, 18, 32, 34, 3),
		rect(t.Text.Tertiary, 18, 42, 22, 3),
		rect(sx.Keyword, editorX+8, 10, 18, 3), // code lines
		rect(sx.Function, editorX+30, 10, 34, 3),
		rect(sx.Comment, editorX+8, 20, 70, 3),
		rect(theme.Over(t.Accent.Selection, t.Surface.Base), editorX+4, 27, w-editorX-8, 9),
		rect(sx.Variable, editorX+8, 30, 24, 3),
		rect(sx.String, editorX+36, 30, 40, 3),
		rect(sx.Number, editorX+8, 40, 16, 3),
		rect(sx.Operator, editorX+28, 40, 8, 3),
		rect(sx.Type, editorX+40, 40, 28, 3),
		rect(t.Border.Hairline, 13, previewH-26, w-13, 1), // dock rule
		rect(t.Surface.Sunken, 13, previewH-25, w-13, 25), // terminal
		rect(t.Text.Secondary, 20, previewH-16, 60, 3),
		rect(t.Status.Success, 84, previewH-16, 14, 3),
	}
	if k == 1 { // the glass chip only fits the full-width preview
		glass := canvas.NewRectangle(t.Glass.Fill)
		glass.StrokeColor, glass.StrokeWidth, glass.CornerRadius = t.Glass.Rim, 1, 3
		glass.Move(fyne.NewPos(x+w-70, 48))
		glass.Resize(fyne.NewSize(62, 30))
		out = append(out, glass,
			rect(t.Text.Primary, w-64, 56, 34, 3),
			rect(t.Accent.Primary, w-64, 66, 20, 3))
	}
	return out
}

// accentCard shows one accent's dark and light values.
type accentCard struct {
	widget.BaseWidget
	s       *Shell
	a       theme.Accent
	hovered bool
}

func newAccentCard(s *Shell, a theme.Accent) *accentCard {
	c := &accentCard{s: s, a: a}
	c.ExtendBaseWidget(c)
	s.onAppearance(c.Refresh)
	return c
}

func (c *accentCard) Tapped(*fyne.PointEvent) {
	sel := c.s.Selection()
	sel.AccentID = c.a.ID
	c.s.SetSelection(sel)
}
func (c *accentCard) MouseIn(*desktop.MouseEvent)    { c.hovered = true; c.Refresh() }
func (c *accentCard) MouseMoved(*desktop.MouseEvent) {}
func (c *accentCard) MouseOut()                      { c.hovered = false; c.Refresh() }

func (c *accentCard) CreateRenderer() fyne.WidgetRenderer {
	frame := canvas.NewRectangle(nil)
	dark, light := canvas.NewCircle(c.a.Dark.Primary), canvas.NewCircle(c.a.Light.Primary)
	name := kit.NewText(c.a.Name, kit.Strong, kit.Primary)
	r := &accentCardRenderer{c: c, frame: frame, dark: dark, light: light, name: name}
	r.Refresh()
	return r
}

type accentCardRenderer struct {
	c           *accentCard
	frame       *canvas.Rectangle
	dark, light *canvas.Circle
	name        *kit.Text
}

func (r *accentCardRenderer) Refresh() {
	cur := theme.Current()
	r.frame.CornerRadius = theme.RadiusPanel
	r.frame.FillColor = cur.Surface.Raised
	r.frame.StrokeColor, r.frame.StrokeWidth = cur.Border.Hairline, 1
	if r.c.hovered {
		r.frame.StrokeColor = cur.Border.Strong
	}
	if r.c.s.Selection().AccentID == r.c.a.ID {
		r.frame.StrokeColor, r.frame.StrokeWidth = cur.Accent.Primary, 2
	}
	r.frame.Refresh()
	r.name.Refresh()
}

func (r *accentCardRenderer) Layout(s fyne.Size) {
	r.frame.Resize(s)
	d := float32(20)
	r.dark.Move(fyne.NewPos(theme.Space3, (s.Height-d)/2))
	r.dark.Resize(fyne.NewSquareSize(d))
	r.light.Move(fyne.NewPos(theme.Space3+d+theme.Space1, (s.Height-d)/2))
	r.light.Resize(fyne.NewSquareSize(d))
	m := r.name.MinSize()
	r.name.Move(fyne.NewPos(theme.Space3+2*d+theme.Space3, (s.Height-m.Height)/2))
	r.name.Resize(m)
}

func (r *accentCardRenderer) MinSize() fyne.Size { return fyne.NewSize(176, 48) }
func (r *accentCardRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.frame, r.dark, r.light, r.name}
}
func (r *accentCardRenderer) Destroy() {}
