package shell

import (
	"fmt"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// Editor tab management: the tab's right-click menu, pinning, moving and closing tabs in bulk.
// Bulk closes (Close Others, Close to the Right, Close Saved, Close All) leave pinned tabs open;
// closing one tab by name (Close, Ctrl+W) closes it even when it is pinned.

// tabMenu is the menu for a right click on an editor tab.
func (s *Shell) tabMenu(it *container.TabItem) *fyne.Menu {
	g := s.groupOf(it)
	if g == nil {
		return fyne.NewMenu("")
	}
	pin := "Pin"
	if g.Pinned(it) {
		pin = "Unpin"
	}
	items := []*fyne.MenuItem{
		fyne.NewMenuItem("Close", func() { g.CloseIntercept(it) }),
		fyne.NewMenuItem("Close Others", func() { s.closeOthers(it) }),
		fyne.NewMenuItem("Close to the Right", func() { s.closeToTheRight(it) }),
		fyne.NewMenuItem("Close Saved", s.closeSaved),
		fyne.NewMenuItem("Close All", s.closeAllTabs),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem(pin, func() { g.SetPinned(it, !g.Pinned(it)) }),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("Split Right", func() { s.moveToGroup(it, s.groupIndex(it)+1) }),
	}
	if i := s.groupIndex(it); i > 0 {
		items = append(items, fyne.NewMenuItem("Move to Left Group", func() { s.moveToGroup(it, i-1) }))
	}
	if p := s.tabFile(it); p != "" {
		items = append(items, fyne.NewMenuItemSeparator(),
			fyne.NewMenuItem("Copy Path", func() { s.app.Clipboard().SetContent(p) }))
		if rel, err := filepath.Rel(s.root, p); err == nil && s.root != "" && !strings.HasPrefix(rel, "..") {
			items = append(items, fyne.NewMenuItem("Copy Relative Path", func() { s.app.Clipboard().SetContent(filepath.ToSlash(rel)) }))
		}
		items = append(items, fyne.NewMenuItem(revealLabel(), func() { s.revealInFileManager(p, false) }))
	}
	return fyne.NewMenu("", items...)
}

// showTabMenu opens a tab's menu at the pointer.
func (s *Shell) showTabMenu(it *container.TabItem, at fyne.Position) {
	widget.ShowPopUpMenuAtPosition(s.tabMenu(it), s.win.Canvas(), at)
}

// unpinned lists the editor tabs that a bulk close may close, among items.
func (s *Shell) unpinned(items []*container.TabItem) []*container.TabItem {
	var out []*container.TabItem
	for _, it := range items {
		if g := s.groupOf(it); g != nil && !g.Pinned(it) {
			out = append(out, it)
		}
	}
	return out
}

// closeOthers closes the other tabs of keep's group.
func (s *Shell) closeOthers(keep *container.TabItem) {
	if g := s.groupOf(keep); g != nil {
		s.closeTabs(s.unpinned(slices.DeleteFunc(slices.Clone(g.Items), func(it *container.TabItem) bool { return it == keep })))
	}
}

// closeToTheRight closes the tabs after of in its group.
func (s *Shell) closeToTheRight(of *container.TabItem) {
	g := s.groupOf(of)
	if g == nil {
		return
	}
	s.closeTabs(s.unpinned(slices.Clone(g.Items[slices.Index(g.Items, of)+1:])))
}

// closeSaved closes the tabs without unsaved changes, so it never asks.
func (s *Shell) closeSaved() {
	var saved []*container.TabItem
	for _, it := range s.unpinned(s.allTabs()) {
		if d := s.docOf(it); d == nil || !d.ed.Buf.Modified() {
			saved = append(saved, it)
		}
	}
	s.closeTabs(saved)
}

// closeAllTabs closes every editor tab, in every group, but the pinned ones (File > Close All,
// Ctrl+Shift+W). Unsaved changes are offered for saving first.
func (s *Shell) closeAllTabs() { s.closeTabs(s.unpinned(s.allTabs())) }

// docOf is the document an editor tab shows, or nil for other tabs.
func (s *Shell) docOf(it *container.TabItem) *nativeDoc {
	if s.nat == nil {
		return nil
	}
	return s.nat.docs[it]
}

// closeTabs closes several editor tabs, asking once about the unsaved changes among them:
// Save writes them first and closes nothing if one could not be written; Don't Save closes
// them all.
func (s *Shell) closeTabs(items []*container.TabItem) {
	items = slices.Clone(items)
	var mod []*nativeDoc
	for _, it := range items {
		if d := s.docOf(it); d != nil && d.ed.Buf.Modified() {
			mod = append(mod, d)
		}
	}
	closeAll := func() {
		for _, it := range items {
			if s.groupOf(it) == nil {
				continue // closed meanwhile
			}
			if d := s.docOf(it); d != nil {
				s.nat.remove(d)
				continue
			}
			s.removeTab(it)
		}
	}
	if len(mod) == 0 {
		closeAll()
		return
	}
	sort.Slice(mod, func(i, j int) bool { return mod[i].name() < mod[j].name() })
	var names []string
	for _, d := range mod {
		names = append(names, displayName(d.path))
	}
	s.confirmUnsaved(fmt.Sprintf("Save changes to %s?", strings.Join(names, ", ")), func() {
		ok := true
		for _, d := range mod {
			ok = s.nat.save(d) && ok
		}
		if ok {
			closeAll()
		}
	}, closeAll)
}

// togglePinTab pins or unpins the selected editor tab.
func (s *Shell) togglePinTab() {
	if it := s.editors.Selected(); it != nil {
		s.editors.SetPinned(it, !s.editors.Pinned(it))
	}
}

// moveTab moves the selected editor tab by delta places, within its group.
func (s *Shell) moveTab(delta int) {
	if it := s.editors.Selected(); it != nil {
		s.editors.MoveTab(it, s.editors.SelectedIndex()+delta)
	}
}

// tabFor is the editor tab showing path, or nil.
func (s *Shell) tabFor(path string) *container.TabItem {
	for _, it := range s.allTabs() {
		if p := s.tabFile(it); p != "" && sameFile(p, path) {
			return it
		}
	}
	return nil
}
