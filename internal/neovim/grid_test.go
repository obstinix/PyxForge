package neovim

import "testing"

// ev builds one redraw update: [name, call, call...], with every call's args as []any.
func ev(name string, calls ...[]any) []any {
	out := []any{name}
	for _, c := range calls {
		out = append(out, c)
	}
	return out
}

func TestGridLineAndFlush(t *testing.T) {
	var g Grid
	flushed := g.Apply([]any{
		ev("grid_resize", []any{int64(1), int64(10), int64(3)}),
		ev("hl_attr_define", []any{int64(7), map[string]any{"foreground": int64(0xff0000), "bold": true}, map[string]any{}, []any{}}),
		ev("grid_line",
			// "mov" in hl 7, then "ax" in the same hl (omitted ID), then 3 spaces with hl 0.
			[]any{int64(1), int64(0), int64(0), []any{[]any{"m", int64(7)}, []any{"o"}, []any{"v"}, []any{" ", int64(0), int64(3)}}, false},
			[]any{int64(1), int64(1), int64(2), []any{[]any{"x", int64(0), int64(2)}}, false},
		),
		ev("grid_cursor_goto", []any{int64(1), int64(1), int64(3)}),
		ev("flush", []any{}),
	})
	if !flushed {
		t.Fatal("flush not reported")
	}
	s := g.Snapshot()
	if rows, cols := s.Size(); rows != 3 || cols != 10 {
		t.Fatalf("size %dx%d", rows, cols)
	}
	if got := s.Line(0); got != "mov       " {
		t.Errorf("row 0 = %q", got)
	}
	if got := s.Line(1); got != "  xx      " {
		t.Errorf("row 1 = %q", got)
	}
	if s.Rows[0][1].HL != 7 || s.Rows[0][3].HL != 0 {
		t.Errorf("highlight IDs: %+v", s.Rows[0][:4])
	}
	if a := s.Attr(7); a.FG != 0xff0000 || !a.Bold || a.BG != s.BG {
		t.Errorf("attr 7 = %+v", a)
	}
	if s.CursorRow != 1 || s.CursorCol != 3 || s.Version != 1 {
		t.Errorf("cursor %d,%d version %d", s.CursorRow, s.CursorCol, s.Version)
	}
}

func TestGridScroll(t *testing.T) {
	var g Grid
	g.Apply([]any{ev("grid_resize", []any{int64(1), int64(3), int64(4)})})
	for r, s := range []string{"aaa", "bbb", "ccc", "ddd"} {
		cells := []any{}
		for _, ch := range s {
			cells = append(cells, []any{string(ch)})
		}
		g.Apply([]any{ev("grid_line", []any{int64(1), int64(r), int64(0), cells, false})})
	}
	// Scroll rows 0..3 up by one (as Ctrl-E does), then down by two.
	g.Apply([]any{ev("grid_scroll", []any{int64(1), int64(0), int64(4), int64(0), int64(3), int64(1), int64(0)})})
	s := g.Snapshot()
	if s.Line(0) != "bbb" || s.Line(1) != "ccc" || s.Line(2) != "ddd" {
		t.Errorf("after scroll up: %q %q %q", s.Line(0), s.Line(1), s.Line(2))
	}
	g.Apply([]any{ev("grid_scroll", []any{int64(1), int64(0), int64(4), int64(0), int64(3), int64(-2), int64(0)})})
	s = g.Snapshot()
	if s.Line(2) != "bbb" || s.Line(3) != "ccc" {
		t.Errorf("after scroll down: %q %q", s.Line(2), s.Line(3))
	}
}

func TestGridModesColorsAndResize(t *testing.T) {
	var g Grid
	g.Apply([]any{
		ev("grid_resize", []any{int64(1), int64(4), int64(2)}),
		ev("default_colors_set", []any{int64(0xeeeeee), int64(0x101010), int64(-1), int64(0), int64(0)}),
		ev("mode_info_set", []any{true, []any{
			map[string]any{"name": "normal", "cursor_shape": "block", "cell_percentage": int64(0)},
			map[string]any{"name": "insert", "cursor_shape": "vertical", "cell_percentage": int64(25)},
		}}),
		ev("mode_change", []any{"insert", int64(1)}),
		ev("hl_attr_define", []any{int64(3), map[string]any{"reverse": true}, map[string]any{}, []any{}}),
		ev("set_title", []any{"boot.asm - NVIM"}),
	})
	s := g.Snapshot()
	if s.FG != 0xeeeeee || s.BG != 0x101010 || s.Mode != "insert" || s.Cursor.Shape != "vertical" || s.Cursor.Percent != 25 || s.Title != "boot.asm - NVIM" {
		t.Errorf("snapshot = %+v", s)
	}
	if a := s.Attr(3); a.FG != 0x101010 || a.BG != 0xeeeeee {
		t.Errorf("reverse attr = %+v", a)
	}
	// Growing keeps existing cells and fills new ones with spaces.
	g.Apply([]any{ev("grid_line", []any{int64(1), int64(0), int64(0), []any{[]any{"x"}}, false})})
	g.Apply([]any{ev("grid_resize", []any{int64(1), int64(6), int64(3)})})
	s = g.Snapshot()
	if s.Line(0) != "x     " || s.Line(2) != "      " {
		t.Errorf("after grow: %q %q", s.Line(0), s.Line(2))
	}
	// A cursor beyond a shrunken grid is clamped.
	g.Apply([]any{ev("grid_cursor_goto", []any{int64(1), int64(2), int64(5)}), ev("grid_resize", []any{int64(1), int64(2), int64(1)})})
	if s := g.Snapshot(); s.CursorRow != 0 || s.CursorCol != 1 {
		t.Errorf("cursor not clamped: %d,%d", s.CursorRow, s.CursorCol)
	}
}

func TestGridIgnoresMalformedEvents(t *testing.T) {
	var g Grid
	g.Apply([]any{"not a list", []any{}, ev("grid_line", []any{int64(1)}), ev("grid_resize", []any{int64(1), int64(2), int64(2)})})
	g.Apply([]any{ev("grid_line", []any{int64(1), int64(9), int64(0), []any{[]any{"x"}}, false})}) // row out of range
	if s := g.Snapshot(); s.Line(0) != "  " {
		t.Errorf("grid = %q", s.Line(0))
	}
}
