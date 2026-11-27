package kit

import (
	"image/color"
	"net/url"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	fynetheme "fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/obstinix/PyxForge/internal/ui/theme"
)

// Defer runs f after the current event or layout pass, on the UI thread. The shell sets it to
// its dispatcher; the default is fyne.Do.
var Defer = func(f func()) { fyne.Do(f) }

// SettingRow is one setting: its title and a description on the left, the control on the
// right, both top aligned, 16 px above and below (Obsidian's settings rows). Keywords are extra
// words a search should find it by.
type SettingRow struct {
	widget.BaseWidget
	Title       string
	Description string
	Keywords    []string
	Control     fyne.CanvasObject // nil for a row that only explains

	// Link, when set, is one link drawn under the description; OnLink runs when it is chosen.
	Link   string
	OnLink func()

	highlight float32 // 0…1: the wash a search result's jump draws, fading
	anim      *fyne.Animation
}

// NewSettingRow returns a row.
func NewSettingRow(title, description string, control fyne.CanvasObject) *SettingRow {
	r := &SettingRow{Title: title, Description: description, Control: control}
	r.ExtendBaseWidget(r)
	return r
}

// WithKeywords adds search words and returns the row.
func (r *SettingRow) WithKeywords(words ...string) *SettingRow {
	r.Keywords = append(r.Keywords, words...)
	return r
}

// WithLink adds the row's one link and returns the row.
func (r *SettingRow) WithLink(text string, open func()) *SettingRow {
	r.Link, r.OnLink = text, open
	return r
}

// Matches reports whether every word of query appears in the title, the description or a
// keyword, ignoring case. An empty query matches nothing.
func (r *SettingRow) Matches(query string) bool {
	words := strings.Fields(strings.ToLower(query))
	if len(words) == 0 {
		return false
	}
	hay := strings.ToLower(r.Title + " " + r.Description + " " + strings.Join(r.Keywords, " "))
	for _, w := range words {
		if !strings.Contains(hay, w) {
			return false
		}
	}
	return true
}

// Flash draws a brief highlight over the row (where a search result jumped to); under Reduce
// Motion nothing is drawn.
func (r *SettingRow) Flash() {
	if ReduceMotion() || fyne.CurrentApp() == nil {
		return
	}
	if r.anim != nil {
		r.anim.Stop()
	}
	r.anim = fyne.NewAnimation(900*time.Millisecond, func(p float32) {
		r.highlight = 1 - p
		r.Refresh()
	})
	r.anim.Start()
}

func (r *SettingRow) CreateRenderer() fyne.WidgetRenderer {
	rr := &settingRowRenderer{r: r, wash: canvas.NewRectangle(color.Transparent),
		title: NewText(r.Title, Body, Primary)}
	// Fyne draws a RichText's (and a Hyperlink's) text inside the theme's inner padding, so the
	// title is set in by as much: all three share a left edge (#51).
	left := []fyne.CanvasObject{container.New(layout.NewCustomPaddedLayout(0, 0, theme.Space2, 0), rr.title)}
	if r.Description != "" {
		rr.desc = widget.NewRichText(&widget.TextSegment{Text: r.Description,
			Style: widget.RichTextStyle{ColorName: theme.ColorNameTextSecondary, Inline: true,
				SizeName: fynetheme.SizeNameCaptionText}})
		rr.desc.Wrapping = fyne.TextWrapWord
		left = append(left, rr.desc)
	}
	if r.Link != "" {
		link := widget.NewHyperlink(r.Link, &url.URL{})
		link.OnTapped = func() {
			if r.OnLink != nil {
				r.OnLink()
			}
		}
		left = append(left, link)
	}
	text := container.New(layout.NewCustomPaddedVBoxLayout(theme.Space1), left...)
	var right fyne.CanvasObject = layout.NewSpacer()
	if r.Control != nil {
		right = container.NewVBox(r.Control) // top aligned
	}
	rr.body = container.New(layout.NewCustomPaddedLayout(theme.Space4, theme.Space4, theme.Space4, theme.Space4),
		container.New(&rowSplit{}, text, right))
	rr.Refresh()
	return rr
}

type settingRowRenderer struct {
	r               *SettingRow
	relayoutPending bool
	laidWidth       float32
	wash            *canvas.Rectangle
	title           *Text
	desc            *widget.RichText
	body            *fyne.Container
}

func (rr *settingRowRenderer) Refresh() {
	t := theme.Current()
	rr.wash.FillColor = color.Transparent
	if rr.r.highlight > 0 {
		c := t.Accent.Selection
		c.A = uint8(float32(c.A) * rr.r.highlight)
		rr.wash.FillColor = c
	}
	rr.wash.CornerRadius = theme.RadiusControl
	rr.wash.Refresh()
	rr.title.SetText(rr.r.Title)
	if rr.desc != nil {
		rr.desc.Refresh()
	}
}

func (rr *settingRowRenderer) Layout(s fyne.Size) {
	rr.wash.Resize(s)
	rr.body.Resize(s)
	if s.Width != rr.laidWidth && rr.desc != nil {
		// RichText caches its size until it is refreshed, not when it is resized: refresh it at
		// its new width, so the height below is the wrapped one.
		rr.laidWidth = s.Width
		rr.desc.Refresh()
		rr.body.Resize(s)
	}
	// A description wraps only once the row knows its width, and then needs more (or less)
	// height than the row was given: lay the window out once more, later, not from inside this
	// layout. The row then gets the height it reports, and this does not repeat.
	if h := rr.body.MinSize().Height; h != s.Height && !rr.relayoutPending && s.Width > 0 {
		rr.relayoutPending = true
		Defer(func() {
			rr.relayoutPending = false
			if c := fyne.CurrentApp().Driver().CanvasForObject(rr.r); c != nil && c.Content() != nil {
				c.Content().Refresh()
			}
		})
	}
}

func (rr *settingRowRenderer) MinSize() fyne.Size { return rr.body.MinSize() }
func (rr *settingRowRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{rr.wash, rr.body}
}
func (rr *settingRowRenderer) Destroy() {}

// rowSplit lays out a setting's text and its control: the control at its natural width on the
// right, the text taking the rest, 24 px between them. Its height follows the text's height at
// the width it was last laid out at, so a wrapped description makes the row taller.
type rowSplit struct{ width float32 }

func (l *rowSplit) Layout(objs []fyne.CanvasObject, s fyne.Size) {
	text, ctl := objs[0], objs[1]
	cw := ctl.MinSize().Width
	tw := max(s.Width-cw-theme.Space6, 0)
	l.width = tw
	text.Move(fyne.NewPos(0, 0))
	text.Resize(fyne.NewSize(tw, text.MinSize().Height))
	ctl.Move(fyne.NewPos(s.Width-cw, 0))
	ctl.Resize(fyne.NewSize(cw, ctl.MinSize().Height))
}

func (l *rowSplit) MinSize(objs []fyne.CanvasObject) fyne.Size {
	text, ctl := objs[0].MinSize(), objs[1].MinSize()
	return fyne.NewSize(ctl.Width+theme.Space6+min(text.Width, 160), max(text.Height, ctl.Height))
}

// SettingGroup is a set of rows on one Raised card with a 6 px radius and hairlines between
// the rows, under an optional upper-case heading.
type SettingGroup struct {
	widget.BaseWidget
	Heading string
	Rows    []*SettingRow
}

// NewSettingGroup returns a group.
func NewSettingGroup(heading string, rows ...*SettingRow) *SettingGroup {
	g := &SettingGroup{Heading: heading, Rows: rows}
	g.ExtendBaseWidget(g)
	return g
}

func (g *SettingGroup) CreateRenderer() fyne.WidgetRenderer {
	r := &settingGroupRenderer{g: g, card: canvas.NewRectangle(nil)}
	var items []fyne.CanvasObject
	for i, row := range g.Rows {
		if i > 0 {
			items = append(items, NewRule(false))
		}
		items = append(items, row)
	}
	r.rows = container.New(layout.NewCustomPaddedVBoxLayout(0), items...)
	parts := []fyne.CanvasObject{}
	if g.Heading != "" {
		h := NewText(g.Heading, Label, Tertiary)
		h.Upper = true
		parts = append(parts, container.New(layout.NewCustomPaddedLayout(0, theme.Space1, theme.Space1, 0), h))
	}
	parts = append(parts, container.NewStack(r.card, r.rows))
	r.body = container.New(layout.NewCustomPaddedVBoxLayout(theme.Space2), parts...)
	r.Refresh()
	return r
}

type settingGroupRenderer struct {
	g    *SettingGroup
	card *canvas.Rectangle
	rows *fyne.Container
	body *fyne.Container
}

func (r *settingGroupRenderer) Refresh() {
	t := theme.Current()
	r.card.FillColor, r.card.StrokeColor, r.card.StrokeWidth = t.Surface.Raised, t.Border.Hairline, 1
	r.card.CornerRadius = theme.RadiusPanel
	r.card.Refresh()
	for _, o := range r.rows.Objects {
		o.Refresh()
	}
}

func (r *settingGroupRenderer) Layout(s fyne.Size)           { r.body.Resize(s) }
func (r *settingGroupRenderer) MinSize() fyne.Size           { return r.body.MinSize() }
func (r *settingGroupRenderer) Objects() []fyne.CanvasObject { return []fyne.CanvasObject{r.body} }
func (r *settingGroupRenderer) Destroy()                     {}
