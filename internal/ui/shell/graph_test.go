package shell

import (
	"os"
	"path/filepath"
	"testing"

	"fyne.io/fyne/v2/test"

	"github.com/obstinix/PyxForge/internal/ui/graphview"
	"github.com/obstinix/PyxForge/internal/workspace"
)

// graphProject writes a small C project: main.c includes util.h, which includes io.h.
func graphProject(t *testing.T, root string) {
	t.Helper()
	for name, text := range map[string]string{
		"main.c":    "#include \"util.h\"\nint main(void) { return 0; }\n",
		"util.h":    "#include \"io.h\"\n",
		"io.h":      "/* io */\n",
		"README.md": "See [[notes]].\n",
		"notes.md":  "notes\n",
	} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// TestGraphView: Open Graph View builds the project's graph off the UI thread and shows it as
// an editor tab; a click on a node opens its file, Ctrl+click in another editor group.
func TestGraphView(t *testing.T) {
	s, root, q := newEditorShell(t)
	graphProject(t, root)
	if !s.Commands().Run("view.graph") {
		t.Fatal("no Open Graph View command")
	}
	h := s.graphs
	q.pumpUntil(t, "the graph", func() bool { return h.view != nil && h.view.Graph() != nil })
	h.view.StopLayout()
	if s.editors.Selected() != h.item {
		t.Error("the Graph tab is not selected")
	}
	g := h.view.Graph()
	if _, ok := g.Index("main.c"); !ok || len(g.Edges) < 3 {
		t.Fatalf("graph: %d nodes, %d edges", len(g.Nodes), len(g.Edges))
	}
	h.view.OnOpen("main.c", false)
	if s.activeFile() != filepath.Join(root, "main.c") {
		t.Errorf("a click opened %q", s.activeFile())
	}
	s.selectTab(h.item, true)
	h.view.OnOpen("util.h", true)
	if len(s.groups) != 2 || s.activeFile() != filepath.Join(root, "util.h") || s.groupOf(h.item) == s.editors {
		t.Errorf("Ctrl+click: %d groups, active %q", len(s.groups), s.activeFile())
	}
	// Opening it again selects the same tab.
	s.Commands().Run("view.graph")
	if s.editors.Selected() != h.item || len(s.allTabs()) != 3 {
		t.Errorf("reopening: %d tabs", len(s.allTabs()))
	}
}

// TestLocalGraph: the inspector's Local Graph shows the active file's neighbourhood at the
// depth chosen, and follows the active file.
func TestLocalGraph(t *testing.T) {
	s, root, q := newEditorShell(t)
	graphProject(t, root)
	s.OpenFile(filepath.Join(root, "main.c"))
	s.ShowLocalGraph()
	h := s.graphs
	q.pumpUntil(t, "the local graph", func() bool { return h.local.Graph() != nil })
	h.local.StopLayout()
	ids := func() map[string]bool {
		out := map[string]bool{}
		for _, n := range h.local.Graph().Nodes {
			out[n.ID] = true
		}
		return out
	}
	if got := ids(); !got["main.c"] || !got["util.h"] || got["io.h"] {
		t.Errorf("depth 1 around main.c: %v", got)
	}
	h.localDepth.SetValue(2)
	h.local.StopLayout()
	if got := ids(); !got["io.h"] {
		t.Errorf("depth 2 around main.c: %v", got)
	}
	s.OpenFile(filepath.Join(root, "README.md"))
	h.local.StopLayout()
	if got := ids(); !got["notes.md"] || got["main.c"] {
		t.Errorf("after opening README.md: %v", got)
	}
}

// TestGraphFollowsChanges: a file written later joins the graph with its links, through the
// explorer's watcher.
func TestGraphFollowsChanges(t *testing.T) {
	s, root, q := newEditorShell(t)
	graphProject(t, root)
	s.Commands().Run("view.graph")
	h := s.graphs
	q.pumpUntil(t, "the graph", func() bool { return h.view.Graph() != nil })
	h.view.StopLayout()
	p := filepath.Join(root, "extra.c")
	os.WriteFile(p, []byte("#include \"io.h\"\n"), 0o644)
	s.explorer.OnFilesChanged([]string{p})
	h.view.StopLayout()
	g := h.view.Graph()
	i, ok := g.Index("extra.c")
	if !ok {
		t.Fatal("extra.c did not join the graph")
	}
	linked := false
	for _, e := range g.Edges {
		if e.From == i && g.Nodes[e.To].ID == "io.h" {
			linked = true
		}
	}
	if !linked {
		t.Error("extra.c's include is not a link")
	}
}

// TestGraphPresetsPersist: saved views and the view's settings come back in the next session.
func TestGraphPresetsPersist(t *testing.T) {
	root := t.TempDir()
	graphProject(t, root)
	store := &workspace.StateStore{Dir: t.TempDir()}
	a := test.NewTempApp(t)
	q := make(queue, 1024)
	s := NewWithOptions(a, root, Options{State: store, Editor: true, Dispatch: q.post})
	t.Cleanup(func() { s.stopEditor(); s.term.stop() })
	s.Commands().Run("view.graph")
	h := s.graphs
	q.pumpUntil(t, "the graph", func() bool { return h.view.Graph() != nil })
	h.view.StopLayout()
	gs := h.view.Settings()
	gs.Query, gs.Scheme = "ext:c", graphview.SchemeSpectrum
	h.view.SetSettings(gs)
	h.controls.Save("C files")
	s.saveState()

	r := NewWithOptions(test.NewTempApp(t), root, Options{State: store, Editor: true, Dispatch: q.post})
	t.Cleanup(func() { r.stopEditor(); r.term.stop() })
	if got := r.graphPresets(); got["C files"].Query != "ext:c" {
		t.Errorf("presets after restart: %+v", got)
	}
	if got := r.graphSettings(); got.Scheme != graphview.SchemeSpectrum || got.Query != "ext:c" {
		t.Errorf("settings after restart: %+v", got)
	}
}
