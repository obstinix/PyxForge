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
	"github.com/srwiley/rasterx"
	"golang.org/x/image/math/fixed"

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

	mu      sync.Mutex // guards the layout's positions between the layout goroutine and drawing
	g       *graph.Graph
	m       graph.Metrics
	lay     *graph.Layout
	running bool
	stop    chan struct{}

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
	labels []*canvas.Text
}

// New is an empty graph view.
func New() *View {
	v := &View{s: DefaultSettings(), zoom: 1, hover: -1, focus: -1, dragNode: -1}
	v.raster = canvas.NewRaster(v.draw)
	v.ExtendBaseWidget(v)
	return v
}

// SetGraph shows a graph, laid out from the start with seed, and starts its layout.
func (v *View) SetGraph(g *graph.Graph, seed uint64) {
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

// Graph is the graph shown.
func (v *View) Graph() *graph.Graph { return v.g }

// Metrics are the graph's metrics.
func (v *View) Metrics() graph.Metrics { return v.m }

// Settings are the view's settings.
func (v *View) Settings() Settings { return v.s }

// SetSettings applies settings (the controls, a preset).
func (v *View) SetSettings(s Settings) {
	forces := s.Forces != v.s.Forces
	v.s = s
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

func pt(x, y float64) fixed.Point26_6 {
	return fixed.Point26_6{X: fixed.Int26_6(x * 64), Y: fixed.Int26_6(y * 64)}
}

// draw paints links, halos and nodes into one image (canvas.Raster's generator), at the
// device pixel size it is given.
func (v *View) draw(w, h int) image.Image {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	bg := v.cols.Background
	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = bg.R, bg.G, bg.B, 0xff
	}
	if v.g == nil || v.lay == nil || w == 0 || h == 0 {
		return img
	}
	size := v.Size()
	sc := float64(w) / math.Max(float64(size.Width), 1)
	scanner := rasterx.NewScannerGV(w, h, img, img.Bounds())
	filler := rasterx.NewFiller(w, h, scanner)
	dasher := rasterx.NewDasher(w, h, scanner)
	v.mu.Lock()
	xs := append([]float64(nil), v.lay.X...)
	ys := append([]float64(nil), v.lay.Y...)
	v.mu.Unlock()
	at := func(i int) (float64, float64) {
		x, y := v.toScreen(xs[i], ys[i])
		return float64(x) * sc, float64(y) * sc
	}
	// Links, in two passes: those dimmed under a hover first, then the rest over them.
	lw := math.Max(v.s.LinkWidth*sc, 0.5)
	for pass := 0; pass < 2; pass++ {
		col := fade(v.cols.Edge, v.s.LinkOpacity)
		if pass == 0 && v.hover >= 0 {
			col = fade(v.cols.Edge, v.s.LinkOpacity*DimAlpha)
		}
		dasher.Clear()
		dasher.SetColor(col)
		dasher.SetStroke(fixed.Int26_6(lw*64), 0, nil, nil, nil, rasterx.Bevel, nil, 0)
		any := false
		for _, e := range v.g.Edges {
			if !v.vis[e.From] || !v.vis[e.To] {
				continue
			}
			lit := v.hover < 0 || e.From == v.hover || e.To == v.hover
			if (pass == 0) == lit && v.hover >= 0 {
				continue
			}
			if pass == 0 && v.hover < 0 {
				continue
			}
			x1, y1 := at(e.From)
			x2, y2 := at(e.To)
			dasher.Start(pt(x1, y1))
			dasher.Line(pt(x2, y2))
			dasher.Stop(false)
			any = true
			if v.s.Arrows {
				r := v.radius[e.To] * v.zoom * sc
				dx, dy := x2-x1, y2-y1
				d := math.Hypot(dx, dy)
				if d > r+6*sc {
					ux, uy := dx/d, dy/d
					tx, ty := x2-ux*r, y2-uy*r
					s := 5 * sc
					filler.Clear()
					filler.SetColor(col)
					filler.Start(pt(tx, ty))
					filler.Line(pt(tx-ux*s*1.6-uy*s, ty-uy*s*1.6+ux*s))
					filler.Line(pt(tx-ux*s*1.6+uy*s, ty-uy*s*1.6-ux*s))
					filler.Stop(true)
					filler.Draw()
				}
			}
		}
		if any {
			dasher.Draw()
		}
	}
	// Halos (glow, D23: data, not decoration): a soft ring that grows with a node's PageRank.
	glowCut := 1.0
	switch v.s.Glow {
	case "all":
		glowCut = 0
	case "hubs":
		if n := len(v.byRank); n > 0 {
			glowCut = v.m.PageRank[v.byRank[min(n-1, max(n/12, 3))]]
		}
	}
	// Nodes, dimmed ones first.
	order := make([]int, 0, len(v.g.Nodes))
	for i := range v.g.Nodes {
		if v.vis[i] && v.dimmed(i) {
			order = append(order, i)
		}
	}
	for i := range v.g.Nodes {
		if v.vis[i] && !v.dimmed(i) {
			order = append(order, i)
		}
	}
	for _, i := range order {
		x, y := at(i)
		r := math.Max(v.radius[i]*v.zoom, 1.5) * sc
		if x+r*3 < 0 || y+r*3 < 0 || x-r*3 > float64(w) || y-r*3 > float64(h) {
			continue
		}
		c := v.nodeColour(i)
		a := 1.0
		if v.dimmed(i) {
			a = DimAlpha
		}
		if v.s.Glow != "none" && v.m.PageRank[i] >= glowCut && !v.dimmed(i) {
			for k, f := range []float64{2.6, 1.9} {
				filler.Clear()
				filler.SetColor(fade(c, a*[]float64{0.10, 0.18}[k]))
				rasterx.AddCircle(x, y, r*f, filler)
				filler.Draw()
			}
		}
		filler.Clear()
		filler.SetColor(fade(c, a))
		v.shape(i, x, y, r, filler)
		filler.Draw()
		if i == v.focus && v.focused {
			dasher.Clear()
			dasher.SetColor(v.cols.Halo)
			dasher.SetStroke(fixed.Int26_6(2*sc*64), 0, nil, nil, nil, rasterx.Bevel, nil, 0)
			rasterx.AddCircle(x, y, r+4*sc, dasher)
			dasher.Draw()
		}
		if v.lay.Pinned[i] {
			filler.Clear()
			filler.SetColor(v.cols.Background)
			rasterx.AddCircle(x, y, math.Max(r*0.35, 1.2*sc), filler)
			filler.Draw()
		}
	}
	return img
}

// shape adds a node's outline: a disc for a file, a rounded square for a note (D25).
func (v *View) shape(i int, x, y, r float64, p rasterx.Adder) {
	if v.g.Nodes[i].Kind == graph.Note {
		s := r * 0.9 // the square's half side: about the disc's area
		rasterx.AddRoundRect(x-s, y-s, x+s, y+s, s*0.4, s*0.4, 0, rasterx.RoundGap, p)
		return
	}
	rasterx.AddCircle(x, y, r, p)
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
		if v.focus >= 0 && v.OnOpen != nil {
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
