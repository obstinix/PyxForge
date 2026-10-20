package kit

import (
	"fmt"
	"runtime"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/test"
	fynetheme "fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

func items(names ...string) []*container.TabItem {
	var out []*container.TabItem
	for _, n := range names {
		out = append(out, container.NewTabItem(n, widget.NewLabel(n+" content")))
	}
	return out
}

func TestTabsSelection(t *testing.T) {
	test.NewTempApp(t)
	tabs := NewDocTabs()
	var selected []string
	tabs.OnSelected = func(it *container.TabItem) { selected = append(selected, it.Text) }
	if tabs.Selected() != nil || tabs.SelectedIndex() != -1 {
		t.Fatal("empty tabs have a selection")
	}
	is := items("a", "b", "c")
	for _, it := range is {
		tabs.Append(it)
	}
	if fmt.Sprint(selected) != "[a]" || tabs.Selected() != is[0] {
		t.Fatalf("after appending three tabs: selected %v, events %v", tabs.Selected(), selected)
	}
	tabs.Select(is[2])
	tabs.SelectIndex(2) // already selected: no event
	tabs.SelectIndex(7) // out of range: ignored
	if fmt.Sprint(selected) != "[a c]" || tabs.SelectedIndex() != 2 {
		t.Fatalf("Select: events %v, index %d", selected, tabs.SelectedIndex())
	}

	// Removing another tab keeps the selected one (Fyne 2.8.1's DocTabs selects the wrong tab
	// when one to the left of the selected last tab is removed).
	before := tabs.Items
	tabs.Remove(is[0])
	if tabs.Selected() != is[2] || len(tabs.Items) != 2 || len(selected) != 2 {
		t.Fatalf("removing an unselected tab: selected %v, items %d, events %v", tabs.Selected().Text, len(tabs.Items), selected)
	}
	if before[0] != is[0] {
		t.Error("Remove changed the slice a caller was holding")
	}
	// Removing the selected tab selects the one now in its place, or the last.
	tabs.Remove(is[2])
	if tabs.Selected() != is[1] || fmt.Sprint(selected) != "[a c b]" {
		t.Fatalf("removing the selected tab: selected %v, events %v", tabs.Selected(), selected)
	}
	tabs.Remove(is[1])
	if tabs.Selected() != nil || tabs.SelectedIndex() != -1 || len(tabs.Items) != 0 {
		t.Fatal("removing the last tab left a selection")
	}
	tabs.Remove(is[1]) // not there: nothing happens
}

func TestTabsClose(t *testing.T) {
	test.NewTempApp(t)
	is := items("boot.asm", "kernel.c")
	tabs := NewDocTabs(is...)
	w := test.NewTempWindow(t, tabs)
	w.Resize(fyne.NewSize(400, 200))
	var closed []string
	tabs.OnClosed = func(it *container.TabItem) { closed = append(closed, it.Text) }

	b := tabs.button(is[0])
	pos, size := b.closeRect()
	test.TapAt(b, pos.Add(fyne.NewPos(size.Width/2, size.Height/2)))
	if len(tabs.Items) != 1 || fmt.Sprint(closed) != "[boot.asm]" {
		t.Fatalf("tapping the close mark: items %d, closed %v", len(tabs.Items), closed)
	}

	// Middle click closes; CloseIntercept replaces the removal (the editor asks Neovim first).
	var intercepted *container.TabItem
	tabs.CloseIntercept = func(it *container.TabItem) { intercepted = it }
	tabs.button(is[1]).MouseDown(&desktop.MouseEvent{Button: desktop.MouseButtonTertiary})
	if intercepted != is[1] || len(tabs.Items) != 1 {
		t.Fatalf("CloseIntercept: %v, items %d", intercepted, len(tabs.Items))
	}

	// Tapping the label selects; app tabs have no close mark.
	app := NewTabs(items("Terminal", "Build")...)
	test.NewTempWindow(t, app).Resize(fyne.NewSize(400, 200))
	taps := 0
	app.OnSelected = func(*container.TabItem) { taps++ }
	test.Tap(app.button(app.Items[1]))
	if app.SelectedIndex() != 1 {
		t.Error("tapping a tab did not select it")
	}
	test.Tap(app.button(app.Items[1])) // the selected tab: still reported, so its region activates
	if taps != 2 {
		t.Errorf("OnSelected ran %d times for two taps", taps)
	}
	if app.button(app.Items[0]).onClose(fyne.NewPos(1e6, 1e6)) {
		t.Error("app tabs have a close mark")
	}
}

// TestTabsAccent: the selected tab's label and underline are in the accent only while the
// region is active (decision A1), and moving the accent does not grow memory.
func TestTabsAccent(t *testing.T) {
	app := test.NewTempApp(t)
	is := items("Terminal", "Build")
	tabs := NewTabs(is...)
	w := test.NewTempWindow(t, tabs)
	w.Resize(fyne.NewSize(400, 200))
	th, v := app.Settings().Theme(), app.Settings().ThemeVariant()
	primary, fg := th.Color(fynetheme.ColorNamePrimary, v), th.Color(fynetheme.ColorNameForeground, v)
	r := test.WidgetRenderer(tabs).(*tabsRenderer)
	label := func(i int) any { return test.WidgetRenderer(tabs.button(is[i])).(*tabButtonRenderer).label.Color }

	if r.indicator.FillColor != fg || label(0) != fg || label(1) != fg {
		t.Error("an inactive region's tabs are in the accent")
	}
	tabs.SetAccent(true)
	if !tabs.Accent() || r.indicator.FillColor != primary || label(0) != primary || label(1) != fg {
		t.Error("the active region's selected tab is not in the accent, or another tab is")
	}

	heap := func() uint64 {
		runtime.GC()
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		return m.HeapAlloc
	}
	_ = w.Canvas().Capture()
	start := heap()
	for i := range 50 {
		tabs.SetAccent(i%2 == 0)
		_ = w.Canvas().Capture()
	}
	if grew := int64(heap()) - int64(start); grew > 4<<20 {
		t.Errorf("50 accent changes grew the heap by %d MB", grew>>20)
	}
}
