package shell

import (
	"slices"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"

	"github.com/obstinix/PyxForge/internal/ui/kit"
	"github.com/obstinix/PyxForge/internal/ui/notifications"
)

// Editor groups: the editor area split side by side into up to maxGroups tab groups. A file is
// open in at most one group (owner's decision, 2026-11-27): moving a tab to another group takes
// it out of the one it was in, so one document never needs two views.
//
// s.editors is always the active group, the one keyboard commands act on. Code that means
// every tab uses allTabs, groupOf and removeTab instead.

// maxGroups is how many groups the editor area holds side by side.
const maxGroups = 3

// newGroup makes an editor tab group wired to the shell's handlers.
func (s *Shell) newGroup() *kit.Tabs {
	g := kit.NewDocTabs()
	// A tab the user picks makes its group and the editor active and takes keyboard focus. A tab
	// the shell selects quietly (a preview) is only shown: where focus goes then is decided by
	// whoever asked for the file (selectTab).
	g.OnSelected = func(it *container.TabItem) {
		quiet := s.selectQuietly
		s.setActiveGroup(g)
		if !quiet {
			s.activate(regionEditor)
		}
		if s.nat != nil {
			s.nat.selected(it, !quiet)
		}
		if it == s.welcome {
			s.win.Canvas().Unfocus()
		}
		s.explorer.SetActive(s.activeFile())
		s.graphs.syncLocal() // the Local Graph follows the active file
	}
	// A document's tab asks about unsaved changes before it closes; other tabs
	// (Settings, placeholders) close at once.
	g.CloseIntercept = func(it *container.TabItem) {
		if d := s.docOf(it); d != nil {
			s.nat.close(d)
			return
		}
		s.removeTab(it)
	}
	g.OnClosed = s.tabClosed
	g.OnContextMenu = s.showTabMenu
	return g
}

// setActiveGroup makes g the group keyboard commands act on; it takes the editor region's
// accent from the group that had it.
func (s *Shell) setActiveGroup(g *kit.Tabs) {
	if s.editors == g {
		return
	}
	if s.editors != nil {
		s.editors.SetAccent(false)
	}
	s.editors = g
	s.regionBars[regionEditor] = g
	g.SetAccent(s.active == regionEditor)
}

// allTabs lists every editor tab, group by group.
func (s *Shell) allTabs() []*container.TabItem {
	var out []*container.TabItem
	for _, g := range s.groups {
		out = append(out, g.Items...)
	}
	return out
}

// groupOf is the group holding a tab, or nil.
func (s *Shell) groupOf(it *container.TabItem) *kit.Tabs {
	for _, g := range s.groups {
		if slices.Contains(g.Items, it) {
			return g
		}
	}
	return nil
}

// removeTab closes a tab in whatever group holds it, without asking.
func (s *Shell) removeTab(it *container.TabItem) {
	g := s.groupOf(it)
	if g == nil {
		return
	}
	g.Remove(it)
	s.tabClosed(it)
}

// tabClosed forgets what the shell kept about a closed tab, and closes its group if it was
// the group's last tab and other groups remain.
func (s *Shell) tabClosed(it *container.TabItem) {
	if it == s.welcome {
		s.welcome, s.welcomeRecent = nil, nil
	}
	if s.graphs != nil && it == s.graphs.item {
		s.graphs.view.StopLayout() // the layout stops with its tab; reopening starts it again
	}
	for p, t := range s.open {
		if t == it {
			delete(s.open, p)
		}
	}
	for p, v := range s.diffs {
		if v.item == it {
			delete(s.diffs, p)
		}
	}
	s.collapseEmptyGroups()
	s.syncEditor()
}

// collapseEmptyGroups removes groups without tabs while more than one group is left.
func (s *Shell) collapseEmptyGroups() {
	changed := false
	for i := len(s.groups) - 1; i >= 0 && len(s.groups) > 1; i-- {
		if len(s.groups[i].Items) == 0 {
			g := s.groups[i]
			s.groups = slices.Delete(s.groups, i, i+1)
			if s.editors == g {
				s.setActiveGroup(s.groups[min(i, len(s.groups)-1)])
			}
			changed = true
		}
	}
	if changed {
		s.layoutGroups()
	}
}

// layoutGroups lays the groups out side by side, divided by hairline rules.
func (s *Shell) layoutGroups() {
	var objs []fyne.CanvasObject
	for i, g := range s.groups {
		if i > 0 {
			objs = append(objs, kit.NewRule(true))
		}
		objs = append(objs, g)
	}
	s.groupRow.Objects = objs
	s.groupRow.Refresh()
}

// moveToGroup moves a tab into the group at index to (len(s.groups) adds a group on the right),
// selects it there and makes that group active. A pinned tab stays pinned.
func (s *Shell) moveToGroup(it *container.TabItem, to int) {
	from := s.groupOf(it)
	if from == nil || to < 0 || to > len(s.groups) || (to < len(s.groups) && s.groups[to] == from) {
		return
	}
	if to == len(s.groups) {
		if len(s.groups) == maxGroups {
			s.Notify(notifications.Info, "No room for another group", "The editor holds at most three groups side by side.")
			return
		}
		if len(from.Items) == 1 {
			s.Notify(notifications.Info, "Only one tab in this group", "A file is open in one group at a time: open another file here first, then split.")
			return
		}
		s.groups = append(s.groups, s.newGroup())
		s.layoutGroups()
	}
	dest := s.groups[to]
	pinned := from.Pinned(it)
	s.selectQuietly = true
	from.Remove(it) // before it is shown in dest: from hides the content it no longer shows
	dest.Append(it)
	s.selectQuietly = false
	if pinned {
		dest.SetPinned(it, true)
	}
	s.collapseEmptyGroups()
	s.selectTab(it, true)
}

// splitRight moves the selected tab to the group on the right of its own, adding one if need be.
func (s *Shell) splitRight() {
	if it := s.editors.Selected(); it != nil {
		s.moveToGroup(it, slices.Index(s.groups, s.editors)+1)
	}
}

// moveLeft moves the selected tab to the group on the left of its own.
func (s *Shell) moveLeft() {
	if it := s.editors.Selected(); it != nil {
		if i := slices.Index(s.groups, s.editors); i > 0 {
			s.moveToGroup(it, i-1)
		}
	}
}

// focusGroup makes the next (step 1) or previous (step -1) group active, with keyboard focus
// in its selected tab.
func (s *Shell) focusGroup(step int) {
	n := len(s.groups)
	if n < 2 {
		return
	}
	i := slices.Index(s.groups, s.editors)
	g := s.groups[((i+step)%n+n)%n]
	if it := g.Selected(); it != nil {
		s.selectTab(it, true)
	}
}

// joinGroups moves every tab into the first group, in group order.
func (s *Shell) joinGroups() {
	if len(s.groups) < 2 {
		return
	}
	sel := s.editors.Selected()
	first := s.groups[0]
	s.selectQuietly = true
	for _, g := range s.groups[1:] {
		for _, it := range slices.Clone(g.Items) {
			pinned := g.Pinned(it)
			g.Remove(it)
			first.Append(it)
			if pinned {
				first.SetPinned(it, true)
			}
		}
	}
	s.selectQuietly = false
	s.groups = s.groups[:1]
	s.setActiveGroup(first)
	s.layoutGroups()
	if sel != nil {
		s.selectTab(sel, true)
	}
}

// showTab selects a tab in its group as a user's pick would: the group and the editor become
// active (selectTab is the quiet form).
func (s *Shell) showTab(it *container.TabItem) {
	if g := s.groupOf(it); g != nil {
		s.setActiveGroup(g)
		g.Select(it)
	}
}

// groupIndex is the index of the group holding a tab, or -1.
func (s *Shell) groupIndex(it *container.TabItem) int {
	return slices.Index(s.groups, s.groupOf(it))
}

// groupsLayout lays the groups and their rules out in a row: rules at their minimum width, the
// groups sharing the rest equally.
type groupsLayout struct{}

func (groupsLayout) Layout(objs []fyne.CanvasObject, size fyne.Size) {
	var rules float32
	groups := 0
	for _, o := range objs {
		if _, ok := o.(*kit.Tabs); ok {
			groups++
		} else {
			rules += o.MinSize().Width
		}
	}
	if groups == 0 {
		return
	}
	w := (size.Width - rules) / float32(groups)
	x := float32(0)
	for _, o := range objs {
		ow := w
		if _, ok := o.(*kit.Tabs); !ok {
			ow = o.MinSize().Width
		}
		o.Move(fyne.NewPos(x, 0))
		o.Resize(fyne.NewSize(ow, size.Height))
		x += ow
	}
}

func (groupsLayout) MinSize(objs []fyne.CanvasObject) fyne.Size {
	var s fyne.Size
	for _, o := range objs {
		m := o.MinSize()
		s.Width += m.Width
		s.Height = max(s.Height, m.Height)
	}
	return s
}
