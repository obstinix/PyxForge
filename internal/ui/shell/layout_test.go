package shell

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"github.com/obstinix/PyxForge/internal/ui/kit"
	"github.com/obstinix/PyxForge/internal/ui/theme"
	"github.com/obstinix/PyxForge/internal/workspace"
)

// TestPanelsResize (PXF-DEF-007, PXF-DEF-016): the sashes beside the explorer, above the dock
// and beside the inspector resize them by dragging, within limits that keep the editor usable;
// a double click restores the default; the sizes come back with the workspace.
func TestPanelsResize(t *testing.T) {
	a := test.NewTempApp(t)
	root := t.TempDir()
	store := &workspace.StateStore{Dir: t.TempDir()}
	s := NewWithOptions(a, root, Options{State: store})
	noToastExpiry(s)
	s.win.Resize(fyne.NewSize(1600, 1000))
	objs := s.content.Objects
	width := func(i int) float32 { return objs[i].Size().Width }
	height := func(i int) float32 { return objs[i].Size().Height }
	drag := func(i int, dx, dy float32) {
		objs[i].(*kit.Sash).Dragged(&fyne.DragEvent{Dragged: fyne.NewDelta(dx, dy)})
		objs[i].(*kit.Sash).DragEnd()
	}
	if width(objExplorer) != theme.ExplorerWidth || height(objDock) != theme.DockHeight || width(objInspector) != theme.InspectorWidth {
		t.Fatalf("defaults: explorer %v, dock %v, inspector %v", width(objExplorer), height(objDock), width(objInspector))
	}

	drag(objExplorerRule, 100, 0)
	drag(objDockRule, 0, -150) // up: a taller dock
	drag(objInspectorRule, -60, 0)
	if width(objExplorer) != theme.ExplorerWidth+100 || height(objDock) != theme.DockHeight+150 ||
		width(objInspector) != theme.InspectorWidth+60 {
		t.Fatalf("after dragging: explorer %v, dock %v, inspector %v", width(objExplorer), height(objDock), width(objInspector))
	}

	// Limits: the panels stop at their minimum, and the editor keeps its own.
	drag(objExplorerRule, -1000, 0)
	drag(objDockRule, 0, 1000)
	if width(objExplorer) != minExplorer || height(objDock) != minDock {
		t.Errorf("at the minimum: explorer %v, dock %v", width(objExplorer), height(objDock))
	}
	drag(objExplorerRule, 5000, 0)
	drag(objDockRule, 0, -5000)
	if width(objEditor) < minEditorW || height(objEditor) < minEditorH {
		t.Errorf("the editor shrank to %v×%v", width(objEditor), height(objEditor))
	}
	drag(objInspectorRule, -5000, 0)
	if width(objInspector) != maxInspector {
		t.Errorf("inspector at its maximum: %v", width(objInspector))
	}

	// A double click gives the default back.
	objs[objExplorerRule].(*kit.Sash).DoubleTapped(nil)
	if width(objExplorer) != theme.ExplorerWidth {
		t.Errorf("after reset: explorer %v", width(objExplorer))
	}

	// The sizes come back with the workspace.
	drag(objExplorerRule, 40, 0)
	wantDock := height(objDock)
	s.saveState()
	r := NewWithOptions(a, root, Options{State: store})
	noToastExpiry(r)
	r.win.Resize(fyne.NewSize(1600, 1000))
	ro := r.content.Objects
	if ro[objExplorer].Size().Width != theme.ExplorerWidth+40 || ro[objDock].Size().Height != wantDock ||
		ro[objInspector].Size().Width != maxInspector {
		t.Errorf("restored: explorer %v, dock %v, inspector %v", ro[objExplorer].Size().Width, ro[objDock].Size().Height,
			ro[objInspector].Size().Width)
	}
}
