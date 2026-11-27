package codeedit

import (
	"image/color"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"

	"github.com/obstinix/PyxForge/internal/syntax"
	"github.com/obstinix/PyxForge/internal/ui/theme"
)

type renderer struct {
	e       *Editor
	bg      *canvas.Rectangle
	objects []fyne.CanvasObject
}

func (r *renderer) Layout(s fyne.Size) {
	r.bg.Resize(s)
	e := r.e
	e.rows = max(int(s.Height/e.cellH), 1)
	e.cols = max(int((s.Width-e.gutterWidth())/e.cellW), 1)
}

func (r *renderer) MinSize() fyne.Size { return fyne.NewSize(r.e.cellW*24, r.e.cellH*3) }

func (r *renderer) Objects() []fyne.CanvasObject {
	if r.objects == nil {
		return []fyne.CanvasObject{r.bg}
	}
	return r.objects
}

func (r *renderer) Destroy() {}

func syntaxColor(t theme.Tokens, k syntax.Kind) color.NRGBA {
	s := t.Syntax
	switch k {
	case syntax.Comment:
		return s.Comment
	case syntax.Keyword:
		return s.Keyword
	case syntax.Type:
		return s.Type
	case syntax.Function:
		return s.Function
	case syntax.Variable:
		return s.Variable
	case syntax.Constant:
		return s.Constant
	case syntax.String:
		return s.String
	case syntax.Number:
		return s.Number
	case syntax.Operator:
		return s.Operator
	case syntax.Attribute:
		return s.Attribute
	case syntax.Error:
		return s.Error
	}
	return t.Text.Primary
}

func rect(c color.Color, x, y, w, h float32) *canvas.Rectangle {
	b := canvas.NewRectangle(c)
	b.Move(fyne.NewPos(x, y))
	b.Resize(fyne.NewSize(w, h))
	return b
}

// expand returns a line with tabs as spaces and the display column of every rune column.
func expand(line []rune, tab int) (string, []int) {
	var s strings.Builder
	cols := make([]int, len(line)+1)
	d := 0
	for i, ch := range line {
		cols[i] = d
		if ch == '\t' {
			w := tab - d%tab
			s.WriteString(strings.Repeat(" ", w))
			d += w
		} else {
			s.WriteRune(ch)
			d++
		}
	}
	cols[len(line)] = d
	return s.String(), cols
}

// Refresh rebuilds the frame: gutter, current line, selection, text runs coloured by the
// highlighter, problem underlines, the matching bracket, the cursor, and the completion list
// or information box.
func (r *renderer) Refresh() {
	e := r.e
	t := theme.Current()
	e.highlight()
	size := e.Size()
	r.bg.FillColor = t.Surface.Base
	r.bg.Resize(size)
	objs := []fyne.CanvasObject{r.bg}
	gw := e.gutterWidth()
	objs = append(objs, rect(t.Surface.Base, 0, 0, gw, size.Height))
	lines := e.VisibleLines(e.top, e.rows+1)
	sel := e.Selection()
	_, match, hasMatch := e.MatchBracket(e.Cur)
	bracketAt := Pos{Line: -1, Col: -1}
	if hasMatch {
		if a, _, ok := e.MatchBracket(e.Cur); ok {
			bracketAt = a
		}
	}
	textX := func(dcol int) float32 { return gw + float32(dcol-e.left)*e.cellW }
	var curX, curY float32 = -1, -1
	for row, l := range lines {
		y := float32(row) * e.cellH
		line := e.Buf.Line(l)
		shown, dcols := expand(line, e.TabWidth)
		srunes := []rune(shown)
		if l == e.Cur.Line && !e.HasSelection() {
			objs = append(objs, rect(t.State.Hover, gw, y, size.Width-gw, e.cellH))
		}
		// Gutter: mark, number, fold marker.
		if c, ok := e.marks[l]; ok {
			objs = append(objs, rect(c, 0, y, 3, e.cellH))
		}
		num := canvas.NewText(strconv.Itoa(l+1), t.Text.Tertiary)
		if l == e.Cur.Line {
			num.Color = t.Text.Secondary
		}
		num.TextSize, num.TextStyle = e.textSize, fyne.TextStyle{Monospace: true}
		num.Alignment = fyne.TextAlignTrailing
		num.Move(fyne.NewPos(0, y+1))
		num.Resize(fyne.NewSize(gw-2*e.cellW, e.cellH))
		objs = append(objs, num)
		if _, folded := e.Folded(l); folded {
			m := canvas.NewText("›", t.Accent.Primary)
			m.TextSize, m.TextStyle = e.textSize, fyne.TextStyle{Monospace: true, Bold: true}
			m.Move(fyne.NewPos(gw-1.6*e.cellW, y+1))
			objs = append(objs, m)
		}
		// Selection on this line.
		if !sel.Empty() && l >= sel.Start.Line && l <= sel.End.Line {
			from, to := 0, dcols[len(line)]+1
			if l == sel.Start.Line {
				from = dcols[sel.Start.Col]
			}
			if l == sel.End.Line {
				to = dcols[sel.End.Col]
			}
			x0, x1 := max(textX(from), gw), textX(to)
			if x1 > x0 {
				objs = append(objs, rect(t.Accent.Selection, x0, y, x1-x0, e.cellH))
			}
		}
		// Text runs: plain between spans, coloured inside them.
		var spans []syntax.Span
		if l < len(e.spans) {
			spans = e.spans[l]
		}
		emit := func(fromRune, toRune int, c color.NRGBA, bold bool) {
			if toRune <= fromRune {
				return
			}
			d0, d1 := dcols[min(fromRune, len(line))], dcols[min(toRune, len(line))]
			d0, d1 = max(d0, e.left), min(d1, e.left+e.cols+1)
			if d1 <= d0 || d0 >= len(srunes) {
				return
			}
			s := string(srunes[d0:min(d1, len(srunes))])
			if strings.TrimSpace(s) == "" {
				return
			}
			tx := canvas.NewText(s, c)
			tx.TextSize, tx.TextStyle = e.textSize, fyne.TextStyle{Monospace: true, Bold: bold}
			tx.Move(fyne.NewPos(textX(d0), y+1))
			objs = append(objs, tx)
		}
		at := 0
		for _, sp := range spans {
			emit(at, sp.Start, t.Text.Primary, false)
			emit(sp.Start, sp.End, syntaxColor(t, sp.Kind), sp.Kind == syntax.Keyword)
			at = sp.End
		}
		emit(at, len(line), t.Text.Primary, false)
		if end, folded := e.Folded(l); folded {
			tag := canvas.NewText(" … "+strconv.Itoa(end-l)+" lines", t.Text.Tertiary)
			tag.TextSize, tag.TextStyle = e.textSize, fyne.TextStyle{Monospace: true, Italic: true}
			tag.Move(fyne.NewPos(textX(dcols[len(line)]), y+1))
			objs = append(objs, tag)
		}
		// Problems: a 2 px rule under the range.
		for _, d := range e.diags {
			if l < d.Range.Start.Line || l > d.Range.End.Line {
				continue
			}
			c := t.Status.Error
			switch d.Severity {
			case SevWarning:
				c = t.Status.Warning
			case SevInfo, SevHint:
				c = t.Status.Info
			}
			from, to := 0, len(line)
			if l == d.Range.Start.Line {
				from = min(d.Range.Start.Col, len(line))
			}
			if l == d.Range.End.Line {
				to = min(d.Range.End.Col, len(line))
			}
			if to <= from { // an empty range marks one character
				to = min(from+1, len(line))
				if to == from {
					from, to = max(from-1, 0), max(from, 1)
				}
			}
			x0, x1 := max(textX(dcols[from]), gw), textX(dcols[min(to, len(line))])
			if x1 <= x0 {
				x1 = x0 + e.cellW
			}
			objs = append(objs, rect(c, x0, y+e.cellH-2, x1-x0, 2))
			objs = append(objs, rect(c, 0, y, 3, e.cellH))
		}
		// Matching brackets: a hairline box round both.
		for _, b := range []Pos{bracketAt, match} {
			if hasMatch && b.Line == l {
				x := textX(dcols[b.Col])
				if x >= gw {
					box := rect(color.Transparent, x, y, e.cellW, e.cellH)
					box.StrokeColor, box.StrokeWidth = t.Border.Strong, 1
					objs = append(objs, box)
				}
			}
		}
		if l == e.Cur.Line {
			curX, curY = textX(dcols[e.Cur.Col]), y
		}
	}
	// Cursor: a 2 px bar while focused, an outline otherwise.
	if curX >= gw {
		if e.focused {
			objs = append(objs, rect(t.Text.Primary, curX, curY, 2, e.cellH))
		} else {
			box := rect(color.Transparent, curX, curY, e.cellW, e.cellH)
			box.StrokeColor, box.StrokeWidth = t.Text.Tertiary, 1
			objs = append(objs, box)
		}
	}
	objs = append(objs, rect(t.Border.Hairline, gw-1, 0, 1, size.Height))
	if curY >= 0 {
		objs = append(objs, r.popups(t, curX, curY, size)...)
	}
	r.objects = objs
	// Refreshing the background marks the canvas dirty, so the window repaints the new frame.
	r.bg.Refresh()
}

// popups draws the completion list or the information box under the cursor line (above it
// when there is no room below).
func (r *renderer) popups(t theme.Tokens, x, y float32, size fyne.Size) []fyne.CanvasObject {
	e := r.e
	var out []fyne.CanvasObject
	box := func(lines []string, highlight int, width int) {
		h := float32(len(lines)) * e.cellH
		w := float32(width+2) * e.cellW
		top := y + e.cellH + 2
		if top+h > size.Height && y-h-2 >= 0 {
			top = y - h - 2
		}
		left := min(max(x, e.gutterWidth()), max(size.Width-w, 0))
		if theme.GlassOn() {
			// Liquid Glass (D21): a blurred, tinted frame around an opaque plate the list sits on.
			const frame = 3
			blur := canvas.NewBlur(theme.GlassRadius())
			blur.CornerRadius = theme.RadiusControl + frame
			blur.Move(fyne.NewPos(left-frame, top-frame))
			blur.Resize(fyne.NewSize(w+2*frame, h+2*frame))
			glass := rect(t.Float.GlassTint, left-frame, top-frame, w+2*frame, h+2*frame)
			glass.CornerRadius = theme.RadiusControl + frame
			glass.StrokeColor, glass.StrokeWidth = t.Border.Strong, 1
			plate := rect(t.Surface.Overlay, left, top, w, h)
			plate.CornerRadius = theme.RadiusControl
			out = append(out, blur, glass, plate)
		} else {
			bg := rect(t.Surface.Overlay, left, top, w, h)
			bg.StrokeColor, bg.StrokeWidth = t.Border.Strong, 1
			out = append(out, bg)
		}
		for i, s := range lines {
			c := t.Text.Primary
			if i == highlight {
				out = append(out, rect(t.Accent.Selection, left+1, top+float32(i)*e.cellH, w-2, e.cellH))
			}
			tx := canvas.NewText(s, c)
			tx.TextSize, tx.TextStyle = e.textSize, fyne.TextStyle{Monospace: true}
			tx.Move(fyne.NewPos(left+e.cellW, top+float32(i)*e.cellH+1))
			out = append(out, tx)
		}
	}
	if len(e.completes) > 0 {
		const shown = 10
		first := max(e.sel-shown+1, 0)
		var lines []string
		width := 0
		for _, c := range e.completes[first:min(first+shown, len(e.completes))] {
			s := c.Label
			if c.Detail != "" {
				s += "  " + c.Detail
			}
			if len([]rune(s)) > 60 {
				s = string([]rune(s)[:59]) + "…"
			}
			lines = append(lines, s)
			width = max(width, len([]rune(s)))
		}
		box(lines, e.sel-first, width)
		return out
	}
	if e.info != "" {
		var lines []string
		width := 0
		for _, l := range strings.Split(e.info, "\n") {
			for len([]rune(l)) > 80 {
				lines = append(lines, string([]rune(l)[:80]))
				l = string([]rune(l)[80:])
			}
			lines = append(lines, l)
			if len(lines) >= 12 {
				break
			}
		}
		for _, l := range lines {
			width = max(width, len([]rune(l)))
		}
		box(lines, -1, width)
	}
	return out
}

// SyntaxColor is a token kind's colour in the theme, as the editor draws it; the Markdown
// preview's code blocks use it too.
func SyntaxColor(t theme.Tokens, k syntax.Kind) color.NRGBA { return syntaxColor(t, k) }
