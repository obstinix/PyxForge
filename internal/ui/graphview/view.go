package graphview

import (
	"image"
	"image/color"
	"math"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"

	"github.com/obstinix/PyxForge/internal/graph"
	"github.com/obstinix/PyxForge/internal/ui/theme"
)

// View is the graph: nodes and links in one raster, labels as text, the layout running on a
// goroutine of its own.
type View struct {
	widget.BaseWidget

	// OnOpen opens a node's file; other is true for Ctrl+click (another editor group).
	OnOpen func(id string, other bool)
	// Dispatch runs a function on the UI goroutine; fyne.Do when nil.
	Dispatch func(func())
	// OnSettings is told when the settings change (the controls), for presets and state.
	OnSettings func(Settings)

	mu       sync.Mutex      // guards the layout's positions between the layout goroutine and drawing
	full     *graph.Graph    // the whole graph
	expanded map[string]bool // folders opened past the node cap
	seed     uint64
	g        *graph.Graph // the graph shown
	m        graph.Metrics
	lay      *graph.Layout
	running  bool
	stop     chan struct{}

	s      Settings
	cols   Colours
	vis    []bool
	group  []int
	adj    [][]int
	byRank []int
	radius []float64 // a node's radius in graph units

	zoom       float64 // pixels per graph unit
	panX, panY float64 // the graph point at the view's centre
	fitted     bool

	hover    int // the node under the pointer, -1 none
	focus    int // the keyboard's node, -1 none
	focusAt  int // its place in byRank
	focused  bool
	dragNode int // the node being dragged, -1 the background
	dragging bool
	mods     fyne.KeyModifier

	raster *canvas.Raster
	// Synchronous draws each frame on the UI thread, for review renders that capture once.
	Synchronous bool
	version     int // counts changes to what is drawn
	drawing     bool
	frameMu     sync.Mutex
	lastImg     image.Image
	lastKey     [3]int // width, height, version
	labels      []*canvas.Text
}

// New is an empty graph view.
func New() *View {
	v := &View{s: DefaultSettings(), zoom: 1, hover: -1, focus: -1, dragNode: -1}
	v.raster = canvas.NewRaster(v.rasterFor)
	v.ExtendBaseWidget(v)
	return v
}

// SetGraph shows a graph, laid out from the start with seed, and starts its layout.
func (v *View) SetGraph(g *graph.Graph, seed uint64) {
	v.full, v.seed, v.expanded = g, seed, map[string]bool{}
	v.show()
}

// show lays out the graph as shown: the full graph, or, above the node cap, its files gathered
// into folders (graph.Collapse), with the folders the user expanded open.
func (v *View) show() {
	g, _ := v.full.Collapse(v.nodeCap(), v.expanded)
	seed := v.seed
	v.StopLayout()
	v.mu.Lock()
	v.g = g
	v.m = g.Compute()
	v.lay = graph.NewLayout(g, seed)
	v.lay.F = v.s.Forces
	v.adj = g.Adjacency()
	v.byRank = g.ByPageRank(v.m)
	v.mu.Unlock()
	v.hover, v.focus, v.fitted = -1, -1, false
	v.apply()
	v.StartLayout()
}

// Graph is the graph shown (collapsed into folders above the node cap).
func (v *View) Graph() *graph.Graph { return v.g }

// Full is the whole graph, before any folder is collapsed.
func (v *View) Full() *graph.Graph { return v.full }

// DefaultNodeCap is how many nodes the graph shows before it gathers files into folders.
const DefaultNodeCap = 2000

func (v *View) nodeCap() int {
	if v.s.NodeCap <= 0 {
		return DefaultNodeCap
	}
	return v.s.NodeCap
}

// Expand opens a collapsed folder node into its files.
func (v *View) Expand(dir string) {
	v.expanded[dir] = true
	v.show()
}

// CollapseAll gathers every expanded folder again.
func (v *View) CollapseAll() {
	if len(v.expanded) == 0 {
		return
	}
	v.expanded = map[string]bool{}
	v.show()
}

// Metrics are the graph's metrics.
func (v *View) Metrics() graph.Metrics { return v.m }

// Settings are the view's settings.
func (v *View) Settings() Settings { return v.s }

// SetSettings applies settings (the controls, a preset).
func (v *View) SetSettings(s Settings) {
	forces := s.Forces != v.s.Forces
	recap := s.NodeCap != v.s.NodeCap && v.full != nil
	v.s = s
	if recap {
		v.show()
		if v.OnSettings != nil {
			v.OnSettings(s)
		}
		return
	}
	if v.lay != nil && forces {
		v.mu.Lock()
		v.lay.F = s.Forces
		v.lay.Reheat(0.5)
		v.mu.Unlock()
		v.StartLayout()
	}
	v.apply()
	if v.OnSettings != nil {
		v.OnSettings(s)
	}
}

// apply works out visibility, colours and sizes from the settings.
func (v *View) apply() {
	v.cols = Resolve(v.s.Scheme, theme.Current())
	if v.g == nil {
		v.Refresh()
		return
	}
	v.vis = visible(v.g, v.m, v.s)
	v.group = groups(v.g, v.m, v.s)
	v.radius = make([]float64, len(v.g.Nodes))
	maxPR := 0.0
	for _, p := range v.m.PageRank {
		maxPR = math.Max(maxPR, p)
	}
	for i := range v.g.Nodes {
		r := 4.0
		switch v.s.NodeSize {
		case "degree":
			r = 3 + 2.2*math.Sqrt(float64(v.m.Degree[i]))
		case "pagerank":
			if maxPR > 0 {
				r = 3 + 13*math.Sqrt(v.m.PageRank[i]/maxPR)
			}
		}
		v.radius[i] = r * math.Max(v.s.SizeFactor, 0.2)
	}
	v.Refresh()
}

// Layout on a goroutine: ticks until it settles, publishing at most 60 times a second.

// StartLayout runs the layout until it settles.
func (v *View) StartLayout() {
	if v.lay == nil || v.running {
		return
	}
	v.running = true
	stop := make(chan struct{})
	v.stop = stop
	go func() {
		last := time.Time{}
		for {
			select {
			case <-stop:
				return
			default:
			}
			v.mu.Lock()
			v.lay.Tick()
			settled := v.lay.Settled()
			v.mu.Unlock()
			if settled || time.Since(last) >= time.Second/60 {
				last = time.Now()
				v.publish(settled)
			}
			if settled {
				return
			}
		}
	}()
}

// publish redraws on the UI goroutine, once however many ticks ran meanwhile.
func (v *View) publish(settled bool) {
	do := v.Dispatch
	if do == nil {
		do = fyne.Do
	}
	do(func() {
		if settled {
			v.running = false
		}
		v.Refresh()
	})
}

// StopLayout stops the layout goroutine.
func (v *View) StopLayout() {
	if v.stop != nil {
		close(v.stop)
		v.stop = nil
	}
	v.running = false
}

// Settle runs the layout to rest on this goroutine; for tests and review renders.
func (v *View) Settle(max int) {
	v.StopLayout()
	if v.lay == nil {
		return
	}
	v.mu.Lock()
	v.lay.Run(max)
	v.mu.Unlock()
	v.Refresh()
}

// Reform lays the graph out again from a new start (Forces > Re-form).
func (v *View) Reform(seed uint64) {
	if v.g == nil {
		return
	}
	v.StopLayout()
	v.mu.Lock()
	v.lay = graph.NewLayout(v.g, seed)
	v.lay.F = v.s.Forces
	v.mu.Unlock()
	v.fitted = false
	v.StartLayout()
}

// Geometry

// fit centres and scales the view on the visible nodes.
func (v *View) fit(size fyne.Size) {
	if v.lay == nil || size.Width <= 0 {
		return
	}
	v.mu.Lock()
	minX, minY, maxX, maxY := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	for i := range v.lay.X {
		if v.vis != nil && !v.vis[i] {
			continue
		}
		minX, maxX = math.Min(minX, v.lay.X[i]), math.Max(maxX, v.lay.X[i])
		minY, maxY = math.Min(minY, v.lay.Y[i]), math.Max(maxY, v.lay.Y[i])
	}
	v.mu.Unlock()
	if math.IsInf(minX, 0) {
		minX, minY, maxX, maxY = -1, -1, 1, 1
	}
	w, h := math.Max(maxX-minX, 60), math.Max(maxY-minY, 60)
	v.zoom = math.Min(float64(size.Width)/(w*1.15), float64(size.Height)/(h*1.15))
	// Never so close that the largest node is more than 24 px across its radius: a graph of
	// one or two nodes would otherwise fill the view.
	maxR := 1.0
	for _, r := range v.radius {
		maxR = math.Max(maxR, r)
	}
	v.zoom = math.Max(0.02, math.Min(v.zoom, math.Min(6, 24/maxR)))
	v.panX, v.panY = (minX+maxX)/2, (minY+maxY)/2
	v.fitted = true
}

// Fit fits the view to the visible nodes.
func (v *View) Fit() { v.fit(v.Size()); v.Refresh() }

// toScreen maps a graph point to the view.
func (v *View) toScreen(x, y float64) (float32, float32) {
	s := v.Size()
	return float32((x-v.panX)*v.zoom + float64(s.Width)/2), float32((y-v.panY)*v.zoom + float64(s.Height)/2)
}

// toGraph maps a point on the view to the graph.
func (v *View) toGraph(p fyne.Position) (float64, float64) {
	s := v.Size()
	return (float64(p.X)-float64(s.Width)/2)/v.zoom + v.panX, (float64(p.Y)-float64(s.Height)/2)/v.zoom + v.panY
}

// nodeAt is the visible node under a point, the nearest one inside its radius (with a few
// pixels' grace), or -1.
func (v *View) nodeAt(p fyne.Position) int {
	if v.lay == nil {
		return -1
	}
	gx, gy := v.toGraph(p)
	best, bestD := -1, math.Inf(1)
	v.mu.Lock()
	defer v.mu.Unlock()
	for i := range v.lay.X {
		if !v.vis[i] {
			continue
		}
		dx, dy := v.lay.X[i]-gx, v.lay.Y[i]-gy
		d := math.Hypot(dx, dy)
		if d <= v.radius[i]+4/v.zoom && d < bestD {
			best, bestD = i, d
		}
	}
	return best
}

// NodePos is a node's position on the view; for tests.
func (v *View) NodePos(id string) (fyne.Position, bool) {
	i, ok := v.g.Index(id)
	if !ok {
		return fyne.Position{}, false
	}
	v.mu.Lock()
	x, y := v.toScreen(v.lay.X[i], v.lay.Y[i])
	v.mu.Unlock()
	return fyne.NewPos(x, y), true
}

// Zoom is the view's scale, pixels per graph unit.
func (v *View) Zoom() float64 { return v.zoom }

// Pan is the graph point at the view's centre.
func (v *View) Pan() (float64, float64) { return v.panX, v.panY }

// Hovered is the node under the pointer, "" none.
func (v *View) Hovered() string {
	if v.hover < 0 {
		return ""
	}
	return v.g.Nodes[v.hover].ID
}

// FocusedNode is the keyboard's node, "" none.
func (v *View) FocusedNode() string {
	if v.focus < 0 {
		return ""
	}
	return v.g.Nodes[v.focus].ID
}

// Pinned reports whether a node is pinned.
func (v *View) Pinned(id string) bool {
	i, ok := v.g.Index(id)
	return ok && v.lay.Pinned[i]
}

// Drawing

// dimmed reports whether node i is dimmed: something is hovered and i is neither it nor a
// neighbour.
func (v *View) dimmed(i int) bool {
	if v.hover < 0 || i == v.hover {
		return false
	}
	for _, n := range v.adj[v.hover] {
		if n == i {
			return false
		}
	}
	return true
}

// DimAlpha is how much of a dimmed node or link shows while another is hovered.
const DimAlpha = 0.15

func fade(c color.NRGBA, a float64) color.NRGBA {
	c.A = uint8(math.Round(float64(c.A) * math.Max(0, math.Min(a, 1))))
	return c
}

// frame is what one drawing of the graph needs, copied on the UI thread, so the drawing can run
// on a worker without touching the view (owner review F5: a dense graph's frame must never
// stall the UI).
type frame struct {
	g                *graph.Graph
	m                graph.Metrics
	xs, ys           []float64
	pinned           []bool
	vis              []bool
	radius           []float64
	group            []int
	adj              [][]int
	byRank           []int
	cols             Colours
	s                Settings
	hover, focus     int
	focused          bool
	zoom, panX, panY float64
	size             fyne.Size
}

// snapshot copies the view's drawing state; call it on the UI thread.
func (v *View) snapshot() *frame {
	f := &frame{g: v.g, m: v.m, vis: v.vis, radius: v.radius, group: v.group, adj: v.adj, byRank: v.byRank,
		cols: v.cols, s: v.s, hover: v.hover, focus: v.focus, focused: v.focused, zoom: v.zoom,
		panX: v.panX, panY: v.panY, size: v.Size()}
	if v.lay != nil {
		v.mu.Lock()
		f.xs = append([]float64(nil), v.lay.X...)
		f.ys = append([]float64(nil), v.lay.Y...)
		f.pinned = append([]bool(nil), v.lay.Pinned...)
		v.mu.Unlock()
	}
	return f
}

// rasterFor is the raster's generator, called on the UI thread when Fyne paints. It returns the
// last finished frame at once; when the view has changed since, a worker draws the new frame
// from a snapshot and asks for a repaint when it is done. A dense graph's slow frame therefore
// never stalls the UI; the picture catches up a frame later.
func (v *View) rasterFor(w, h int) image.Image {
	if v.Synchronous {
		return v.draw(w, h)
	}
	want := [3]int{w, h, v.version}
	v.frameMu.Lock()
	img, key := v.lastImg, v.lastKey
	v.frameMu.Unlock()
	if img != nil && key == want {
		return img
	}
	if !v.drawing {
		v.drawing = true
		f := v.snapshot()
		do := v.Dispatch
		if do == nil {
			do = fyne.Do
		}
		go func() {
			defer func() {
				if r := recover(); r != nil { // a drawing bug must not take the app down
					do(func() { v.drawing = false })
				}
			}()
			out := f.draw(w, h)
			v.frameMu.Lock()
			v.lastImg, v.lastKey = out, want
			v.frameMu.Unlock()
			do(func() {
				v.drawing = false
				v.raster.Refresh()
			})
		}()
	}
	if img == nil { // nothing drawn yet: the background, one colour, costing nothing to make
		return image.NewUniform(v.cols.Background)
	}
	return img // the last frame, until the new one is ready
}

// draw draws the view now, on the calling goroutine; for tests.
func (v *View) draw(w, h int) image.Image { return v.snapshot().draw(w, h) }

func (f *frame) toScreen(x, y float64) (float32, float32) {
	return float32((x-f.panX)*f.zoom + float64(f.size.Width)/2), float32((y-f.panY)*f.zoom + float64(f.size.Height)/2)
}

func (f *frame) dimmed(i int) bool {
	if f.hover < 0 || i == f.hover {
		return false
	}
	for _, n := range f.adj[f.hover] {
		if n == i {
			return false
		}
	}
	return true
}

func (f *frame) nodeColour(i int) color.NRGBA {
	gi := f.group[i]
	if gi < 0 || len(f.cols.Nodes) == 0 {
		return f.cols.Edge
	}
	return f.cols.Nodes[gi%len(f.cols.Nodes)]
}

// draw paints links, halos and nodes into one image (canvas.Raster's generator), at the
// device pixel size it is given. Every shape costs only the pixels it covers (raster.go), and
// shapes off the image cost nothing (owner review F5).
func (f *frame) draw(w, h int) image.Image {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	bg := f.cols.Background
	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = bg.R, bg.G, bg.B, 0xff
	}
	if f.g == nil || f.xs == nil || w == 0 || h == 0 {
		return img
	}
	sc := float64(w) / math.Max(float64(f.size.Width), 1)
	xs, ys, pinned := f.xs, f.ys, f.pinned
	at := func(i int) (float64, float64) {
		x, y := f.toScreen(xs[i], ys[i])
		return float64(x) * sc, float64(y) * sc
	}
	// Links: those dimmed under a hover first, then the rest over them.
	lw := math.Max(f.s.LinkWidth*sc, 0.75)
	for pass := 0; pass < 2; pass++ {
		col := fade(f.cols.Edge, f.s.LinkOpacity)
		if pass == 0 {
			if f.hover < 0 {
				continue
			}
			col = fade(f.cols.Edge, f.s.LinkOpacity*DimAlpha)
		}
		for _, e := range f.g.Edges {
			if !f.vis[e.From] || !f.vis[e.To] {
				continue
			}
			lit := f.hover < 0 || e.From == f.hover || e.To == f.hover
			if (pass == 1) != lit {
				continue
			}
			x1, y1 := at(e.From)
			x2, y2 := at(e.To)
			drawLine(img, x1, y1, x2, y2, lw, col)
			if f.s.Arrows {
				r := f.radius[e.To] * f.zoom * sc
				dx, dy := x2-x1, y2-y1
				if d := math.Hypot(dx, dy); d > r+6*sc {
					ux, uy := dx/d, dy/d
					tx, ty := x2-ux*r, y2-uy*r
					s := 5 * sc
					fillTriangle(img, tx, ty, tx-ux*s*1.6-uy*s, ty-uy*s*1.6+ux*s, tx-ux*s*1.6+uy*s, ty-uy*s*1.6-ux*s, col)
				}
			}
		}
	}
	// Halos (glow, D23: data, not decoration): a soft ring that grows with a node's PageRank.
	glowCut := 1.0
	switch f.s.Glow {
	case "all":
		glowCut = 0
	case "hubs":
		if n := len(f.byRank); n > 0 {
			glowCut = f.m.PageRank[f.byRank[min(n-1, max(n/12, 3))]]
		}
	}
	// Nodes, dimmed ones first.
	order := make([]int, 0, len(f.g.Nodes))
	for i := range f.g.Nodes {
		if f.vis[i] && f.dimmed(i) {
			order = append(order, i)
		}
	}
	for i := range f.g.Nodes {
		if f.vis[i] && !f.dimmed(i) {
			order = append(order, i)
		}
	}
	for _, i := range order {
		x, y := at(i)
		r := math.Max(f.radius[i]*f.zoom, 1.5) * sc
		if x+r*3 < 0 || y+r*3 < 0 || x-r*3 > float64(w) || y-r*3 > float64(h) {
			continue // off the view
		}
		c := f.nodeColour(i)
		a := 1.0
		if f.dimmed(i) {
			a = DimAlpha
		}
		if f.s.Glow != "none" && f.m.PageRank[i] >= glowCut && !f.dimmed(i) {
			fillDisc(img, x, y, r*2.6, fade(c, 0.10))
			fillDisc(img, x, y, r*1.9, fade(c, 0.18))
		}
		f.shape(img, i, x, y, r, fade(c, a))
		if i == f.focus && f.focused {
			strokeRing(img, x, y, r+4*sc, 2*sc, f.cols.Halo)
		}
		if pinned[i] {
			fillDisc(img, x, y, math.Max(r*0.35, 1.2*sc), f.cols.Background)
		}
	}
	return img
}

// shape fills a node: a disc for a file, a rounded square for a note (D25).
func (f *frame) shape(img *image.NRGBA, i int, x, y, r float64, c color.NRGBA) {
	switch f.g.Nodes[i].Kind {
	case graph.Note:
		s := r * 0.9 // the square's half side: about the disc's area
		fillRoundSquare(img, x, y, s, s*0.4, c)
		return
	case graph.Folder: // a ring around a smaller disc: several files in one
		strokeRing(img, x, y, r, math.Max(r*0.22, 1.5), c)
		fillDisc(img, x, y, r*0.55, c)
		return
	}
	fillDisc(img, x, y, r, c)
}

// nodeColour is a node's group colour.
func (v *View) nodeColour(i int) color.NRGBA {
	gi := v.group[i]
	if gi < 0 || len(v.cols.Nodes) == 0 {
		return v.cols.Edge
	}
	return v.cols.Nodes[gi%len(v.cols.Nodes)]
}

// NodeColour is a node's colour; for tests.
func (v *View) NodeColour(id string) color.NRGBA {
	i, _ := v.g.Index(id)
	return v.nodeColour(i)
}

// Shown reports whether a node shows under the filters.
func (v *View) Shown(id string) bool {
	i, ok := v.g.Index(id)
	return ok && v.vis[i]
}

// Labels are the label texts drawn; for tests.
func (v *View) Labels() []string {
	var out []string
	for _, l := range v.labels {
		if l.Visible() {
			out = append(out, l.Text)
		}
	}
	return out
}

// CreateRenderer implements fyne.Widget.
func (v *View) CreateRenderer() fyne.WidgetRenderer { return &renderer{v: v} }

type renderer struct{ v *View }

func (r *renderer) Layout(s fyne.Size) {
	r.v.raster.Resize(s)
	if !r.v.fitted {
		r.v.fit(s)
	}
	r.Refresh()
}

func (r *renderer) MinSize() fyne.Size { return fyne.NewSize(120, 120) }

func (r *renderer) Refresh() {
	v := r.v
	v.version++ // what is drawn may have changed
	v.cols = Resolve(v.s.Scheme, theme.Current())
	if !v.fitted && v.Size().Width > 0 {
		v.fit(v.Size())
	}
	v.raster.Refresh()
	r.labels()
}

// labels shows names for nodes big enough on screen, and for the hovered and focused nodes
// and the hovered one's neighbours.
func (r *renderer) labels() {
	v := r.v
	for _, l := range v.labels {
		l.Hide()
	}
	if v.g == nil || v.lay == nil {
		return
	}
	want := map[int]bool{}
	for _, i := range v.byRank {
		if v.vis[i] && v.radius[i]*v.zoom >= v.s.LabelAt && !v.dimmed(i) {
			want[i] = true
		}
		if len(want) >= 80 { // no more than the screen can read
			break
		}
	}
	if v.hover >= 0 {
		want[v.hover] = true
		for _, n := range v.adj[v.hover] {
			if v.vis[n] {
				want[n] = true
			}
		}
	}
	if v.focus >= 0 && v.focused {
		want[v.focus] = true
	}
	k := 0
	v.mu.Lock()
	defer v.mu.Unlock()
	for _, i := range v.byRank {
		if !want[i] {
			continue
		}
		if k == len(v.labels) {
			t := canvas.NewText("", v.cols.Label)
			t.TextSize = theme.TextSmall
			v.labels = append(v.labels, t)
		}
		t := v.labels[k]
		k++
		t.Text = v.g.Nodes[i].Name()
		t.Color = v.cols.Label
		if i == v.hover || i == v.focus && v.focused {
			t.TextStyle.Bold = true
		} else {
			t.TextStyle.Bold = false
		}
		x, y := v.toScreen(v.lay.X[i], v.lay.Y[i])
		ms := t.MinSize()
		t.Move(fyne.NewPos(x-ms.Width/2, y+float32(v.radius[i]*v.zoom)+2))
		t.Resize(ms)
		t.Show()
		t.Refresh()
	}
}

func (r *renderer) Objects() []fyne.CanvasObject {
	objs := []fyne.CanvasObject{r.v.raster}
	for _, l := range r.v.labels {
		objs = append(objs, l)
	}
	return objs
}

func (r *renderer) Destroy() { r.v.StopLayout() }

// Interaction

// Scrolled zooms around the pointer.
func (v *View) Scrolled(e *fyne.ScrollEvent) {
	f := math.Pow(1.0015, float64(e.Scrolled.DY))
	v.zoomAround(e.Position, f)
}

func (v *View) zoomAround(p fyne.Position, f float64) {
	gx, gy := v.toGraph(p)
	v.zoom = math.Max(0.02, math.Min(v.zoom*f, 12))
	// Keep the graph point under the pointer where it was.
	nx, ny := v.toGraph(p)
	v.panX += gx - nx
	v.panY += gy - ny
	v.Refresh()
}

// MouseDown remembers the modifiers (Ctrl+click opens in another group) and what a drag
// would move.
func (v *View) MouseDown(e *desktop.MouseEvent) {
	v.mods = e.Modifier
	v.dragNode = v.nodeAt(e.Position)
	if c := fyne.CurrentApp().Driver().CanvasForObject(v); c != nil {
		c.Focus(v)
	}
}

func (v *View) MouseUp(*desktop.MouseEvent) {}

// Dragged pans on the background and moves (and pins) a node.
func (v *View) Dragged(e *fyne.DragEvent) {
	v.dragging = true
	if v.dragNode >= 0 {
		gx, gy := v.toGraph(e.Position)
		v.mu.Lock()
		v.lay.Move(v.dragNode, gx, gy)
		v.lay.Pin(v.dragNode, true)
		v.lay.Reheat(0.3)
		v.mu.Unlock()
		v.StartLayout()
		v.Refresh()
		return
	}
	v.panX -= float64(e.Dragged.DX) / v.zoom
	v.panY -= float64(e.Dragged.DY) / v.zoom
	v.Refresh()
}

func (v *View) DragEnd() { v.dragging = false }

// Tapped opens the node clicked, Ctrl+click in another editor group.
func (v *View) Tapped(e *fyne.PointEvent) {
	if c := fyne.CurrentApp().Driver().CanvasForObject(v); c != nil {
		c.Focus(v)
	}
	i := v.nodeAt(e.Position)
	if i >= 0 && v.g.Nodes[i].Kind == graph.Folder { // a gathered folder opens into its files
		v.Expand(v.g.Nodes[i].Dir)
		return
	}
	if i < 0 || v.OnOpen == nil {
		return
	}
	v.OnOpen(v.g.Nodes[i].ID, v.mods&(fyne.KeyModifierControl|fyne.KeyModifierSuper) != 0)
	v.mods = 0
}

// DoubleTapped unpins a node.
func (v *View) DoubleTapped(e *fyne.PointEvent) {
	if i := v.nodeAt(e.Position); i >= 0 {
		v.mu.Lock()
		v.lay.Pin(i, false)
		v.lay.Reheat(0.3)
		v.mu.Unlock()
		v.StartLayout()
		v.Refresh()
	}
}

func (v *View) MouseIn(e *desktop.MouseEvent)    { v.setHover(v.nodeAt(e.Position)) }
func (v *View) MouseMoved(e *desktop.MouseEvent) { v.setHover(v.nodeAt(e.Position)) }
func (v *View) MouseOut()                        { v.setHover(-1) }

func (v *View) setHover(i int) {
	if i != v.hover {
		v.hover = i
		v.Refresh()
	}
}

// HoverAt moves the pointer over a point; for tests and review renders.
func (v *View) HoverAt(p fyne.Position) { v.setHover(v.nodeAt(p)) }

func (v *View) Cursor() desktop.Cursor {
	if v.hover >= 0 {
		return desktop.PointerCursor
	}
	return desktop.DefaultCursor
}

// Keyboard: arrows pan, + and - zoom, Tab cycles nodes in PageRank order, Enter opens.

func (v *View) FocusGained() { v.focused = true; v.Refresh() }
func (v *View) FocusLost()   { v.focused = false; v.Refresh() }
func (v *View) AcceptsTab() bool {
	return true
}

func (v *View) TypedRune(r rune) {
	switch r {
	case '+', '=':
		v.zoomAround(fyne.NewPos(v.Size().Width/2, v.Size().Height/2), 1.25)
	case '-', '_':
		v.zoomAround(fyne.NewPos(v.Size().Width/2, v.Size().Height/2), 0.8)
	case '0':
		v.Fit()
	}
}

// PanStep is how far an arrow key pans, in pixels.
const PanStep = 60

func (v *View) TypedKey(e *fyne.KeyEvent) {
	switch e.Name {
	case fyne.KeyLeft:
		v.panX -= PanStep / v.zoom
	case fyne.KeyRight:
		v.panX += PanStep / v.zoom
	case fyne.KeyUp:
		v.panY -= PanStep / v.zoom
	case fyne.KeyDown:
		v.panY += PanStep / v.zoom
	case fyne.KeyTab:
		v.cycle(1)
		return
	case fyne.KeyReturn, fyne.KeyEnter:
		switch {
		case v.focus >= 0 && v.g.Nodes[v.focus].Kind == graph.Folder:
			v.Expand(v.g.Nodes[v.focus].Dir)
		case v.focus >= 0 && v.OnOpen != nil:
			v.OnOpen(v.g.Nodes[v.focus].ID, false)
		}
		return
	case fyne.KeyEscape:
		v.focus = -1
	default:
		return
	}
	v.Refresh()
}

// cycle moves the keyboard to the next visible node in PageRank order and centres on it.
func (v *View) cycle(step int) {
	if v.g == nil || len(v.byRank) == 0 {
		return
	}
	n := len(v.byRank)
	for k := 1; k <= n; k++ {
		at := ((v.focusAt+step*k)%n + n) % n
		if v.focus < 0 {
			at = ((step*(k-1))%n + n) % n
		}
		if i := v.byRank[at]; v.vis[i] {
			v.focus, v.focusAt = i, at
			v.mu.Lock()
			v.panX, v.panY = v.lay.X[i], v.lay.Y[i]
			v.mu.Unlock()
			v.Refresh()
			return
		}
	}
}

// TypedShortcut takes Shift+Tab (the previous node).
func (v *View) TypedShortcut(s fyne.Shortcut) {
	if cs, ok := s.(*desktop.CustomShortcut); ok && cs.KeyName == fyne.KeyTab && cs.Modifier == fyne.KeyModifierShift {
		v.cycle(-1)
	}
}

// Pick shows the neighbourhood of a node (Local Graph) or centres on it.
func (v *View) Pick(id string) {
	if v.g == nil {
		return
	}
	if i, ok := v.g.Index(id); ok {
		v.focus = i
		for k, j := range v.byRank {
			if j == i {
				v.focusAt = k
			}
		}
		v.Refresh()
	}
}
