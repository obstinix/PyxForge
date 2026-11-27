// Package mdview is the Markdown preview (Forge Notes, D25): a note or any Markdown file
// rendered in a widget.RichText. Headings, paragraphs with bold, italic and inline code,
// lists, task checkboxes, quotes, rules, code blocks coloured as the editor colours code, and
// links: [text](target) and [[wikilinks]] are clickable and go to OnLink. Images show their
// alt text. Nothing in the file is run.
package mdview

import (
	"image/color"
	"regexp"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	fynetheme "fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/obstinix/PyxForge/internal/syntax"
	"github.com/obstinix/PyxForge/internal/ui/codeedit"
	"github.com/obstinix/PyxForge/internal/ui/icons"
	"github.com/obstinix/PyxForge/internal/ui/kit"
	"github.com/obstinix/PyxForge/internal/ui/theme"
)

// Link is a link the reader clicked: a [[wikilink]]'s text, or a Markdown link's target.
type Link struct {
	Target string
	Wiki   bool
}

// Render turns Markdown into rich-text segments; onLink is told about clicked links.
func Render(src string, onLink func(Link)) []widget.RichTextSegment {
	var out []widget.RichTextSegment
	lines := strings.Split(strings.ReplaceAll(src, "\r\n", "\n"), "\n")
	var para []string
	flush := func() {
		if len(para) > 0 {
			out = append(out, inline(strings.Join(para, " "), widget.RichTextStyleParagraph, onLink)...)
			para = nil
		}
	}
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		trim := strings.TrimSpace(line)
		switch {
		case trim == "":
			flush()
		case strings.HasPrefix(trim, "```") || strings.HasPrefix(trim, "~~~"):
			flush()
			fenceMark := trim[:3]
			lang := strings.TrimSpace(trim[3:])
			var code []string
			for i++; i < len(lines) && !strings.HasPrefix(strings.TrimSpace(lines[i]), fenceMark); i++ {
				code = append(code, lines[i])
			}
			out = append(out, &CodeSegment{Lang: lang, Code: strings.Join(code, "\n")})
		case headingLevel(trim) > 0:
			flush()
			n := headingLevel(trim)
			text := strings.TrimSpace(strings.TrimRight(trim[n:], "#"))
			style := widget.RichTextStyleHeading
			switch {
			case n == 2:
				style = widget.RichTextStyleSubHeading
			case n >= 3:
				style = widget.RichTextStyleStrong
			}
			style.Inline = false // Fyne's heading styles are inline: a heading ends its line
			out = append(out, &widget.TextSegment{Text: text, Style: style})
		case isRule(trim):
			flush()
			out = append(out, &widget.SeparatorSegment{})
		case strings.HasPrefix(trim, ">"):
			flush()
			var quote []string
			for ; i < len(lines) && strings.HasPrefix(strings.TrimSpace(lines[i]), ">"); i++ {
				quote = append(quote, strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(lines[i]), ">")))
			}
			i--
			qs := widget.RichTextStyleBlockquote
			qs.Inline = false
			qs.ColorName = fynetheme.ColorNamePlaceHolder // quieter than body text
			out = append(out, &widget.TextSegment{Text: strings.Join(quote, " "), Style: qs})
		case listItem.MatchString(line):
			flush()
			var items []listEntry
			for ; i < len(lines) && listItem.MatchString(lines[i]); i++ {
				m := listItem.FindStringSubmatch(lines[i])
				items = append(items, listEntry{marker: m[2], text: m[3]})
			}
			i--
			out = append(out, listSegments(items, onLink)...)
		default:
			para = append(para, trim)
		}
	}
	flush()
	return out
}

func headingLevel(s string) int {
	n := 0
	for n < len(s) && n < 6 && s[n] == '#' {
		n++
	}
	if n > 0 && n < len(s) && s[n] == ' ' {
		return n
	}
	return 0
}

func isRule(s string) bool {
	c := strings.ReplaceAll(s, " ", "")
	return len(c) >= 3 && (strings.Count(c, "-") == len(c) || strings.Count(c, "*") == len(c) || strings.Count(c, "_") == len(c))
}

var listItem = regexp.MustCompile(`^(\s*)([-*+]|\d+[.)])\s+(.*)$`)

type listEntry struct{ marker, text string }

var taskBox = regexp.MustCompile(`^\[([ xX])\]\s+(.*)$`)

// listSegments makes a list; task items become checkbox rows.
func listSegments(items []listEntry, onLink func(Link)) []widget.RichTextSegment {
	ordered := len(items) > 0 && items[0].marker[0] >= '0' && items[0].marker[0] <= '9'
	var out []widget.RichTextSegment
	var plain []widget.RichTextSegment
	flushList := func() {
		if len(plain) > 0 {
			out = append(out, &widget.ListSegment{Items: plain, Ordered: ordered})
			plain = nil
		}
	}
	for _, it := range items {
		if m := taskBox.FindStringSubmatch(it.text); m != nil {
			flushList()
			out = append(out, &CheckSegment{Done: m[1] != " ", Text: m[2]})
			continue
		}
		segs := inline(it.text, widget.RichTextStyleInline, onLink)
		plain = append(plain, &widget.ParagraphSegment{Texts: segs})
	}
	flushList()
	return out
}

var inlineRe = regexp.MustCompile("\\[\\[([^\\]\\n]+)\\]\\]|!\\[([^\\]]*)\\]\\([^)]*\\)|\\[([^\\]\\n]+)\\]\\(([^)\\s]+)[^)]*\\)|`([^`]+)`|\\*\\*([^*]+)\\*\\*|__([^_]+)__|\\*([^*\\s][^*]*)\\*|_([^_\\s][^_]*)_")

// inline splits a run of text into plain, bold, italic, code and link segments.
func inline(s string, base widget.RichTextStyle, onLink func(Link)) []widget.RichTextSegment {
	var out []widget.RichTextSegment
	text := func(t string, st widget.RichTextStyle) {
		if t != "" {
			out = append(out, &widget.TextSegment{Text: t, Style: st})
		}
	}
	plain := base
	plain.Inline = true
	last := 0
	for _, m := range inlineRe.FindAllStringSubmatchIndex(s, -1) {
		text(s[last:m[0]], plain)
		g := func(i int) string {
			if m[2*i] < 0 {
				return ""
			}
			return s[m[2*i]:m[2*i+1]]
		}
		switch {
		case m[2] >= 0: // [[wikilink]]
			target := g(1)
			label := target
			if i := strings.Index(target, "|"); i >= 0 {
				label = strings.TrimSpace(target[i+1:])
			}
			out = append(out, link(label, Link{Target: target, Wiki: true}, onLink))
		case m[4] >= 0: // ![alt](image)
			text("[image: "+g(2)+"]", widget.RichTextStyleEmphasis)
		case m[6] >= 0: // [text](target)
			out = append(out, link(g(3), Link{Target: g(4)}, onLink))
		case m[10] >= 0:
			text(g(5), widget.RichTextStyleCodeInline)
		case m[12] >= 0 || m[14] >= 0:
			text(g(6)+g(7), widget.RichTextStyleStrong)
		default:
			text(g(8)+g(9), widget.RichTextStyleEmphasis)
		}
		last = m[1]
	}
	text(s[last:], plain)
	if !base.Inline && len(out) > 0 {
		// A paragraph ends its line (a ParagraphSegment does not, in Fyne 2.8).
		out = append(out, &widget.TextSegment{Style: widget.RichTextStyleParagraph})
	}
	return out
}

func link(label string, l Link, onLink func(Link)) widget.RichTextSegment {
	return &widget.HyperlinkSegment{Text: label, Alignment: fyne.TextAlignLeading, OnTapped: func() {
		if onLink != nil {
			onLink(l)
		}
	}}
}

// CheckSegment is a task item: a box, ticked when done, and its text.
type CheckSegment struct {
	Done bool
	Text string
}

func (c *CheckSegment) Inline() bool    { return false }
func (c *CheckSegment) Textual() string { return c.Text }
func (c *CheckSegment) Visual() fyne.CanvasObject {
	ic := kit.NewIcon(icons.Square, kit.Secondary)
	if c.Done {
		ic = kit.NewIcon(icons.CircleCheck, kit.Success)
	}
	role := kit.Primary
	if c.Done {
		role = kit.Secondary
	}
	return container.NewBorder(nil, nil, container.NewCenter(ic), nil, kit.NewText(c.Text, kit.Body, role))
}
func (c *CheckSegment) Update(fyne.CanvasObject)  {}
func (c *CheckSegment) Select(_, _ fyne.Position) {}
func (c *CheckSegment) SelectedText() string      { return "" }
func (c *CheckSegment) Unselect()                 {}

// CodeSegment is a fenced code block, coloured by its language as the editor colours code.
type CodeSegment struct {
	Lang string
	Code string
}

func (c *CodeSegment) Inline() bool    { return false }
func (c *CodeSegment) Textual() string { return c.Code }

// Visual draws the block: the sunken surface, one row per line, each token run in its colour.
func (c *CodeSegment) Visual() fyne.CanvasObject {
	t := theme.Current()
	h := syntax.ForFile("code."+langExt(c.Lang), c.Code)
	var spans [][]syntax.Span
	if h != nil {
		spans = h.Lines(c.Code)
	}
	size := theme.TextCode
	cell := fyne.MeasureText("M", size, fyne.TextStyle{Monospace: true})
	var rows []fyne.CanvasObject
	for i, line := range strings.Split(c.Code, "\n") {
		r := []rune(line)
		var objs []fyne.CanvasObject
		run := func(from, to int, col color.NRGBA) {
			if from >= to || strings.TrimSpace(string(r[from:to])) == "" {
				return
			}
			tx := canvas.NewText(string(r[from:to]), col)
			tx.TextSize, tx.TextStyle = size, fyne.TextStyle{Monospace: true}
			tx.Move(fyne.NewPos(float32(from)*cell.Width, 0))
			objs = append(objs, tx)
		}
		at := 0
		if i < len(spans) {
			for _, sp := range spans[i] {
				start, end := min(sp.Start, len(r)), min(sp.End, len(r))
				run(at, start, t.Text.Primary)
				run(start, end, codeedit.SyntaxColor(t, sp.Kind))
				at = max(at, end)
			}
		}
		run(at, len(r), t.Text.Primary)
		row := container.NewWithoutLayout(objs...)
		rows = append(rows, container.New(&fixedRow{w: float32(len(r)) * cell.Width, h: cell.Height + 2}, row))
	}
	bg := canvas.NewRectangle(t.Surface.Sunken)
	bg.CornerRadius = theme.RadiusControl
	body := container.New(layout.NewCustomPaddedLayout(theme.Space2, theme.Space2, theme.Space3, theme.Space3), container.NewVBox(rows...))
	return container.NewStack(bg, body)
}

func (c *CodeSegment) Update(fyne.CanvasObject)  {}
func (c *CodeSegment) Select(_, _ fyne.Position) {}
func (c *CodeSegment) SelectedText() string      { return "" }
func (c *CodeSegment) Unselect()                 {}

// fixedRow gives a code line its width and height.
type fixedRow struct{ w, h float32 }

func (l *fixedRow) Layout(objs []fyne.CanvasObject, s fyne.Size) {
	for _, o := range objs {
		o.Resize(s)
	}
}
func (l *fixedRow) MinSize([]fyne.CanvasObject) fyne.Size { return fyne.NewSize(l.w, l.h) }

// langExt maps a fence's language to a file extension the highlighter knows.
func langExt(lang string) string {
	switch strings.ToLower(lang) {
	case "", "text", "txt":
		return "txt"
	case "c", "h":
		return "c"
	case "cpp", "c++", "cc":
		return "cpp"
	case "asm", "nasm", "x86", "s":
		return "asm"
	case "go", "golang":
		return "go"
	case "rust", "rs":
		return "rs"
	case "python", "py":
		return "py"
	case "sh", "bash", "shell", "console":
		return "sh"
	case "powershell", "ps1", "pwsh":
		return "ps1"
	case "toml":
		return "toml"
	case "json":
		return "json"
	case "js", "javascript":
		return "js"
	case "ts", "typescript":
		return "ts"
	}
	return strings.ToLower(lang)
}

// View is a scrolling preview of one Markdown text.
type View struct {
	widget.BaseWidget
	rich   *widget.RichText
	scroll *container.Scroll
	OnLink func(Link)
}

// New is an empty preview.
func New() *View {
	v := &View{rich: widget.NewRichText()}
	v.rich.Wrapping = fyne.TextWrapWord
	v.scroll = container.NewVScroll(container.New(layout.NewCustomPaddedLayout(theme.Space4, theme.Space6, theme.Space6, theme.Space6), v.rich))
	v.ExtendBaseWidget(v)
	return v
}

// SetText shows a Markdown text, keeping the scroll position.
func (v *View) SetText(src string) {
	v.rich.Segments = Render(src, func(l Link) {
		if v.OnLink != nil {
			v.OnLink(l)
		}
	})
	v.rich.Refresh()
}

// Segments are the segments shown; for tests.
func (v *View) Segments() []widget.RichTextSegment { return v.rich.Segments }

func (v *View) CreateRenderer() fyne.WidgetRenderer {
	bg := kit.NewSurface(kit.Base, nil)
	return widget.NewSimpleRenderer(container.NewStack(bg, v.scroll))
}
