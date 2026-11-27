package graph

import (
	"hash/fnv"
	"math"
)

// Forces are the layout's parameters (Graph view > Forces).
type Forces struct {
	Repulsion    float64 // how hard nodes push apart
	LinkLength   float64 // a link's rest length
	LinkStrength float64 // how hard a link pulls toward its length, 0…1
	Centre       float64 // pull toward the middle, 0…1
	Damping      float64 // velocity kept each tick, 0…1
}

// DefaultForces are the forces a new view starts with.
var DefaultForces = Forces{Repulsion: 900, LinkLength: 70, LinkStrength: 0.12, Centre: 0.012, Damping: 0.82}

// BarnesHutAbove is the node count above which repulsion is approximated with a quadtree.
const BarnesHutAbove = 500

// Layout places nodes with forces: every pair repels (Barnes–Hut above BarnesHutAbove
// nodes), links pull toward their length, everything is pulled to the middle, and velocity
// is damped. It is deterministic: the same nodes, edges, forces and seed give the same
// positions tick for tick. It is not safe for concurrent use.
type Layout struct {
	X, Y   []float64
	vx, vy []float64
	Pinned []bool
	edges  [][2]int
	F      Forces
	alpha  float64 // the temperature: forces are scaled by it, and it cools each tick
	theta  float64
	tree   quadtree
}

// NewLayout starts a layout of g's nodes on a deterministic spiral, its order shuffled by seed.
func NewLayout(g *Graph, seed uint64) *Layout {
	n := len(g.Nodes)
	l := &Layout{X: make([]float64, n), Y: make([]float64, n), vx: make([]float64, n), vy: make([]float64, n),
		Pinned: make([]bool, n), F: DefaultForces, alpha: 1, theta: 0.9}
	for _, e := range g.Edges {
		l.edges = append(l.edges, [2]int{e.From, e.To})
	}
	golden := math.Pi * (3 - math.Sqrt(5))
	for i, nd := range g.Nodes {
		h := fnv.New64a()
		h.Write([]byte(nd.ID))
		k := float64((h.Sum64()^seed)%uint64(max(n, 1)*4)) + float64(i)*0.25
		r := 12 * math.Sqrt(k+1)
		l.X[i], l.Y[i] = r*math.Cos(k*golden), r*math.Sin(k*golden)
	}
	return l
}

// Settled reports whether the layout has cooled down: further ticks barely move it.
func (l *Layout) Settled() bool { return l.alpha < 0.004 }

// Reheat restarts the cooling, after the forces changed or a node was dragged (Re-form).
func (l *Layout) Reheat(a float64) { l.alpha = max(l.alpha, a) }

// Tick moves every node one step.
func (l *Layout) Tick() {
	n := len(l.X)
	if n == 0 {
		return
	}
	fx, fy := make([]float64, n), make([]float64, n)
	// Repulsion.
	k := l.F.Repulsion * l.alpha
	if n > BarnesHutAbove {
		l.tree.build(l.X, l.Y)
		for i := range n {
			ax, ay := l.tree.force(i, l.X[i], l.Y[i], l.theta)
			fx[i] += ax * k
			fy[i] += ay * k
		}
	} else {
		for i := range n {
			for j := i + 1; j < n; j++ {
				dx, dy := l.X[i]-l.X[j], l.Y[i]-l.Y[j]
				d2 := dx*dx + dy*dy + 0.01
				f := k / d2
				fx[i] += dx * f
				fy[i] += dy * f
				fx[j] -= dx * f
				fy[j] -= dy * f
			}
		}
	}
	// Links pull toward their length.
	for _, e := range l.edges {
		a, b := e[0], e[1]
		dx, dy := l.X[b]-l.X[a], l.Y[b]-l.Y[a]
		d := math.Sqrt(dx*dx+dy*dy) + 1e-9
		f := (d - l.F.LinkLength) / d * l.F.LinkStrength * l.alpha
		fx[a] += dx * f
		fy[a] += dy * f
		fx[b] -= dx * f
		fy[b] -= dy * f
	}
	// The middle, damping, and a speed limit so a burst never throws a node far.
	for i := range n {
		if l.Pinned[i] {
			l.vx[i], l.vy[i] = 0, 0
			continue
		}
		fx[i] -= l.X[i] * l.F.Centre * l.alpha
		fy[i] -= l.Y[i] * l.F.Centre * l.alpha
		l.vx[i] = (l.vx[i] + fx[i]) * l.F.Damping
		l.vy[i] = (l.vy[i] + fy[i]) * l.F.Damping
		if s := math.Hypot(l.vx[i], l.vy[i]); s > 40 {
			l.vx[i], l.vy[i] = l.vx[i]/s*40, l.vy[i]/s*40
		}
		l.X[i] += l.vx[i]
		l.Y[i] += l.vy[i]
	}
	l.alpha *= 0.985
}

// Run ticks until the layout settles or max ticks have run.
func (l *Layout) Run(max int) {
	for i := 0; i < max && !l.Settled(); i++ {
		l.Tick()
	}
}

// Pin holds a node where it is (dragged) or lets it go.
func (l *Layout) Pin(i int, on bool) {
	if i >= 0 && i < len(l.Pinned) {
		l.Pinned[i] = on
	}
}

// Move puts a node at x, y (a drag).
func (l *Layout) Move(i int, x, y float64) {
	if i >= 0 && i < len(l.X) {
		l.X[i], l.Y[i] = x, y
		l.vx[i], l.vy[i] = 0, 0
	}
}

// Bounds is the box the nodes are in.
func (l *Layout) Bounds() (minX, minY, maxX, maxY float64) {
	if len(l.X) == 0 {
		return -1, -1, 1, 1
	}
	minX, minY, maxX, maxY = l.X[0], l.Y[0], l.X[0], l.Y[0]
	for i := range l.X {
		minX, maxX = math.Min(minX, l.X[i]), math.Max(maxX, l.X[i])
		minY, maxY = math.Min(minY, l.Y[i]), math.Max(maxY, l.Y[i])
	}
	return
}

// quadtree is a Barnes–Hut tree in flat slices, rebuilt each tick.
type quadtree struct {
	cx, cy, half []float64 // a cell's centre and half size
	mx, my, mass []float64 // its centre of mass
	child        [][4]int32
	body         []int32 // the one body in a leaf, -1 none, -2 an inner cell
	xs, ys       []float64
}

func (q *quadtree) reset() {
	q.cx, q.cy, q.half = q.cx[:0], q.cy[:0], q.half[:0]
	q.mx, q.my, q.mass = q.mx[:0], q.my[:0], q.mass[:0]
	q.child, q.body = q.child[:0], q.body[:0]
}

func (q *quadtree) cell(cx, cy, half float64) int32 {
	q.cx, q.cy, q.half = append(q.cx, cx), append(q.cy, cy), append(q.half, half)
	q.mx, q.my, q.mass = append(q.mx, 0), append(q.my, 0), append(q.mass, 0)
	q.child = append(q.child, [4]int32{-1, -1, -1, -1})
	q.body = append(q.body, -1)
	return int32(len(q.cx) - 1)
}

func (q *quadtree) build(xs, ys []float64) {
	q.reset()
	q.xs, q.ys = xs, ys
	minX, minY, maxX, maxY := xs[0], ys[0], xs[0], ys[0]
	for i := range xs {
		minX, maxX = math.Min(minX, xs[i]), math.Max(maxX, xs[i])
		minY, maxY = math.Min(minY, ys[i]), math.Max(maxY, ys[i])
	}
	half := math.Max(maxX-minX, maxY-minY)/2 + 1
	root := q.cell((minX+maxX)/2, (minY+maxY)/2, half)
	for i := range xs {
		q.insert(root, int32(i), 0)
	}
	q.summarise(root)
}

func (q *quadtree) quadrant(c int32, x, y float64) int {
	k := 0
	if x >= q.cx[c] {
		k |= 1
	}
	if y >= q.cy[c] {
		k |= 2
	}
	return k
}

func (q *quadtree) insert(c, b int32, depth int) {
	for {
		switch {
		case q.body[c] == -1 && q.child[c] == [4]int32{-1, -1, -1, -1}:
			q.body[c] = b
			return
		case q.body[c] >= 0:
			if depth > 40 { // coincident points: share the leaf
				q.body[c] = -2
				q.child[c][0] = q.cell(q.cx[c], q.cy[c], q.half[c]/2)
				q.body[q.child[c][0]] = b
				return
			}
			old := q.body[c]
			q.body[c] = -2
			q.push(c, old)
		}
		k := q.quadrant(c, q.xs[b], q.ys[b])
		if q.child[c][k] < 0 {
			h := q.half[c] / 2
			dx, dy := -h, -h
			if k&1 != 0 {
				dx = h
			}
			if k&2 != 0 {
				dy = h
			}
			q.child[c][k] = q.cell(q.cx[c]+dx, q.cy[c]+dy, h)
		}
		c = q.child[c][k]
		depth++
	}
}

// push moves a leaf's body into a child.
func (q *quadtree) push(c, b int32) {
	k := q.quadrant(c, q.xs[b], q.ys[b])
	h := q.half[c] / 2
	dx, dy := -h, -h
	if k&1 != 0 {
		dx = h
	}
	if k&2 != 0 {
		dy = h
	}
	n := q.cell(q.cx[c]+dx, q.cy[c]+dy, h)
	q.child[c][k] = n
	q.body[n] = b
}

func (q *quadtree) summarise(c int32) {
	if b := q.body[c]; b >= 0 {
		q.mx[c], q.my[c], q.mass[c] = q.xs[b], q.ys[b], 1
		return
	}
	var m, x, y float64
	for _, ch := range q.child[c] {
		if ch < 0 {
			continue
		}
		q.summarise(ch)
		m += q.mass[ch]
		x += q.mx[ch] * q.mass[ch]
		y += q.my[ch] * q.mass[ch]
	}
	if m > 0 {
		q.mx[c], q.my[c], q.mass[c] = x/m, y/m, m
	}
}

// force is the repulsion on body i at x, y from every other body, approximated by cells
// that are far enough away (size/distance < theta).
func (q *quadtree) force(i int, x, y, theta float64) (fx, fy float64) {
	stack := [64]int32{0}
	top := 1
	for top > 0 {
		top--
		c := stack[top]
		if q.mass[c] == 0 {
			continue
		}
		dx, dy := x-q.mx[c], y-q.my[c]
		d2 := dx*dx + dy*dy + 0.01
		if b := q.body[c]; b >= 0 {
			if int(b) != i {
				fx += dx / d2
				fy += dy / d2
			}
			continue
		}
		if s := 2 * q.half[c]; s*s < theta*theta*d2 {
			fx += dx / d2 * q.mass[c]
			fy += dy / d2 * q.mass[c]
			continue
		}
		for _, ch := range q.child[c] {
			if ch >= 0 && top < len(stack) {
				stack[top] = ch
				top++
			}
		}
	}
	return
}
