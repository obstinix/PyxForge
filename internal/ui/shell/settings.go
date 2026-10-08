package shell

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
	"github.com/obstinix/PyxForge/internal/ui/icons"
	"github.com/obstinix/PyxForge/internal/ui/kit"
	"github.com/obstinix/PyxForge/internal/ui/theme"
)

// settingsView is Settings → Appearance (D2: theme, accent, System, previews).
// Every card and the glass check are keyboard-focusable: Tab moves between them, Space or
// Enter applies.
func (s *Shell) settingsView() fyne.CanvasObject {
	s.themeCards = []*themeCard{newThemeCard(s, theme.SystemID, "System", "follows the OS")}
	for _, p := range theme.Palettes {
		pol := "dark"
		if p.Polarity == theme.Light {
			pol = "light"
		}
		s.themeCards = append(s.themeCards, newThemeCard(s, p.ID, p.Name, pol))
	}
	var cards, accents []fyne.CanvasObject
	for _, c := range s.themeCards {
		cards = append(cards, c)
	}
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

// themeCard previews one palette with the current accent, drawn from its real tokens. The
// current theme carries a check badge; keyboard focus draws the focus ring. The two never look
// alike (Phase 1 review: an accent stroke for "selected" read as a focus ring).
type themeCard struct {
	widget.BaseWidget
	s                *Shell
	id               string
	name, note       string
	hovered, focused bool
	frame, ring      *canvas.Rectangle
	preview          *fyne.Container
	title, desc      *kit.Text
	badge            *selectedBadge
}

func newThemeCard(s *Shell, id, name, note string) *themeCard {
	c := &themeCard{s: s, id: id, name: name, note: note, frame: canvas.NewRectangle(nil),
		ring: kit.NewFocusRing(), preview: container.NewWithoutLayout(),
		title: kit.NewText(name, kit.Strong, kit.Primary), desc: kit.NewText(note, kit.Label, kit.Tertiary),
		badge: newSelectedBadge()}
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

func (c *themeCard) FocusGained() {
	c.focused = true
	c.Refresh()
	c.s.activate(regionEditor)
}
func (c *themeCard) FocusLost()                { c.focused = false; c.Refresh() }
func (c *themeCard) TypedRune(r rune)          { activateKey(r, nil, c.Tapped) }
func (c *themeCard) TypedKey(e *fyne.KeyEvent) { activateKey(0, e, c.Tapped) }

// activateKey runs tap for Space, Enter or Return: how every PyxForge control activates from the
// keyboard (DESIGN_SYSTEM.md §12).
func activateKey(r rune, e *fyne.KeyEvent, tap func(*fyne.PointEvent)) {
	if r == ' ' || (e != nil && (e.Name == fyne.KeyReturn || e.Name == fyne.KeyEnter)) {
		tap(nil)
	}
}

func (c *themeCard) CreateRenderer() fyne.WidgetRenderer {
	r := &themeCardRenderer{c: c}
	r.Refresh()
	return r
}

type themeCardRenderer struct{ c *themeCard }

const previewH = 112

func (r *themeCardRenderer) Refresh() {
	c := r.c
	selected := c.s.Selection().PaletteID == c.id
	styleCardFrame(c.frame, c.hovered || selected)
	c.badge.show(selected)
	kit.StyleFocusRing(c.ring, c.focused)

	sel := c.s.Selection()
	accent, _ := theme.AccentByID(sel.AccentID)
	resolve := func(p theme.Palette) theme.Tokens { return theme.Resolve(p, accent).WithGlass(sel.Glass) }
	if c.id == theme.SystemID {
		c.preview.Objects = append(miniature(resolve(theme.SmokedKraft), 0, 112),
			miniature(resolve(theme.InkPaper), 112, 112)...)
	} else {
		p, _ := theme.PaletteByID(c.id)
		c.preview.Objects = miniature(resolve(p), 0, 224)
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
	c.badge.place(s.Width-theme.Space2-badgeSize, c.title.Position().Y+theme.Space1)
	c.ring.Resize(s)
}

func (r *themeCardRenderer) MinSize() fyne.Size { return fyne.NewSize(240, 168) }
func (r *themeCardRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.c.frame, r.c.preview, r.c.title, r.c.desc, r.c.badge.disc, r.c.badge.check, r.c.ring}
}

// styleCardFrame is the shared card body: Raised fill, hairline, a stronger line on hover and
// for the current choice.
func styleCardFrame(f *canvas.Rectangle, strong bool) {
	cur := theme.Current()
	f.CornerRadius = theme.RadiusPanel
	f.FillColor = cur.Surface.Raised
	f.StrokeColor, f.StrokeWidth = cur.Border.Hairline, 1
	if strong {
		f.StrokeColor = cur.Border.Strong
	}
	f.Refresh()
}

const badgeSize = 18

// selectedBadge marks the current choice: a Text.Primary disc with a check. It is neutral,
// so the accent stays on the focused region's tab, and it is a shape, not a stroke, so it
// cannot be mistaken for the focus ring.
type selectedBadge struct {
	disc  *canvas.Circle
	check *canvas.Image
}

func newSelectedBadge() *selectedBadge {
	b := &selectedBadge{disc: canvas.NewCircle(color.Transparent), check: canvas.NewImageFromResource(nil)}
	b.check.FillMode = canvas.ImageFillContain
	return b
}

func (b *selectedBadge) show(on bool) {
	cur := theme.Current()
	b.disc.FillColor = cur.Text.Primary
	b.check.Resource = icons.Get(icons.Check, cur.Surface.Base)
	b.disc.Hidden, b.check.Hidden = !on, !on
	b.disc.Refresh()
	b.check.Refresh()
}

func (b *selectedBadge) place(x, y float32) {
	b.disc.Move(fyne.NewPos(x, y))
	b.disc.Resize(fyne.NewSquareSize(badgeSize))
	b.check.Move(fyne.NewPos(x+3, y+3))
	b.check.Resize(fyne.NewSquareSize(badgeSize - 6))
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

// accentCard shows one accent's dark and light values, with the same badge and focus ring as
// the theme cards.
type accentCard struct {
	widget.BaseWidget
	s                *Shell
	a                theme.Accent
	hovered, focused bool
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

func (c *accentCard) FocusGained() {
	c.focused = true
	c.Refresh()
	c.s.activate(regionEditor)
}
func (c *accentCard) FocusLost()                { c.focused = false; c.Refresh() }
func (c *accentCard) TypedRune(r rune)          { activateKey(r, nil, c.Tapped) }
func (c *accentCard) TypedKey(e *fyne.KeyEvent) { activateKey(0, e, c.Tapped) }

func (c *accentCard) CreateRenderer() fyne.WidgetRenderer {
	frame := canvas.NewRectangle(nil)
	dark, light := canvas.NewCircle(c.a.Dark.Primary), canvas.NewCircle(c.a.Light.Primary)
	name := kit.NewText(c.a.Name, kit.Strong, kit.Primary)
	r := &accentCardRenderer{c: c, frame: frame, dark: dark, light: light, name: name,
		badge: newSelectedBadge(), ring: kit.NewFocusRing()}
	r.Refresh()
	return r
}

type accentCardRenderer struct {
	c           *accentCard
	frame, ring *canvas.Rectangle
	dark, light *canvas.Circle
	name        *kit.Text
	badge       *selectedBadge
}

func (r *accentCardRenderer) Refresh() {
	selected := r.c.s.Selection().AccentID == r.c.a.ID
	styleCardFrame(r.frame, r.c.hovered || selected)
	r.badge.show(selected)
	kit.StyleFocusRing(r.ring, r.c.focused)
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
	r.badge.place(s.Width-theme.Space3-badgeSize, (s.Height-badgeSize)/2)
	r.ring.Resize(s)
}

func (r *accentCardRenderer) MinSize() fyne.Size { return fyne.NewSize(176, 48) }
func (r *accentCardRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.frame, r.dark, r.light, r.name, r.badge.disc, r.badge.check, r.ring}
}
func (r *accentCardRenderer) Destroy() {}
