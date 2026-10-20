package shell

import (
	"fyne.io/fyne/v2"
	"github.com/obstinix/PyxForge/internal/ui/kit"
	"github.com/obstinix/PyxForge/internal/ui/theme"
)

// DockBreakpoint is the window width below which the inspector floats instead of docking
// (Section 11.6).
const DockBreakpoint = 1280

// Indices of the workbench's objects, in paint order. The sashes come after the panels they
// separate, so their whole grip, which overlaps both panels, is on top and takes the drag.
const (
	objRail = iota
	objRailRule
	objExplorer
	objEditor
	objDock
	objInspector
	objExplorerRule // the sash between the explorer and the editor
	objDockRule     // the sash between the editor and the dock
	objInspectorRule
	objStatusRule
	objStatus
	objNotifications
	objCount
)

// Limits on the resizable panels (PXF-DEF-007): the editor keeps room to work in.
const (
	minExplorer   = 160
	minInspector  = 220
	maxInspector  = 600
	minDock       = 72 // a tab row and three terminal lines
	minEditorW    = 360
	minEditorH    = 160
	sashHalfWidth = (kit.SashGrip - 1) / 2
)

// workbench lays out the window: rail, explorer, editor over dock, inspector, status bar.
// Panels are flush and separated by 1 px rules (DESIGN_SYSTEM.md §8); the rules beside the
// explorer, the dock and the inspector are sashes the user drags to resize them.
type workbench struct {
	explorerOn, dockOn, inspectorOn bool
	inspector                       *sidePanel
	width                           float32 // last laid-out width, for the narrow rule
	// onBreakpoint runs when the window crosses DockBreakpoint, before the inspector is placed:
	// docks collapse below the breakpoint and reopen as overlays (Section 11.6).
	onBreakpoint func(narrow bool)

	// The sizes the user chose by dragging; 0 means the default. Layout keeps them inside the
	// limits for the window's size and records what it used in the laid-out fields.
	explorerW, dockH, inspectorW    float32
	laidExplorer, laidDock, laidIns float32
}

func clamp(v, lo, hi float32) float32 { return max(lo, min(v, hi)) }

func orDefault(v, d float32) float32 {
	if v <= 0 {
		return d
	}
	return v
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

	floating := s.Width < DockBreakpoint
	if l.inspector.floating != floating {
		if l.onBreakpoint != nil {
			l.onBreakpoint(floating)
		}
		l.inspector.floating = floating
		l.inspector.Refresh()
	}
	docked := l.inspectorOn && !floating
	iw := float32(0)
	if l.inspectorOn {
		iw = clamp(orDefault(l.inspectorW, theme.InspectorWidth), minInspector, maxInspector)
	}

	x := float32(0)
	show(objRail, x, 0, theme.RailWidth, h)
	show(objRailRule, theme.RailWidth, 0, 1, h)
	x = theme.RailWidth + 1
	if l.explorerOn {
		room := s.Width - x - 1 - minEditorW // the explorer's rule takes a pixel
		if docked {
			room -= iw
		}
		ew := clamp(orDefault(l.explorerW, theme.ExplorerWidth), minExplorer, max(minExplorer, room))
		l.laidExplorer = ew
		show(objExplorer, x, 0, ew, h)
		show(objExplorerRule, x+ew-sashHalfWidth, 0, kit.SashGrip, h)
		x += ew + 1
	} else {
		o[objExplorer].Hide()
		o[objExplorerRule].Hide()
	}

	right := s.Width
	switch {
	case !l.inspectorOn:
		o[objInspector].Hide()
		o[objInspectorRule].Hide()
	case floating:
		// Over the editor, clear of the rail and status bar, as a glass panel.
		show(objInspector, s.Width-theme.InspectorWidth-theme.Space2, theme.Space2,
			theme.InspectorWidth, h-2*theme.Space2)
		o[objInspectorRule].Hide()
	default:
		right -= iw
		l.laidIns = iw
		show(objInspector, right, 0, iw, h)
		show(objInspectorRule, right-sashHalfWidth, 0, kit.SashGrip, h)
	}

	w := right - x
	if l.dockOn {
		dock := clamp(orDefault(l.dockH, theme.DockHeight), minDock, max(minDock, h-1-minEditorH))
		l.laidDock = dock
		show(objEditor, x, 0, w, h-dock-1)
		show(objDockRule, x, h-dock-1-sashHalfWidth, w, kit.SashGrip)
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
		w += minExplorer + 1
	}
	return fyne.NewSize(w, 400)
}
