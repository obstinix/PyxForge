package neovim

import (
	"sync"
)

// Cell is one screen cell: its text (one grapheme, or "" for the right half of a wide
// character) and its highlight ID.
type Cell struct {
	Text string
	HL   int
}

// Attr is a highlight's resolved attributes. Colors are 0xRRGGBB; -1 means "use the default".
type Attr struct {
	FG, BG, SP                                          int32
	Bold, Italic, Underline, Undercurl, Strike, Reverse bool
}

var defaultAttr = Attr{FG: -1, BG: -1, SP: -1}

// CursorShape is how the current mode draws the cursor.
type CursorShape struct {
	Shape   string // "block", "horizontal" or "vertical"
	Percent int    // cell percentage for horizontal and vertical shapes
}

// Grid is Neovim's screen as the linegrid UI protocol describes it (`:help ui-linegrid`).
// Redraw batches are applied from the RPC goroutine; renderers read a Snapshot. The zero
// value is ready to use.
type Grid struct {
	mu        sync.Mutex
	rows      [][]Cell
	attrs     map[int]Attr
	fg, bg    int32
	sp        int32
	cursorRow int
	cursorCol int
	mode      string
	shapes    []CursorShape
	shape     CursorShape
	title     string
	busy      bool
	version   uint64 // increases on every flush
}

// Snapshot is a consistent copy of the grid for one frame.
type Snapshot struct {
	Rows       [][]Cell
	Attrs      map[int]Attr
	FG, BG, SP int32
	CursorRow  int
	CursorCol  int
	Mode       string
	Cursor     CursorShape
	Title      string
	Busy       bool
	Version    uint64
}

// Size reports the grid's dimensions.
func (s Snapshot) Size() (rows, cols int) {
	if len(s.Rows) == 0 {
		return 0, 0
	}
	return len(s.Rows), len(s.Rows[0])
}

// Attr resolves a highlight ID, falling back to the default colors.
func (s Snapshot) Attr(id int) Attr {
	a, ok := s.Attrs[id]
	if !ok {
		a = defaultAttr
	}
	if a.FG < 0 {
		a.FG = s.FG
	}
	if a.BG < 0 {
		a.BG = s.BG
	}
	if a.SP < 0 {
		a.SP = s.SP
	}
	if a.Reverse {
		a.FG, a.BG = a.BG, a.FG
	}
	return a
}

// Line returns row r as plain text, for tests and accessibility.
func (s Snapshot) Line(r int) string {
	if r < 0 || r >= len(s.Rows) {
		return ""
	}
	b := make([]byte, 0, len(s.Rows[r]))
	for _, c := range s.Rows[r] {
		b = append(b, c.Text...)
	}
	return string(b)
}

// Snapshot copies the grid.
func (g *Grid) Snapshot() Snapshot {
	g.mu.Lock()
	defer g.mu.Unlock()
	rows := make([][]Cell, len(g.rows))
	for i, r := range g.rows {
		rows[i] = append([]Cell(nil), r...)
	}
	attrs := make(map[int]Attr, len(g.attrs))
	for k, v := range g.attrs {
		attrs[k] = v
	}
	return Snapshot{rows, attrs, g.fg, g.bg, g.sp, g.cursorRow, g.cursorCol, g.mode, g.shape, g.title, g.busy, g.version}
}

// Apply handles one "redraw" notification: a batch of [event, args...] updates. It reports
// whether the batch ended with a flush, meaning the screen is consistent and can be drawn.
func (g *Grid) Apply(updates []any) (flushed bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.attrs == nil {
		g.attrs = map[int]Attr{}
		g.fg, g.bg, g.sp = 0xd4d4d4, 0x1e1e1e, 0xff0000
	}
	for _, u := range updates {
		ev, ok := u.([]any)
		if !ok || len(ev) == 0 {
			continue
		}
		name, _ := ev[0].(string)
		for _, call := range ev[1:] {
			args, _ := call.([]any)
			if g.apply(name, args) {
				flushed = true
			}
		}
		if name == "flush" && len(ev) == 1 {
			g.version++
			flushed = true
		}
	}
	return flushed
}

func (g *Grid) apply(name string, a []any) (flush bool) {
	switch name {
	case "grid_resize":
		if len(a) >= 3 {
			g.resize(toInt(a[2]), toInt(a[1]))
		}
	case "grid_clear":
		for _, r := range g.rows {
			clear(r)
		}
	case "grid_cursor_goto":
		if len(a) >= 3 {
			g.cursorRow, g.cursorCol = toInt(a[1]), toInt(a[2])
		}
	case "grid_line":
		if len(a) >= 4 {
			g.line(toInt(a[1]), toInt(a[2]), a[3])
		}
	case "grid_scroll":
		if len(a) >= 6 {
			g.scroll(toInt(a[1]), toInt(a[2]), toInt(a[3]), toInt(a[4]), toInt(a[5]))
		}
	case "hl_attr_define":
		if len(a) >= 2 {
			g.attrs[toInt(a[0])] = parseAttr(a[1])
		}
	case "default_colors_set":
		if len(a) >= 3 {
			g.fg, g.bg, g.sp = colorOr(a[0], g.fg), colorOr(a[1], g.bg), colorOr(a[2], g.sp)
		}
	case "mode_info_set":
		if len(a) >= 2 {
			g.shapes = parseModes(a[1])
		}
	case "mode_change":
		if len(a) >= 2 {
			g.mode, _ = a[0].(string)
			if i := toInt(a[1]); i >= 0 && i < len(g.shapes) {
				g.shape = g.shapes[i]
			}
		}
	case "set_title":
		if len(a) >= 1 {
			g.title, _ = a[0].(string)
		}
	case "busy_start":
		g.busy = true
	case "busy_stop":
		g.busy = false
	case "flush":
		g.version++
		return true
	}
	return false
}

func (g *Grid) resize(rows, cols int) {
	next := make([][]Cell, rows)
	for r := range next {
		next[r] = make([]Cell, cols)
		if r < len(g.rows) {
			copy(next[r], g.rows[r])
		}
		for c := range next[r] {
			if next[r][c].Text == "" && (r >= len(g.rows) || c >= len(g.rows[r])) {
				next[r][c].Text = " "
			}
		}
	}
	g.rows = next
	if g.cursorRow >= rows {
		g.cursorRow = max(rows-1, 0)
	}
	if g.cursorCol >= cols {
		g.cursorCol = max(cols-1, 0)
	}
}

func (g *Grid) line(row, col int, cells any) {
	if row < 0 || row >= len(g.rows) {
		return
	}
	r := g.rows[row]
	list, _ := cells.([]any)
	hl := 0
	for _, c := range list {
		cell, _ := c.([]any)
		if len(cell) == 0 {
			continue
		}
		text, _ := cell[0].(string)
		if len(cell) >= 2 {
			hl = toInt(cell[1])
		}
		repeat := 1
		if len(cell) >= 3 {
			repeat = toInt(cell[2])
		}
		for range repeat {
			if col >= 0 && col < len(r) {
				r[col] = Cell{Text: text, HL: hl}
			}
			col++
		}
	}
}

// scroll moves the region [top, bot) × [left, right) by rows: positive moves content up.
func (g *Grid) scroll(top, bot, left, right, rows int) {
	if top < 0 || bot > len(g.rows) || left < 0 || rows == 0 {
		return
	}
	if rows > 0 {
		for r := top; r < bot-rows; r++ {
			copyRange(g.rows[r], g.rows[r+rows], left, right)
		}
	} else {
		for r := bot - 1; r >= top-rows; r-- {
			copyRange(g.rows[r], g.rows[r+rows], left, right)
		}
	}
}

func copyRange(dst, src []Cell, left, right int) {
	right = min(right, len(dst), len(src))
	if left < right {
		copy(dst[left:right], src[left:right])
	}
}

func parseAttr(v any) Attr {
	a := defaultAttr
	m, _ := v.(map[string]any)
	for k, val := range m {
		switch k {
		case "foreground":
			a.FG = int32(toInt(val))
		case "background":
			a.BG = int32(toInt(val))
		case "special":
			a.SP = int32(toInt(val))
		case "bold":
			a.Bold = toBool(val)
		case "italic":
			a.Italic = toBool(val)
		case "underline":
			a.Underline = toBool(val)
		case "undercurl":
			a.Undercurl = toBool(val)
		case "strikethrough":
			a.Strike = toBool(val)
		case "reverse":
			a.Reverse = toBool(val)
		}
	}
	return a
}

func parseModes(v any) []CursorShape {
	list, _ := v.([]any)
	out := make([]CursorShape, len(list))
	for i, m := range list {
		mm, _ := m.(map[string]any)
		out[i] = CursorShape{Shape: "block", Percent: 100}
		if s, ok := mm["cursor_shape"].(string); ok {
			out[i].Shape = s
		}
		if p, ok := mm["cell_percentage"]; ok {
			out[i].Percent = toInt(p)
		}
	}
	return out
}

// colorOr returns a decoded color, or def for -1 (Neovim's "no color").
func colorOr(v any, def int32) int32 {
	c := toInt(v)
	if c < 0 {
		return def
	}
	return int32(c)
}

// toInt reads the integer types the msgpack decoder produces.
func toInt(v any) int {
	switch n := v.(type) {
	case int64:
		return int(n)
	case uint64:
		return int(n)
	case int:
		return n
	case int32:
		return int(n)
	case uint32:
		return int(n)
	case int8:
		return int(n)
	case uint8:
		return int(n)
	case int16:
		return int(n)
	case uint16:
		return int(n)
	case float64:
		return int(n)
	}
	return 0
}

func toBool(v any) bool {
	b, ok := v.(bool)
	return ok && b
}
