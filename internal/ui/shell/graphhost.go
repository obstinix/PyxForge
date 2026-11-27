package shell

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"

	"github.com/obstinix/PyxForge/internal/config"
	"github.com/obstinix/PyxForge/internal/graph"
	"github.com/obstinix/PyxForge/internal/ui/graphview"
	"github.com/obstinix/PyxForge/internal/ui/icons"
	"github.com/obstinix/PyxForge/internal/ui/kit"
	"github.com/obstinix/PyxForge/internal/ui/notifications"
	"github.com/obstinix/PyxForge/internal/ui/theme"
)

// graphHost is the project graph in the shell (D23, identity prompt Phase 5): the Graph view,
// an editor tab, and the Local Graph, a tab of the inspector that shows the current file's
// neighbourhood. One graph serves both; the explorer's watcher keeps it current.
type graphHost struct {
	s        *Shell
	g        *graph.Graph
	building bool
	waiting  []func() // what to do once the graph is built

	view     *graphview.View
	controls *graphview.Controls
	item     *container.TabItem
	body     *fyne.Container // the graph and its controls, or a message while it builds

	local      *graphview.View
	localTab   *container.TabItem
	localDepth *kit.Slider
	localNote  *kit.Text
	localFor   string // the file the Local Graph shows
}

// graphSeed fixes the layout's start, so the same project lays out the same way.
const graphSeed = 1

func (s *Shell) graphHost() *graphHost {
	if s.graphs == nil {
		s.graphs = &graphHost{s: s}
	}
	return s.graphs
}

// withGraph runs f once the project graph is built, building it off the UI thread first.
func (h *graphHost) withGraph(f func()) {
	if h.s.root == "" {
		h.s.Notify(notifications.Info, "No folder open", "The graph shows a project's files: open a folder first.")
		return
	}
	if h.g != nil {
		f()
		return
	}
	h.waiting = append(h.waiting, f)
	if h.building {
		return
	}
	h.building = true
	root := h.s.root
	var include []string
	if cfg, err := config.Load(filepath.Join(root, "pyxforge.toml")); err == nil {
		include = graph.IncludeDirs(cfg)
	}
	go func() {
		g, err := graph.Build(root, graph.Options{IncludeDirs: include})
		h.s.dispatch(func() {
			h.building = false
			if err != nil {
				h.s.Notify(notifications.Error, "Cannot build the graph", err.Error())
				h.waiting = nil
				return
			}
			h.g = g
			h.s.logf("Graph: %d files, %d links, %d references not resolved", len(g.Nodes), len(g.Edges), len(g.Unresolved))
			for _, w := range h.waiting {
				w()
			}
			h.waiting = nil
		})
	}()
}

// filesChanged is the explorer's watcher reporting changes: the graph re-reads those files.
func (h *graphHost) filesChanged(paths []string) {
	if h == nil || h.g == nil || !h.g.Update(paths) {
		return
	}
	if h.view != nil && h.s.hasTab(h.item) {
		h.view.SetGraph(h.g, graphSeed)
		h.controls.Rebuild()
	}
	h.localFor = "" // the neighbourhood may have changed
	h.syncLocal()
}

// openGraph opens the Graph view as an editor tab, or selects it.
func (s *Shell) openGraph() {
	h := s.graphHost()
	if h.item != nil && s.hasTab(h.item) {
		s.selectTab(h.item, true)
		return
	}
	if h.view == nil {
		h.view = graphview.New()
		h.view.Dispatch = s.dispatch
		h.view.OnOpen = func(id string, other bool) { s.openGraphNode(id, other) }
		h.view.SetSettings(s.graphSettings())
		h.view.OnSettings = func(gs graphview.Settings) { s.graphState = &gs }
		h.controls = graphview.NewControls(h.view)
		h.controls.Presets = s.graphPresets()
		h.controls.OnPresets = func(p map[string]graphview.Settings) { s.saveGraphPresets(p) }
		h.body = container.NewStack(placeholder(icons.Network, "Reading the project", "Following includes, imports and links…"))
		h.item = container.NewTabItemWithIcon("Graph", icons.Get(icons.Network, theme.Current().Text.Secondary), h.body)
	}
	s.selectQuietly = true
	s.editors.Append(h.item)
	s.selectQuietly = false
	s.syncEditor()
	s.selectTab(h.item, false)
	h.withGraph(func() {
		h.view.SetGraph(h.g, graphSeed)
		h.controls.Rebuild()
		h.body.Objects = []fyne.CanvasObject{graphview.Overlay(h.view, h.controls)}
		h.body.Refresh()
		if c := s.win.Canvas(); c != nil && s.editors.Selected() == h.item {
			c.Focus(h.view)
		}
	})
}

// openGraphNode opens a node's file, in the editor group after the graph's for Ctrl+click.
func (s *Shell) openGraphNode(id string, other bool) {
	p := filepath.Join(s.root, filepath.FromSlash(id))
	from := slices.Index(s.groups, s.editors)
	s.OpenFile(p)
	if !other {
		return
	}
	if it := s.editors.Selected(); it != nil && slices.Index(s.groups, s.editors) == from {
		s.moveToGroup(it, from+1)
	}
}

// Local Graph: a tab of the inspector showing the active file's neighbourhood at depth 1–3.

// buildLocalGraph makes the Local Graph tab.
func (s *Shell) buildLocalGraph() *container.TabItem {
	h := s.graphHost()
	h.local = graphview.New()
	h.local.Dispatch = s.dispatch
	h.local.OnOpen = func(id string, other bool) { s.openGraphNode(id, other) }
	ls := graphview.DefaultSettings()
	ls.LabelAt = 0 // a neighbourhood is small: name every node
	ls.Orphans, ls.Attachments = true, true
	h.local.SetSettings(ls)
	h.localDepth = kit.NewSlider("Depth", 1, 3, 1, 1, func(float64) { h.localFor = ""; h.syncLocal() })
	h.localNote = kit.NewText("Open a file to see what it links to.", kit.Body, kit.Tertiary)
	top := container.New(layout.NewCustomPaddedLayout(theme.Space1, theme.Space1, theme.Space2, theme.Space2), h.localDepth)
	h.localTab = container.NewTabItem("Graph", container.NewBorder(top, nil, nil, nil, container.NewStack(h.local, container.NewCenter(h.localNote))))
	return h.localTab
}

// syncLocal shows the active file's neighbourhood, when the Local Graph is on view.
func (h *graphHost) syncLocal() {
	if h == nil {
		return
	}
	s := h.s
	if h.local == nil || s.inspectorTabs == nil || s.inspectorTabs.Selected() != h.localTab || !s.bench.inspectorOn {
		return
	}
	p := s.activePath()
	rel, ok := relWithin(s.root, p)
	if p == "" || !ok {
		h.localNote.SetText("Open a file of the project to see what it links to.")
		h.localNote.Show()
		return
	}
	key := fmt.Sprintf("%s#%d", rel, int(h.localDepth.Value))
	if key == h.localFor {
		return
	}
	h.withGraph(func() {
		i, ok := h.g.Index(rel)
		if !ok {
			h.localNote.SetText(rel + " is not in the graph.")
			h.localNote.Show()
			return
		}
		sub, _ := h.g.Sub(h.g.Neighbours(i, int(h.localDepth.Value)))
		h.localNote.Hide()
		h.localFor = key
		h.local.SetGraph(sub, graphSeed)
		h.local.Pick(rel)
	})
}

// activePath is the file in the selected editor tab, "" for none.
func (s *Shell) activePath() string {
	return s.activeFile()
}

// Settings and presets, kept in workspace state.

// graphSettings are the Graph view's last settings, or the defaults.
func (s *Shell) graphSettings() graphview.Settings {
	if s.graphState != nil {
		return *s.graphState
	}
	return graphview.DefaultSettings()
}

// graphPresets are the saved views, from workspace state.
func (s *Shell) graphPresets() map[string]graphview.Settings {
	out := map[string]graphview.Settings{}
	for name, raw := range s.graphPresetsRaw {
		var gs graphview.Settings
		if json.Unmarshal(raw, &gs) == nil {
			out[name] = gs
		}
	}
	return out
}

// saveGraphPresets keeps the saved views for workspace state.
func (s *Shell) saveGraphPresets(p map[string]graphview.Settings) {
	s.graphPresetsRaw = map[string]json.RawMessage{}
	for name, gs := range p {
		if data, err := json.Marshal(gs); err == nil {
			s.graphPresetsRaw[name] = data
		}
	}
	s.saveState()
}

// GraphView is the Graph view; for review renders.
func (s *Shell) GraphView() (*graphview.View, *graphview.Controls) {
	h := s.graphHost()
	return h.view, h.controls
}

// ShowLocalGraph opens the inspector at the Local Graph; for review renders and the palette.
func (s *Shell) ShowLocalGraph() {
	h := s.graphHost()
	if !s.bench.inspectorOn {
		s.ToggleInspector()
	}
	s.inspectorTabs.Select(h.localTab)
	h.localFor = ""
	h.syncLocal()
}

// LocalGraph is the Local Graph's view; for review renders.
func (s *Shell) LocalGraph() *graphview.View { return s.graphHost().local }
