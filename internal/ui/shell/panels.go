package shell

import (
	"image/color"

	"os"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	fynetheme "fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/obstinix/PyxForge/internal/ui/icons"
	"github.com/obstinix/PyxForge/internal/ui/kit"
	"github.com/obstinix/PyxForge/internal/ui/theme"
)

// header is a panel title row on the tab-bar grid: a Syne title and optional actions.
func header(title string, actions ...fyne.CanvasObject) fyne.CanvasObject {
	row := container.NewHBox(kit.Title(title), layout.NewSpacer())
	for _, a := range actions {
		row.Add(a)
	}
	return container.NewVBox(kit.Row(theme.TabBarHeight, row), kit.NewRule(false))
}

// placeholder is the labelled empty state for a surface whose subsystem has not been built.
// It says what will appear here and when, so it can never be mistaken for a working panel.
func placeholder(icon icons.Name, title, detail string) fyne.CanvasObject {
	i := kit.NewIcon(icon, kit.Tertiary)
	i.IconSize = theme.IconSizeLarge
	t := kit.NewText(title, kit.Strong, kit.Secondary)
	t.Align = fyne.TextAlignCenter
	d := widget.NewRichText(&widget.TextSegment{Text: detail, Style: widget.RichTextStyle{
		Alignment: fyne.TextAlignCenter, ColorName: fynetheme.ColorNamePlaceHolder}})
	d.Wrapping = fyne.TextWrapWord
	return container.New(&column{max: 360}, container.NewVBox(i, t, d))
}

// column centres one object in a column at most max wide, so wrapped text keeps a readable
// measure in wide panels and still fits narrow ones.
type column struct{ max float32 }

func (l *column) Layout(objs []fyne.CanvasObject, s fyne.Size) {
	w := min(l.max, s.Width-2*theme.Space4)
	o := objs[0]
	o.Resize(fyne.NewSize(w, o.MinSize().Height)) // let wrapped text settle at this width
	h := o.MinSize().Height
	o.Resize(fyne.NewSize(w, h))
	o.Move(fyne.NewPos((s.Width-w)/2, (s.Height-h)/2))
}

func (l *column) MinSize(objs []fyne.CanvasObject) fyne.Size {
	return fyne.NewSize(120, objs[0].MinSize().Height)
}

// quietTabs draws tab labels and their underline in the text colour rather than the accent,
// keeping the accent for focus, the active rail item, selection and primary actions
// (DESIGN_SYSTEM.md §1 and §12).
type quietTabs struct{}

func (quietTabs) current() fyne.Theme { return fyne.CurrentApp().Settings().Theme() }

func (q quietTabs) Color(n fyne.ThemeColorName, v fyne.ThemeVariant) color.Color {
	if n == fynetheme.ColorNamePrimary {
		n = fynetheme.ColorNameForeground
	}
	return q.current().Color(n, v)
}
func (q quietTabs) Font(s fyne.TextStyle) fyne.Resource     { return q.current().Font(s) }
func (q quietTabs) Icon(n fyne.ThemeIconName) fyne.Resource { return q.current().Icon(n) }
func (q quietTabs) Size(n fyne.ThemeSizeName) float32       { return q.current().Size(n) }
func quiet(tabs fyne.CanvasObject) fyne.CanvasObject {
	return container.NewThemeOverride(tabs, quietTabs{})
}

// sidePanel is the inspector's frame. Docked, it is a flush surface with a leading hairline;
// below DockBreakpoint it floats as glass over the editor (Section 11.6). One widget, two
// appearances, so nothing is re-parented when the window is resized.
type sidePanel struct {
	widget.BaseWidget
	content  fyne.CanvasObject
	floating bool
}

func newSidePanel(content fyne.CanvasObject) *sidePanel {
	p := &sidePanel{content: content}
	p.ExtendBaseWidget(p)
	return p
}

func (p *sidePanel) CreateRenderer() fyne.WidgetRenderer {
	r := &sidePanelRenderer{p: p, body: canvas.NewRectangle(nil), edge: canvas.NewRectangle(nil)}
	r.Refresh()
	return r
}

type sidePanelRenderer struct {
	p          *sidePanel
	body, edge *canvas.Rectangle
}

func (r *sidePanelRenderer) Refresh() {
	t := theme.Current()
	if r.p.floating {
		g := t.Glass
		r.body.FillColor, r.body.StrokeColor, r.body.StrokeWidth = g.Fill, g.Rim, 1
		r.body.CornerRadius = theme.RadiusOverlay
		r.body.Shadow = canvas.Shadow{Color: g.Shadow, BlurRadius: g.ShadowBlur,
			Offset: fyne.NewPos(0, g.ShadowOffsetY), Variant: canvas.DropShadow}
		r.edge.FillColor = g.Edge
	} else {
		r.body.FillColor, r.body.StrokeColor, r.body.StrokeWidth = t.Surface.Base, color.Transparent, 0
		r.body.CornerRadius = 0
		r.body.Shadow = canvas.Shadow{}
		r.edge.FillColor = t.Border.Hairline
	}
	r.body.Refresh()
	r.edge.Refresh()
	r.p.content.Refresh()
}

func (r *sidePanelRenderer) Layout(s fyne.Size) {
	r.body.Resize(s)
	if r.p.floating {
		// Specular line along the top edge; content inset inside the rim.
		r.edge.Move(fyne.NewPos(theme.RadiusOverlay, 1))
		r.edge.Resize(fyne.NewSize(s.Width-2*theme.RadiusOverlay, 1))
		r.p.content.Move(fyne.NewPos(theme.Space1, theme.Space1))
		r.p.content.Resize(fyne.NewSize(s.Width-2*theme.Space1, s.Height-2*theme.Space1))
		return
	}
	// Docked: a hairline on the leading edge, content flush beside it.
	r.edge.Move(fyne.NewPos(0, 0))
	r.edge.Resize(fyne.NewSize(1, s.Height))
	r.p.content.Move(fyne.NewPos(1, 0))
	r.p.content.Resize(fyne.NewSize(s.Width-1, s.Height))
}

func (r *sidePanelRenderer) MinSize() fyne.Size { return r.p.content.MinSize() }
func (r *sidePanelRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.body, r.p.content, r.edge}
}
func (r *sidePanelRenderer) Destroy() {}

// statusItem is a compact, optionally clickable status-bar entry.
type statusItem struct {
	widget.BaseWidget
	icon    *kit.Icon
	text    *kit.Text
	onTap   func()
	hovered bool
}

func newStatusItem(icon icons.Name, text string, onTap func()) *statusItem {
	s := &statusItem{text: kit.NewText(text, kit.Body, kit.Secondary), onTap: onTap}
	s.text.TextSize = theme.TextCaption + 1
	if icon != "" {
		s.icon = kit.NewIcon(icon, kit.Secondary)
		s.icon.IconSize = 14
	}
	s.ExtendBaseWidget(s)
	return s
}

func (s *statusItem) SetText(t string) { s.text.SetText(t) }

func (s *statusItem) Tapped(*fyne.PointEvent) {
	if s.onTap != nil {
		s.onTap()
	}
}

func (s *statusItem) MouseIn(*desktop.MouseEvent) {
	if s.onTap != nil {
		s.hovered = true
		s.Refresh()
	}
}
func (s *statusItem) MouseMoved(*desktop.MouseEvent) {}
func (s *statusItem) MouseOut()                      { s.hovered = false; s.Refresh() }

func (s *statusItem) CreateRenderer() fyne.WidgetRenderer {
	wash := canvas.NewRectangle(nil)
	row := container.NewHBox(s.text)
	if s.icon != nil {
		row.Objects = append([]fyne.CanvasObject{s.icon}, row.Objects...)
	}
	r := &statusItemRenderer{s: s, wash: wash, row: row}
	r.Refresh()
	return r
}

type statusItemRenderer struct {
	s    *statusItem
	wash *canvas.Rectangle
	row  *fyne.Container
}

func (r *statusItemRenderer) Refresh() {
	r.wash.FillColor = color.Transparent
	if r.s.hovered {
		r.wash.FillColor = theme.Current().State.Hover
	}
	r.wash.Refresh()
	r.row.Refresh()
}

func (r *statusItemRenderer) Layout(s fyne.Size) {
	r.wash.Resize(s)
	m := r.row.MinSize()
	r.row.Resize(m)
	r.row.Move(fyne.NewPos(theme.Space2, (s.Height-m.Height)/2))
}

func (r *statusItemRenderer) MinSize() fyne.Size {
	m := r.row.MinSize()
	return fyne.NewSize(m.Width+2*theme.Space2, theme.StatusBarHeight)
}

func (r *statusItemRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.wash, r.row}
}
func (r *statusItemRenderer) Destroy() {}

// gitBranch reads the checked-out branch from root's .git without running git. ok is false
// when root is not a repository. A detached HEAD returns its short hash.
func gitBranch(root string) (string, bool) {
	gitPath := filepath.Join(root, ".git")
	if fi, err := os.Stat(gitPath); err == nil && !fi.IsDir() {
		// A worktree or submodule: .git is a file pointing at the real git directory.
		b, err := os.ReadFile(gitPath)
		if err != nil {
			return "", false
		}
		dir := strings.TrimSpace(strings.TrimPrefix(string(b), "gitdir:"))
		if !filepath.IsAbs(dir) {
			dir = filepath.Join(root, dir)
		}
		gitPath = dir
	}
	head, err := os.ReadFile(filepath.Join(gitPath, "HEAD"))
	if err != nil {
		return "", false
	}
	h := strings.TrimSpace(string(head))
	if ref, ok := strings.CutPrefix(h, "ref: refs/heads/"); ok {
		return ref, true
	}
	if len(h) >= 7 {
		return h[:7], true
	}
	return "", false
}
