package mdview

import (
	"image/color"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"github.com/obstinix/PyxForge/internal/ui/theme"
)

const note = "# Boot hang\n\nThe **stage 1** loader *hangs* at `int 0x13`; see [[boot.asm#read_disk|the reader]] and [the spec](docs/spec.md).\n\n" +
	"## Steps\n\n- one\n- [ ] check the CHS maths\n- [x] read the BIOS table\n\n1. first\n2. second\n\n> A quote\n\n---\n\n```nasm\nmov ah, 0x02 ; read\nint 0x13\n```\n\n![diagram](d.png)\n"

func segs(t *testing.T) []widget.RichTextSegment {
	t.Helper()
	test.NewTempApp(t).Settings().SetTheme(theme.NewFyne(theme.Default))
	return Render(note, nil)
}

// flatten lists every segment, paragraphs and lists opened up.
func flatten(ss []widget.RichTextSegment) []widget.RichTextSegment {
	var out []widget.RichTextSegment
	for _, s := range ss {
		out = append(out, s)
		switch x := s.(type) {
		case *widget.ParagraphSegment:
			out = append(out, flatten(x.Texts)...)
		case *widget.ListSegment:
			out = append(out, flatten(x.Items)...)
		}
	}
	return out
}

func TestHeadingsAndText(t *testing.T) {
	all := flatten(segs(t))
	var h1, h2, bold, em, code bool
	for _, s := range all {
		ts, ok := s.(*widget.TextSegment)
		if !ok {
			continue
		}
		switch {
		case ts.Text == "Boot hang" && ts.Style.SizeName == widget.RichTextStyleHeading.SizeName && !ts.Inline():
			h1 = true
		case ts.Text == "Steps" && ts.Style.SizeName == widget.RichTextStyleSubHeading.SizeName && !ts.Inline():
			h2 = true
		case ts.Text == "stage 1" && ts.Style == widget.RichTextStyleStrong:
			bold = true
		case ts.Text == "hangs" && ts.Style == widget.RichTextStyleEmphasis:
			em = true
		case ts.Text == "int 0x13" && ts.Style == widget.RichTextStyleCodeInline:
			code = true
		}
	}
	if !h1 || !h2 || !bold || !em || !code {
		t.Errorf("heading %v, subheading %v, bold %v, italic %v, inline code %v", h1, h2, bold, em, code)
	}
}

func TestListsAndChecks(t *testing.T) {
	var lists []*widget.ListSegment
	var checks []*CheckSegment
	for _, s := range segs(t) {
		switch x := s.(type) {
		case *widget.ListSegment:
			lists = append(lists, x)
		case *CheckSegment:
			checks = append(checks, x)
		}
	}
	if len(lists) != 2 || lists[0].Ordered || !lists[1].Ordered || len(lists[1].Items) != 2 {
		t.Fatalf("lists %+v", lists)
	}
	if len(checks) != 2 || checks[0].Done || !checks[1].Done || checks[0].Text != "check the CHS maths" {
		t.Errorf("checkboxes %+v", checks)
	}
}

// TestLinks: wikilinks and Markdown links are clickable and say where they go.
func TestLinks(t *testing.T) {
	test.NewTempApp(t)
	var got []Link
	all := flatten(Render(note, func(l Link) { got = append(got, l) }))
	var labels []string
	for _, s := range all {
		if h, ok := s.(*widget.HyperlinkSegment); ok {
			labels = append(labels, h.Text)
			h.OnTapped()
		}
	}
	if strings.Join(labels, "|") != "the reader|the spec" {
		t.Fatalf("links %v", labels)
	}
	if len(got) != 2 || got[0] != (Link{Target: "boot.asm#read_disk|the reader", Wiki: true}) || got[1] != (Link{Target: "docs/spec.md"}) {
		t.Errorf("clicked %+v", got)
	}
}

// TestCodeBlockColours: a fenced block is drawn in the editor's syntax colours, one text run
// per token.
func TestCodeBlockColours(t *testing.T) {
	var code *CodeSegment
	for _, s := range segs(t) {
		if c, ok := s.(*CodeSegment); ok {
			code = c
		}
	}
	if code == nil || code.Lang != "nasm" || !strings.HasPrefix(code.Code, "mov ah, 0x02") {
		t.Fatalf("code block %+v", code)
	}
	colours := map[color.Color]bool{}
	var walk func(fyne.CanvasObject)
	walk = func(o fyne.CanvasObject) {
		if tx, ok := o.(*canvas.Text); ok {
			colours[tx.Color] = true
		}
		if c, ok := o.(*fyne.Container); ok {
			for _, ch := range c.Objects {
				walk(ch)
			}
		}
	}
	walk(code.Visual())
	if len(colours) < 3 {
		t.Errorf("the block has %d colours; a highlighted block has several", len(colours))
	}
}

func TestRulesQuotesImages(t *testing.T) {
	var rule, quote, image bool
	for _, s := range flatten(segs(t)) {
		switch x := s.(type) {
		case *widget.SeparatorSegment:
			rule = true
		case *widget.TextSegment:
			quote = quote || x.Style.TextStyle.Italic && !x.Inline() && x.Text == "A quote"
			image = image || x.Text == "[image: diagram]"
		}
	}
	if !rule || !quote || !image {
		t.Errorf("rule %v, quote %v, image %v", rule, quote, image)
	}
}

func TestView(t *testing.T) {
	test.NewTempApp(t)
	v := New()
	w := test.NewTempWindow(t, v)
	w.Resize(fyne.NewSize(600, 400))
	v.SetText("# Title\n\nbody")
	if len(v.Segments()) != 3 { // the heading, the paragraph, its line end
		t.Errorf("%d segments", len(v.Segments()))
	}
}
