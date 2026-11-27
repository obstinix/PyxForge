// Package graph is the project graph (D23): the project's files and notes as nodes, and the
// links between them that their sources state (includes, imports, Markdown links), with
// metrics (degree, PageRank, components, clusters) and a deterministic force layout. It is
// pure Go: internal/ui/graphview draws it. Every parser reads text; none runs project code
// (D14). Parsing is static and best effort: what a parser cannot resolve is listed in
// Unresolved, never guessed.
package graph

import (
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// Kind is what a node is.
type Kind int

const (
	File   Kind = iota
	Note        // a Markdown file: Forge Notes (D25) are these
	Symbol      // a symbol from a language server (optional; off by default)
	Folder      // a folder's files collapsed into one node (Collapse)
)

// Node is one file, note or symbol.
type Node struct {
	ID   string // the slash-separated path from the root (a symbol: path#name)
	Kind Kind
	Ext  string // the extension without its dot, lower case
	Dir  string // the slash-separated folder, "" for the root
	Tags []string
	// Size is how many files a Folder node stands for.
	Size int
}

// Name is a node's file name; a Folder's is its folder and how many files it holds.
func (n Node) Name() string {
	if n.Kind == Folder {
		return path.Base(n.Dir) + "/ (" + strconv.Itoa(n.Size) + ")"
	}
	return n.ID[strings.LastIndex(n.ID, "/")+1:]
}

// Edge is a link from one node to another, by index into Nodes.
type Edge struct {
	From, To int
	Kind     string // "include", "import", "mod", "link", "wikilink"
}

// Unresolved is a reference a parser found but could not tie to a file in the project.
type Unresolved struct {
	From string // the node ID
	Ref  string // as written
	Kind string
}

// Graph is the project graph.
type Graph struct {
	Root       string
	Nodes      []Node
	Edges      []Edge
	Unresolved []Unresolved

	index map[string]int
	out   map[int][]Edge // edges by source, for incremental updates
	env   *env
	mu    sync.Mutex
}

// Options control Build.
type Options struct {
	// IncludeDirs are extra folders C, C++ and NASM includes resolve against (slash-separated,
	// from the root): the -I and -i folders of the build profiles (IncludeDirs).
	IncludeDirs []string
	// MaxFiles bounds the walk; 0 means 20,000.
	MaxFiles int
}

// Skipped folders: version control, dependencies and build output.
var skipDirs = map[string]bool{".git": true, ".hg": true, ".svn": true, "node_modules": true, "target": true,
	"vendor": true, "dist": true, "build": true, "out": true, "__pycache__": true, ".venv": true, "venv": true,
	".idea": true, ".vscode": true, ".pyxforge": true}

// Build walks root and reads every file's references.
func Build(root string, o Options) (*Graph, error) {
	if o.MaxFiles <= 0 {
		o.MaxFiles = 20000
	}
	g := &Graph{Root: root, index: map[string]int{}, out: map[int][]Edge{}}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // an unreadable folder is left out
		}
		name := d.Name()
		if d.IsDir() {
			if p != root && (skipDirs[name] || strings.HasPrefix(name, ".")) {
				return filepath.SkipDir
			}
			return nil
		}
		if len(g.Nodes) >= o.MaxFiles {
			return filepath.SkipAll
		}
		if !d.Type().IsRegular() {
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return nil
		}
		g.add(filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		return nil, err
	}
	g.env = newEnv(g, o)
	for i := range g.Nodes {
		g.parse(i)
	}
	g.finish()
	return g, nil
}

// add makes a node for a file.
func (g *Graph) add(id string) int {
	if i, ok := g.index[id]; ok {
		return i
	}
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(id), "."))
	n := Node{ID: id, Ext: ext, Dir: dirOf(id)}
	if ext == "md" || ext == "markdown" {
		n.Kind = Note
	}
	g.Nodes = append(g.Nodes, n)
	g.index[id] = len(g.Nodes) - 1
	return len(g.Nodes) - 1
}

func dirOf(id string) string {
	if i := strings.LastIndex(id, "/"); i >= 0 {
		return id[:i]
	}
	return ""
}

// Index is a node's index by ID.
func (g *Graph) Index(id string) (int, bool) {
	i, ok := g.index[id]
	return i, ok
}

// parse reads node i's references and records its edges.
func (g *Graph) parse(i int) {
	n := &g.Nodes[i]
	parser := parserFor(n.Ext)
	if parser == nil {
		g.out[i] = nil
		return
	}
	data, err := os.ReadFile(filepath.Join(g.Root, filepath.FromSlash(n.ID)))
	if err != nil || len(data) > 2<<20 { // unreadable, or too large to be source
		g.out[i] = nil
		return
	}
	var edges []Edge
	seen := map[int]bool{}
	tags := map[string]bool{}
	for _, r := range parser(string(data)) {
		if r.kind == "tag" {
			tags[r.ref] = true
			continue
		}
		to, ok := g.env.resolve(n.ID, r)
		ours := g.env.ours(r) // before the kind is tidied: "import?" refs are only tries
		if r.kind == "import?" {
			r.kind = "import"
		}
		if !ok {
			if ours {
				g.Unresolved = append(g.Unresolved, Unresolved{From: n.ID, Ref: r.ref, Kind: r.kind})
			}
			continue
		}
		j, ok := g.index[to]
		if !ok || j == i || seen[j] {
			continue
		}
		seen[j] = true
		edges = append(edges, Edge{From: i, To: j, Kind: r.kind})
	}
	n.Tags = n.Tags[:0]
	for t := range tags {
		n.Tags = append(n.Tags, t)
	}
	sort.Strings(n.Tags)
	g.out[i] = edges
}

// finish rebuilds the edge list from the per-node edges, in node order.
func (g *Graph) finish() {
	g.Edges = g.Edges[:0]
	for i := range g.Nodes {
		g.Edges = append(g.Edges, g.out[i]...)
	}
	sort.SliceStable(g.Unresolved, func(a, b int) bool {
		if g.Unresolved[a].From != g.Unresolved[b].From {
			return g.Unresolved[a].From < g.Unresolved[b].From
		}
		return g.Unresolved[a].Ref < g.Unresolved[b].Ref
	})
}

// Update re-reads the files that changed (absolute paths, as the explorer's watcher reports
// them): new files become nodes, deleted ones go with their edges, and only changed files are
// parsed again. It reports whether the graph changed.
func (g *Graph) Update(paths []string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	changed := false
	var reparse []int
	removed := map[string]bool{}
	for _, p := range paths {
		rel, err := filepath.Rel(g.Root, p)
		if err != nil || strings.HasPrefix(rel, "..") {
			continue
		}
		id := filepath.ToSlash(rel)
		if hidden(id) {
			continue
		}
		fi, err := os.Stat(p)
		switch {
		case err != nil: // gone
			if _, ok := g.index[id]; ok {
				removed[id] = true
				changed = true
			}
		case fi.Mode().IsRegular():
			i, ok := g.index[id]
			if !ok {
				i = g.add(id)
				g.env.addFile(id)
			}
			reparse = append(reparse, i)
			changed = true
		}
	}
	if len(removed) > 0 {
		g.remove(removed)
		reparse = nil // indexes moved: parse everything that referred to anything
		for i := range g.Nodes {
			reparse = append(reparse, i)
		}
	}
	if !changed {
		return false
	}
	// Unresolved references of a re-parsed file are found again.
	keep := g.Unresolved[:0]
	again := map[string]bool{}
	for _, i := range reparse {
		again[g.Nodes[i].ID] = true
	}
	for _, u := range g.Unresolved {
		if !again[u.From] {
			keep = append(keep, u)
		}
	}
	g.Unresolved = keep
	for _, i := range reparse {
		g.parse(i)
	}
	// A new file can satisfy references other files made before it existed.
	if len(removed) == 0 {
		for _, u := range append([]Unresolved(nil), g.Unresolved...) {
			if i, ok := g.index[u.From]; ok && !again[u.From] {
				again[u.From] = true
				g.dropUnresolved(u.From)
				g.parse(i)
			}
		}
	}
	g.finish()
	return true
}

func (g *Graph) dropUnresolved(from string) {
	keep := g.Unresolved[:0]
	for _, u := range g.Unresolved {
		if u.From != from {
			keep = append(keep, u)
		}
	}
	g.Unresolved = keep
}

// remove drops nodes and reindexes.
func (g *Graph) remove(ids map[string]bool) {
	var nodes []Node
	for _, n := range g.Nodes {
		if !ids[n.ID] {
			nodes = append(nodes, n)
		}
	}
	g.Nodes = nodes
	g.index = map[string]int{}
	for i, n := range g.Nodes {
		g.index[n.ID] = i
	}
	g.out = map[int][]Edge{}
	g.env.files = nil
	g.env.index(g)
}

func hidden(id string) bool {
	for _, part := range strings.Split(id, "/") {
		if skipDirs[part] || strings.HasPrefix(part, ".") {
			return true
		}
	}
	return false
}

// Neighbours are the nodes within depth links of node i, either way, i included, with their
// distance.
func (g *Graph) Neighbours(i, depth int) map[int]int {
	adj := g.Adjacency()
	dist := map[int]int{i: 0}
	frontier := []int{i}
	for d := 1; d <= depth; d++ {
		var next []int
		for _, n := range frontier {
			for _, m := range adj[n] {
				if _, ok := dist[m]; !ok {
					dist[m] = d
					next = append(next, m)
				}
			}
		}
		frontier = next
	}
	return dist
}

// Adjacency lists each node's neighbours, either direction, without repeats.
func (g *Graph) Adjacency() [][]int {
	adj := make([][]int, len(g.Nodes))
	for _, e := range g.Edges {
		adj[e.From] = append(adj[e.From], e.To)
		adj[e.To] = append(adj[e.To], e.From)
	}
	for i := range adj {
		sort.Ints(adj[i])
		adj[i] = compactInts(adj[i])
	}
	return adj
}

func compactInts(s []int) []int {
	if len(s) < 2 {
		return s
	}
	out := s[:1]
	for _, v := range s[1:] {
		if v != out[len(out)-1] {
			out = append(out, v)
		}
	}
	return out
}

// Sub is the graph of the given nodes and the edges among them (Local Graph). The map gives
// each sub-node's index in g.
func (g *Graph) Sub(keep map[int]int) (*Graph, []int) {
	ids := make([]int, 0, len(keep))
	for i := range keep {
		ids = append(ids, i)
	}
	sort.Ints(ids)
	s := &Graph{Root: g.Root, index: map[string]int{}}
	at := map[int]int{}
	for _, i := range ids {
		at[i] = len(s.Nodes)
		s.index[g.Nodes[i].ID] = len(s.Nodes)
		s.Nodes = append(s.Nodes, g.Nodes[i])
	}
	for _, e := range g.Edges {
		a, ok1 := at[e.From]
		b, ok2 := at[e.To]
		if ok1 && ok2 {
			s.Edges = append(s.Edges, Edge{From: a, To: b, Kind: e.Kind})
		}
	}
	return s, ids
}

// Collapse is the graph with no more than max nodes: files are gathered into one Folder node per
// folder, at the deepest folder level that keeps the count within max (owner review F5: a
// graph of thousands of files is unreadable and slow to draw). Folders in expanded keep their
// files as nodes. Links between gathered nodes merge into one. The second result maps each
// node of the collapsed graph to the nodes of g it stands for.
func (g *Graph) Collapse(max int, expanded map[string]bool) (*Graph, [][]int) {
	if len(g.Nodes) <= max && len(expanded) == 0 {
		members := make([][]int, len(g.Nodes))
		for i := range members {
			members[i] = []int{i}
		}
		return g, members
	}
	depthOf := func(dir string) int {
		if dir == "" {
			return 0
		}
		return strings.Count(dir, "/") + 1
	}
	maxDepth := 0
	for _, n := range g.Nodes {
		maxDepth = max0(maxDepth, depthOf(dirOf(n.ID)))
	}
	keyAt := func(n Node, d int) string {
		nd := dirOf(n.ID) // from the path: a node's Dir may be unset
		if expanded[nd] || depthOf(nd) < d || d == 0 {
			return "f:" + n.ID // its own node
		}
		parts := strings.SplitN(nd, "/", d+1)
		dir := strings.Join(parts[:d], "/")
		if expanded[dir] {
			return "f:" + n.ID
		}
		return "d:" + dir
	}
	count := func(d int) int {
		seen := map[string]bool{}
		for _, n := range g.Nodes {
			seen[keyAt(n, d)] = true
		}
		return len(seen)
	}
	depth := 1
	for d := maxDepth; d >= 1; d-- {
		if count(d) <= max {
			depth = d
			break
		}
	}
	c := &Graph{Root: g.Root, index: map[string]int{}}
	var members [][]int
	at := make([]int, len(g.Nodes))
	for i, n := range g.Nodes {
		k := keyAt(n, depth)
		j, ok := c.index[k]
		if !ok {
			j = len(c.Nodes)
			c.index[k] = j
			if strings.HasPrefix(k, "f:") {
				c.Nodes = append(c.Nodes, n)
			} else {
				dir := strings.TrimPrefix(k, "d:")
				c.Nodes = append(c.Nodes, Node{ID: dir + "/", Kind: Folder, Dir: dir})
			}
			members = append(members, nil)
		}
		members[j] = append(members[j], i)
		if c.Nodes[j].Kind == Folder {
			c.Nodes[j].Size++
		}
		at[i] = j
	}
	// Index by node ID too, so Index finds files and folders by what they are.
	for j, n := range c.Nodes {
		c.index[n.ID] = j
	}
	seen := map[[2]int]bool{}
	for _, e := range g.Edges {
		a, b := at[e.From], at[e.To]
		if a == b || seen[[2]int{a, b}] {
			continue
		}
		seen[[2]int{a, b}] = true
		c.Edges = append(c.Edges, Edge{From: a, To: b, Kind: e.Kind})
	}
	return c, members
}

func max0(a, b int) int {
	if a > b {
		return a
	}
	return b
}
