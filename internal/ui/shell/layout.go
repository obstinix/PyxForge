package shell

import (
	"fyne.io/fyne/v2"
	"github.com/obstinix/PyxForge/internal/ui/theme"
)

// DockBreakpoint is the window width below which the inspector floats instead of docking
// (Section 11.6).
const DockBreakpoint = 1280

// Indices of the workbench's objects, in paint order.
const (
	objRail = iota
	objRailRule
	objExplorer
	objExplorerRule
	objEditor
	objDockRule
	objDock
	objInspector
	objStatusRule
	objStatus
	objNotifications
	objCount
)

// workbench lays out the window: rail, explorer, editor over dock, inspector, status bar.
// Panels are flush and separated by 1 px rules (DESIGN_SYSTEM.md §8).
type workbench struct {
	explorerOn, dockOn, inspectorOn bool
	inspector                       *sidePanel
	width                           float32 // last laid-out width, for the narrow rule
	// onBreakpoint runs when the window crosses DockBreakpoint, before the inspector is placed:
	// docks collapse below the breakpoint and reopen as overlays (Section 11.6).
	onBreakpoint func(narrow bool)
}

func (l *workbench) Layout(o []fyne.CanvasObject, s fyne.Size) {
	l.width = s.Width
	show := func(i int, x, y, w, h float32) {
		o[i].Move(fyne.NewPos(x, y))
		o[i].Resize(fyne.NewSize(w, h))
		o[i].Show()
	}
	h := s.Height - theme.StatusBarHeight - 1
	show(objStatusRule, 0, h, s.Width, 1)
	show(objStatus, 0, h+1, s.Width, theme.StatusBarHeight)
	show(objNotifications, 0, 0, s.Width, s.Height)

	x := float32(0)
	show(objRail, x, 0, theme.RailWidth, h)
	show(objRailRule, theme.RailWidth, 0, 1, h)
	x = theme.RailWidth + 1
	if l.explorerOn {
		show(objExplorer, x, 0, theme.ExplorerWidth, h)
		show(objExplorerRule, x+theme.ExplorerWidth, 0, 1, h)
		x += theme.ExplorerWidth + 1
	} else {
		o[objExplorer].Hide()
		o[objExplorerRule].Hide()
	}

	right := s.Width
	floating := s.Width < DockBreakpoint
	if l.inspector.floating != floating {
		if l.onBreakpoint != nil {
			l.onBreakpoint(floating)
		}
		l.inspector.floating = floating
		l.inspector.Refresh()
	}
	switch {
	case !l.inspectorOn:
		o[objInspector].Hide()
	case floating:
		// Over the editor, clear of the rail and status bar, as a glass panel.
		show(objInspector, s.Width-theme.InspectorWidth-theme.Space2, theme.Space2,
			theme.InspectorWidth, h-2*theme.Space2)
	default:
		right -= theme.InspectorWidth
		show(objInspector, right, 0, theme.InspectorWidth, h)
	}

	w := right - x
	if l.dockOn {
		dock := min(theme.DockHeight, h*0.4)
		show(objEditor, x, 0, w, h-dock-1)
		show(objDockRule, x, h-dock-1, w, 1)
		show(objDock, x, h-dock, w, dock)
	} else {
		show(objEditor, x, 0, w, h)
		o[objDockRule].Hide()
		o[objDock].Hide()
	}
}

func (l *workbench) MinSize([]fyne.CanvasObject) fyne.Size {
	w := theme.RailWidth + 1 + 420
	if l.explorerOn {
		w += theme.ExplorerWidth + 1
	}
	return fyne.NewSize(w, 400)
}
