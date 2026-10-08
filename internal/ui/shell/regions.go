package shell

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	fynetheme "fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// region is one keyboard-navigable part of the workbench. Exactly one is active: the one that
// holds keyboard focus or was last used. Its active tab is the only tab drawn in the accent
// (decision A1); every other tab bar stays in the text colour.
type region int

const (
	regionEditor region = iota
	regionExplorer
	regionPanel
	regionInspector
)

// activate makes r the active region and moves the accent to its tab bar.
func (s *Shell) activate(r region) {
	s.active = r
	for reg, ov := range s.tabThemes {
		on := reg == r
		if t, ok := ov.Theme.(tabTheme); ok && t.accent == on {
			continue
		}
		ov.Theme = tabTheme{accent: on}
		ov.Refresh()
	}
}

// regionTabs wraps a region's tab container so its active tab takes the accent only while the
// region is active.
func (s *Shell) regionTabs(r region, tabs fyne.CanvasObject) fyne.CanvasObject {
	ov := container.NewThemeOverride(tabs, tabTheme{accent: r == s.active})
	s.tabThemes[r] = ov
	return ov
}

// tabTheme draws a tab bar's active label and underline in the accent when its region is
// active, and in the text colour otherwise. Fyne colours both with Primary.
type tabTheme struct{ accent bool }

func (tabTheme) current() fyne.Theme { return fyne.CurrentApp().Settings().Theme() }

func (q tabTheme) Color(n fyne.ThemeColorName, v fyne.ThemeVariant) color.Color {
	if n == fynetheme.ColorNamePrimary && !q.accent {
		n = fynetheme.ColorNameForeground
	}
	return q.current().Color(n, v)
}
func (q tabTheme) Font(s fyne.TextStyle) fyne.Resource     { return q.current().Font(s) }
func (q tabTheme) Icon(n fyne.ThemeIconName) fyne.Resource { return q.current().Icon(n) }
func (q tabTheme) Size(n fyne.ThemeSizeName) float32       { return q.current().Size(n) }

// regionArea sits under a region's content and activates the region when the user clicks
// anywhere in it that is not itself interactive (placeholder text, empty panel space).
type regionArea struct {
	widget.BaseWidget
	onTap func()
}

func newRegionArea(content fyne.CanvasObject, onTap func()) fyne.CanvasObject {
	a := &regionArea{onTap: onTap}
	a.ExtendBaseWidget(a)
	return container.NewStack(a, content)
}

func (a *regionArea) Tapped(*fyne.PointEvent) { a.onTap() }

func (a *regionArea) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(container.NewWithoutLayout())
}

// FocusExplorer shows the explorer and gives its tree keyboard focus.
func (s *Shell) FocusExplorer() {
	if !s.bench.explorerOn {
		s.bench.explorerOn = true
		s.relayout()
	}
	s.explorer.Focus(s.win.Canvas())
	s.activate(regionExplorer)
}

// FocusEditor makes the editor the active region. In Settings, keyboard focus moves to the
// first theme card.
func (s *Shell) FocusEditor() {
	c := s.win.Canvas()
	if s.settings != nil && s.editors.Selected() == s.settings && len(s.themeCards) > 0 {
		c.Focus(s.themeCards[0])
	} else {
		c.Unfocus()
	}
	s.activate(regionEditor)
}

// FocusPanel shows the bottom panel and makes it the active region. On the Log tab, the log
// list takes keyboard focus.
func (s *Shell) FocusPanel() {
	if !s.bench.dockOn {
		s.bench.dockOn = true
		s.relayout()
	}
	if s.dock.Selected() == s.logTab {
		s.win.Canvas().Focus(s.logList)
	} else {
		s.win.Canvas().Unfocus()
	}
	s.activate(regionPanel)
}

// FocusInspector shows the inspector and makes it the active region.
func (s *Shell) FocusInspector() {
	if !s.bench.inspectorOn {
		s.inspectorAuto = false
		s.bench.inspectorOn = true
		s.relayout()
	}
	s.win.Canvas().Unfocus()
	s.activate(regionInspector)
}

// showDockTab opens the bottom panel on one of its tabs.
func (s *Shell) showDockTab(i int) {
	s.dock.SelectIndex(i)
	s.FocusPanel()
}

// showInspectorTab opens the inspector on one of its tabs.
func (s *Shell) showInspectorTab(i int) {
	s.inspectorTabs.SelectIndex(i)
	s.FocusInspector()
}

// cycleTab moves to the next (step 1) or previous (step -1) tab of the active region, wrapping.
// The explorer has no tabs, so there it cycles the editor's.
func (s *Shell) cycleTab(step int) {
	next := func(i, n int) int { return ((i+step)%n + n) % n }
	switch s.active {
	case regionPanel:
		s.dock.SelectIndex(next(s.dock.SelectedIndex(), len(s.dock.Items)))
	case regionInspector:
		s.inspectorTabs.SelectIndex(next(s.inspectorTabs.SelectedIndex(), len(s.inspectorTabs.Items)))
	default:
		if n := len(s.editors.Items); n > 0 {
			s.editors.SelectIndex(next(s.editors.SelectedIndex(), n))
		}
	}
}

// logList is Fyne's list with a focus callback, for the same reason as the explorer's tree.
type logList struct {
	widget.List
	onFocus func(bool)
}

func (l *logList) FocusGained() {
	l.List.FocusGained()
	l.onFocus(true)
}

func (l *logList) FocusLost() {
	l.List.FocusLost()
	l.onFocus(false)
}
