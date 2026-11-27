package graphview

import (
	"encoding/json"
	"math"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/test"

	"github.com/obstinix/PyxForge/internal/graph"
	"github.com/obstinix/PyxForge/internal/ui/theme"
)

// small is a hub with spokes, a pair, a note with a tag and an orphan image.
func small() *graph.Graph {
	g := &graph.Graph{Root: "/p"}
	ids := []string{"src/hub.c", "src/a.c", "src/b.c", "src/c.c", "lib/x.go", "lib/y.go", "docs/plan.md", "art/logo.png"}
	for _, id := range ids {
		n := graph.Node{ID: id}
		switch {
		case id == "docs/plan.md":
			n.Kind, n.Ext, n.Tags = graph.Note, "md", []string{"todo"}
		case id == "art/logo.png":
			n.Ext = "png"
		default:
			n.Ext = id[len(id)-1:]
			if id[len(id)-2:] == "go" {
				n.Ext = "go"
			}
		}
		n.Dir = id[:len(id)-len(n.Name())-1]
		g.Nodes = append(g.Nodes, n)
	}
	g = rebuildIndex(g)
	for _, e := range [][2]int{{1, 0}, {2, 0}, {3, 0}, {4, 5}, {6, 0}} {
		g.Edges = append(g.Edges, graph.Edge{From: e[0], To: e[1], Kind: "include"})
	}
	return g
}

// rebuildIndex gives a hand-made graph its ID index (Sub keeps one).
func rebuildIndex(g *graph.Graph) *graph.Graph {
	keep := map[int]int{}
	for i := range g.Nodes {
		keep[i] = 0
	}
	s, _ := g.Sub(keep)
	return s
}

func setupView(t *testing.T) (*View, fyne.Window) {
	t.Helper()
	a := test.NewTempApp(t)
	a.Settings().SetTheme(theme.NewFyne(theme.Default))
	v := New()
	// UI work the layout goroutine posts waits in a queue, as fyne.Do would have it wait for
	// the UI thread; running it on the posting goroutine would race the test's own calls.
	q := make(chan func(), 1024)
	v.Dispatch = func(f func()) {
		select {
		case q <- f:
		default:
		}
	}
	t.Cleanup(v.StopLayout)
	w := test.NewTempWindow(t, v)
	w.Resize(fyne.NewSize(800, 600))
	v.SetGraph(small(), 1)
	v.Settle(3000)
	v.Fit()
	return v, w
}

// TestSchemesContrast: every scheme, in every theme and accent, gives nodes 3:1 against the
// graph's background and labels 4.5:1; no colour is a fixed neon.
func TestSchemesContrast(t *testing.T) {
	for _, p := range theme.Palettes {
		for _, a := range theme.Accents {
			tk := theme.Resolve(p, a)
			for s := range SchemeNames {
				c := Resolve(Scheme(s), tk)
				for i, n := range c.Nodes {
					if r := theme.Contrast(n, c.Background); r < MinNodeContrast {
						t.Errorf("%s %s %s: node colour %d at %.2f:1", p.ID, a.ID, SchemeNames[s], i, r)
					}
				}
				if r := theme.Contrast(c.Label, c.Background); r < 4.5 {
					t.Errorf("%s %s %s: labels at %.2f:1", p.ID, a.ID, SchemeNames[s], r)
				}
			}
			if Resolve(SchemeGalaxy, tk).Background == tk.Surface.Base {
				t.Errorf("%s: Galaxy has no deep background", p.ID)
			}
		}
	}
}

// TestFilters: the query's path:, ext:, tag: and - words, and the switches.
func TestFilters(t *testing.T) {
	g := small()
	m := g.Compute()
	shown := func(s Settings) []string {
		var out []string
		for i, ok := range visible(g, m, s) {
			if ok {
				out = append(out, g.Nodes[i].ID)
			}
		}
		return out
	}
	s := DefaultSettings()
	if got := shown(s); len(got) != 7 { // the image is an attachment
		t.Errorf("default: %v", got)
	}
	s.Query = "path:src -hub"
	if got := shown(s); len(got) != 3 {
		t.Errorf("path:src -hub: %v", got)
	}
	s.Query = "ext:go"
	if got := shown(s); len(got) != 2 {
		t.Errorf("ext:go: %v", got)
	}
	s.Query = "tag:todo"
	if got := shown(s); len(got) != 1 || got[0] != "docs/plan.md" {
		t.Errorf("tag:todo: %v", got)
	}
	s = DefaultSettings()
	s.NotesOnly = true
	if got := shown(s); len(got) != 1 {
		t.Errorf("notes only: %v", got)
	}
	s = DefaultSettings()
	s.Attachments, s.Orphans = true, false
	if got := shown(s); len(got) != 7 || contains(got, "art/logo.png") {
		t.Errorf("attachments without orphans: %v", got)
	}
}

func contains(s []string, x string) bool {
	for _, v := range s {
		if v == x {
			return true
		}
	}
	return false
}

// TestGroups: colour by folder, extension, cluster and query.
func TestGroups(t *testing.T) {
	g := small()
	m := g.Compute()
	s := DefaultSettings()
	gr := groups(g, m, s)
	if gr[0] != gr[1] || gr[0] == gr[4] {
		t.Errorf("by folder: %v", gr)
	}
	s.GroupBy = "query"
	s.Groups = []Group{{Query: "ext:go", Colour: 3}}
	gr = groups(g, m, s)
	if gr[4] != 3 || gr[0] != -1 {
		t.Errorf("by query: %v", gr)
	}
}

// TestZoomAndPan: the wheel zooms around the pointer, keeping the graph point under it; a drag
// on the background pans; arrows pan and + and - zoom.
func TestZoomAndPan(t *testing.T) {
	v, _ := setupView(t)
	p := fyne.NewPos(200, 150)
	gx, gy := v.toGraph(p)
	z := v.Zoom()
	v.Scrolled(&fyne.ScrollEvent{PointEvent: fyne.PointEvent{Position: p}, Scrolled: fyne.Delta{DY: 200}})
	if v.Zoom() <= z {
		t.Fatalf("the wheel did not zoom in: %f -> %f", z, v.Zoom())
	}
	if nx, ny := v.toGraph(p); math.Abs(nx-gx) > 1e-6 || math.Abs(ny-gy) > 1e-6 {
		t.Errorf("the point under the pointer moved: %f,%f -> %f,%f", gx, gy, nx, ny)
	}
	empty := fyne.NewPos(5, 5)
	v.MouseDown(&desktop.MouseEvent{PointEvent: fyne.PointEvent{Position: empty}})
	px, _ := v.Pan()
	v.Dragged(&fyne.DragEvent{PointEvent: fyne.PointEvent{Position: empty}, Dragged: fyne.Delta{DX: 40, DY: 0}})
	if nx, _ := v.Pan(); math.Abs(nx-(px-40/v.Zoom())) > 1e-9 {
		t.Errorf("a drag on the background panned to %f, want %f", nx, px-40/v.Zoom())
	}
	v.DragEnd()
	_, py := v.Pan()
	v.TypedKey(&fyne.KeyEvent{Name: fyne.KeyDown})
	if _, ny := v.Pan(); ny <= py {
		t.Error("Down did not pan")
	}
	z = v.Zoom()
	v.TypedRune('-')
	if v.Zoom() >= z {
		t.Error("- did not zoom out")
	}
}

// TestHoverDims: hovering a node dims everything but it and its neighbours; leaving clears it.
func TestHoverDims(t *testing.T) {
	v, _ := setupView(t)
	at, _ := v.NodePos("src/hub.c")
	v.HoverAt(at)
	if v.Hovered() != "src/hub.c" {
		t.Fatalf("hovered %q", v.Hovered())
	}
	hub, _ := v.g.Index("src/hub.c")
	a, _ := v.g.Index("src/a.c")
	x, _ := v.g.Index("lib/x.go")
	if v.dimmed(hub) || v.dimmed(a) || !v.dimmed(x) {
		t.Errorf("dimmed: hub %v, neighbour %v, other %v", v.dimmed(hub), v.dimmed(a), v.dimmed(x))
	}
	if !contains(v.Labels(), "a.c") || !contains(v.Labels(), "hub.c") {
		t.Errorf("labels %v", v.Labels())
	}
	v.MouseOut()
	if v.dimmed(x) {
		t.Error("leaving the node kept the dimming")
	}
}

// TestOpenAndPin: a click opens a node's file, Ctrl+click in another group; dragging a node
// pins it, a double click unpins it.
func TestOpenAndPin(t *testing.T) {
	v, _ := setupView(t)
	var opened []string
	v.OnOpen = func(id string, other bool) {
		if other {
			id += " (other group)"
		}
		opened = append(opened, id)
	}
	at, _ := v.NodePos("lib/x.go")
	v.MouseDown(&desktop.MouseEvent{PointEvent: fyne.PointEvent{Position: at}})
	v.Tapped(&fyne.PointEvent{Position: at})
	v.MouseDown(&desktop.MouseEvent{PointEvent: fyne.PointEvent{Position: at}, Modifier: fyne.KeyModifierControl})
	v.Tapped(&fyne.PointEvent{Position: at})
	if len(opened) != 2 || opened[0] != "lib/x.go" || opened[1] != "lib/x.go (other group)" {
		t.Errorf("opened %v", opened)
	}
	v.MouseDown(&desktop.MouseEvent{PointEvent: fyne.PointEvent{Position: at}})
	v.Dragged(&fyne.DragEvent{PointEvent: fyne.PointEvent{Position: at.AddXY(30, 0)}, Dragged: fyne.Delta{DX: 30}})
	v.DragEnd()
	v.StopLayout()
	if !v.Pinned("lib/x.go") {
		t.Fatal("dragging did not pin the node")
	}
	moved, _ := v.NodePos("lib/x.go")
	v.DoubleTapped(&fyne.PointEvent{Position: moved})
	v.StopLayout()
	if v.Pinned("lib/x.go") {
		t.Error("a double click did not unpin it")
	}
}

// TestKeyboardNodes: Tab goes through nodes in PageRank order, the hub first; Enter opens.
func TestKeyboardNodes(t *testing.T) {
	v, _ := setupView(t)
	var opened string
	v.OnOpen = func(id string, other bool) { opened = id }
	v.FocusGained()
	v.TypedKey(&fyne.KeyEvent{Name: fyne.KeyTab})
	if v.FocusedNode() != "src/hub.c" {
		t.Fatalf("first Tab: %q", v.FocusedNode())
	}
	v.TypedKey(&fyne.KeyEvent{Name: fyne.KeyTab})
	second := v.FocusedNode()
	if second == "src/hub.c" || second == "" {
		t.Errorf("second Tab: %q", second)
	}
	v.TypedShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyTab, Modifier: fyne.KeyModifierShift})
	if v.FocusedNode() != "src/hub.c" {
		t.Errorf("Shift+Tab: %q", v.FocusedNode())
	}
	v.TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})
	if opened != "src/hub.c" {
		t.Errorf("Enter opened %q", opened)
	}
}

// TestPresets: Save stores the settings by name, Choose restores them, Delete removes them, and
// the stored form survives JSON (workspace state).
func TestPresets(t *testing.T) {
	v, _ := setupView(t)
	c := NewControls(v)
	var stored map[string]Settings
	c.OnPresets = func(p map[string]Settings) { stored = p }
	s := v.Settings()
	s.Query, s.Scheme, s.NodeSize = "ext:c", SchemeGalaxy, "degree"
	v.SetSettings(s)
	c.Save("C only")
	if _, ok := stored["C only"]; !ok {
		t.Fatal("the preset was not handed on to be stored")
	}
	data, _ := json.Marshal(stored)
	var back map[string]Settings
	if err := json.Unmarshal(data, &back); err != nil || back["C only"].Query != "ext:c" || back["C only"].Scheme != SchemeGalaxy {
		t.Fatalf("round trip: %v %+v", err, back)
	}
	v.SetSettings(DefaultSettings())
	c.Presets = back
	c.Choose("C only")
	if got := v.Settings(); got.Query != "ext:c" || got.NodeSize != "degree" || v.Shown("lib/x.go") {
		t.Errorf("after choosing: %+v", got)
	}
	c.Delete("C only")
	if _, ok := stored["C only"]; ok {
		t.Error("Delete kept the preset")
	}
	if !c.Open() {
		t.Error("the panel starts collapsed")
	}
	c.Toggle()
	if c.Open() {
		t.Error("Toggle did not collapse the panel")
	}
}

// TestDraws: the raster paints the background and nodes in the scheme's colours.
func TestDraws(t *testing.T) {
	v, _ := setupView(t)
	img := v.draw(800, 600)
	at, _ := v.NodePos("src/hub.c")
	r, g, b, _ := img.At(int(at.X), int(at.Y)).RGBA()
	want := v.NodeColour("src/hub.c")
	if uint8(r>>8) != want.R || uint8(g>>8) != want.G || uint8(b>>8) != want.B {
		t.Errorf("the hub's centre is %d,%d,%d, want %v", r>>8, g>>8, b>>8, want)
	}
	bg := v.cols.Background
	r, g, b, _ = img.At(1, 1).RGBA()
	if uint8(r>>8) != bg.R || uint8(g>>8) != bg.G || uint8(b>>8) != bg.B {
		t.Error("the corner is not the background")
	}
}

// TestNoteShape: a note is drawn as a rounded square, a file as a disc: near the corner of
// its box is the node's colour for a note and background for a file.
func TestNoteShape(t *testing.T) {
	v, _ := setupView(t)
	two := rebuildIndex(&graph.Graph{Nodes: []graph.Node{{ID: "a.c", Ext: "c"}, {ID: "n.md", Ext: "md", Kind: graph.Note}}})
	v.SetGraph(two, 1)
	gs := v.Settings()
	gs.Glow, gs.NodeSize, gs.SizeFactor = "none", "uniform", 3
	v.SetSettings(gs)
	v.Settle(500)
	v.Fit()
	sz := v.Size() // one image pixel per view unit, so positions match
	img := v.draw(int(sz.Width), int(sz.Height))
	corner := func(id string) bool {
		at, _ := v.NodePos(id)
		i, _ := v.g.Index(id)
		r := v.radius[i] * v.Zoom()
		x, y := int(float64(at.X)+r*0.75), int(float64(at.Y)+r*0.75)
		c := v.NodeColour(id)
		cr, cg, cb, _ := img.At(x, y).RGBA()
		return uint8(cr>>8) == c.R && uint8(cg>>8) == c.G && uint8(cb>>8) == c.B
	}
	if !corner("n.md") {
		t.Error("the note is not a square")
	}
	if corner("a.c") {
		t.Error("the file is not a disc")
	}
}
