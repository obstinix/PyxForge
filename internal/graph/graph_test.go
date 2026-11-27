package graph

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"
)

func fixture(t *testing.T) *Graph {
	t.Helper()
	g, err := Build("testdata/proj", Options{IncludeDirs: []string{"c/include"}})
	if err != nil {
		t.Fatal(err)
	}
	return g
}

// links lists node from's edges as "to (kind)".
func links(g *Graph, from string) []string {
	i, ok := g.Index(from)
	if !ok {
		return nil
	}
	var out []string
	for _, e := range g.Edges {
		if e.From == i {
			out = append(out, g.Nodes[e.To].ID+" ("+e.Kind+")")
		}
	}
	sort.Strings(out)
	return out
}

func unresolved(g *Graph, from string) []string {
	var out []string
	for _, u := range g.Unresolved {
		if u.From == from {
			out = append(out, u.Ref)
		}
	}
	sort.Strings(out)
	return out
}

func eq(t *testing.T, what string, got, want []string) {
	t.Helper()
	sort.Strings(want)
	if !slices.Equal(got, want) {
		t.Errorf("%s:\n got  %q\n want %q", what, got, want)
	}
}

func TestCIncludes(t *testing.T) {
	g := fixture(t)
	eq(t, "main.c", links(g, "c/src/main.c"), []string{"c/src/util.h (include)", "c/include/api.h (include)"})
	eq(t, "main.c unresolved", unresolved(g, "c/src/main.c"), []string{"missing.h"}) // <stdio.h> is the system's; the comment is not read
	eq(t, "util.h", links(g, "c/src/util.h"), []string{"c/include/api.h (include)"}) // through the include folder
}

func TestNASMIncludes(t *testing.T) {
	g := fixture(t)
	eq(t, "boot.asm", links(g, "nasm/boot.asm"), []string{"nasm/macros.inc (include)", "nasm/io/serial.inc (include)"})
	eq(t, "boot.asm unresolved", unresolved(g, "nasm/boot.asm"), []string{"nope.inc"})
}

func TestGoImports(t *testing.T) {
	g := fixture(t)
	// A package is represented by its doc.go, else its first file; tests are not packages.
	eq(t, "main.go", links(g, "go/app/main.go"), []string{"go/lib/util/b.go (import)", "go/lib/doc.go (import)"})
	eq(t, "main.go unresolved", unresolved(g, "go/app/main.go"), []string{"example.com/demo/lib/gone"}) // fmt is the standard library's
}

func TestRustModules(t *testing.T) {
	g := fixture(t)
	eq(t, "main.rs", links(g, "rust/src/main.rs"), []string{"rust/src/net/mod.rs (mod)", "rust/src/config.rs (mod)", "rust/src/net/socket.rs (use)"})
	eq(t, "main.rs unresolved", unresolved(g, "rust/src/main.rs"), []string{"absent"})
	eq(t, "net/mod.rs", links(g, "rust/src/net/mod.rs"), []string{"rust/src/net/socket.rs (mod)"})
}

func TestPythonImports(t *testing.T) {
	g := fixture(t)
	eq(t, "main.py", links(g, "py/pkg/main.py"), []string{"py/pkg/sub/__init__.py (import)", "py/pkg/sub/helpers.py (import)",
		"py/pkg/__init__.py (import)", "py/pkg/other.py (import)"})
	eq(t, "main.py unresolved", unresolved(g, "py/pkg/main.py"), []string{"..gone"}) // os is the standard library's
}

// TestJSImports: the fixture is written at run time, because the repository holds no .js or
// .ts file (tools/forbidcheck: no web technology in PyxForge); the graph only reads them.
func TestJSImports(t *testing.T) {
	dir := t.TempDir()
	for name, text := range map[string]string{
		"src/index.ts": "import { a } from './lib/a';\nexport * from './lib';\nconst b = require('./lib/b.js');\n" +
			"import('./lib/lazy');\nimport React from 'react';\nimport x from './nothing';\n",
		"src/lib/a.ts":     "export const a = 1\n",
		"src/lib/index.js": "export {}\n",
		"src/lib/b.js":     "",
		"src/lib/lazy.tsx": "",
	} {
		p := filepath.Join(dir, filepath.FromSlash(name))
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, []byte(text), 0o644)
	}
	g, err := Build(dir, Options{})
	if err != nil {
		t.Fatal(err)
	}
	eq(t, "index.ts", links(g, "src/index.ts"), []string{"src/lib/a.ts (import)", "src/lib/index.js (import)",
		"src/lib/b.js (import)", "src/lib/lazy.tsx (import)"})
	eq(t, "index.ts unresolved", unresolved(g, "src/index.ts"), []string{"./nothing"}) // react is a package
}

func TestMarkdownLinks(t *testing.T) {
	g := fixture(t)
	eq(t, "README.md", links(g, "md/README.md"), []string{"c/include/api.h (link)", "md/notes/Ideas.md (link)", "md/Plans.md (wikilink)"})
	eq(t, "README.md unresolved", unresolved(g, "md/README.md"), []string{"Nowhere"})
	i, _ := g.Index("md/README.md")
	if !slices.Equal(g.Nodes[i].Tags, []string{"build", "todo/later"}) || g.Nodes[i].Kind != Note {
		t.Errorf("tags %v, kind %v", g.Nodes[i].Tags, g.Nodes[i].Kind)
	}
	eq(t, "Ideas.md", links(g, "md/notes/Ideas.md"), []string{"md/README.md (wikilink)", "nasm/boot.asm (wikilink)", "c/src/main.c (wikilink)"})
	eq(t, "Ideas.md unresolved", unresolved(g, "md/notes/Ideas.md"), []string{"api.h"}) // two files would do: not guessed
}

func TestIncludeDirsFromProfiles(t *testing.T) {
	if got := IncludeDirs(nil); got != nil {
		t.Errorf("nil config: %v", got)
	}
}

// TestUpdate: a changed file is parsed again, a new one joins (and satisfies a reference that
// was unresolved), a deleted one goes with its edges.
func TestUpdate(t *testing.T) {
	dir := t.TempDir()
	write := func(name, text string) string {
		p := filepath.Join(dir, filepath.FromSlash(name))
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, []byte(text), 0o644)
		return p
	}
	a := write("a.c", "#include \"b.h\"\n")
	g, err := Build(dir, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(g.Edges) != 0 || len(g.Unresolved) != 1 {
		t.Fatalf("before: %v %v", g.Edges, g.Unresolved)
	}
	b := write("b.h", "#include \"c.h\"\n")
	write("c.h", "")
	if !g.Update([]string{b, filepath.Join(dir, "c.h")}) {
		t.Fatal("Update reported no change")
	}
	eq(t, "a.c after b.h appeared", links(g, "a.c"), []string{"b.h (include)"})
	eq(t, "b.h", links(g, "b.h"), []string{"c.h (include)"})
	write("a.c", "/* nothing */\n")
	g.Update([]string{a})
	if l := links(g, "a.c"); len(l) != 0 {
		t.Errorf("a.c's include stayed: %v", l)
	}
	os.Remove(b)
	g.Update([]string{b})
	if _, ok := g.Index("b.h"); ok || len(g.Edges) != 0 {
		t.Errorf("b.h stayed: edges %v", g.Edges)
	}
	if g.Update([]string{filepath.Join(dir, ".git", "HEAD")}) {
		t.Error("a change under .git changed the graph")
	}
}

// TestMetrics on known small graphs.
func TestMetrics(t *testing.T) {
	// A star (0 at the centre, edges in) plus a separate pair and a lone node.
	g := &Graph{Nodes: make([]Node, 7)}
	for i := range g.Nodes {
		g.Nodes[i].ID = string(rune('a' + i))
	}
	for _, e := range [][2]int{{1, 0}, {2, 0}, {3, 0}, {4, 5}} {
		g.Edges = append(g.Edges, Edge{From: e[0], To: e[1]})
	}
	m := g.Compute()
	if !slices.Equal(m.Degree, []int{3, 1, 1, 1, 1, 1, 0}) {
		t.Errorf("degree %v", m.Degree)
	}
	sum := 0.0
	for _, p := range m.PageRank {
		sum += p
	}
	if sum < 0.999 || sum > 1.001 {
		t.Errorf("PageRank sums to %f", sum)
	}
	if order := g.ByPageRank(m); order[0] != 0 {
		t.Errorf("the hub is not first: %v (%v)", order, m.PageRank)
	}
	if m.Components != 3 || m.Component[1] != m.Component[0] || m.Component[4] == m.Component[0] {
		t.Errorf("components %d %v", m.Components, m.Component)
	}
	// Two triangles joined by one edge: two clusters.
	h := &Graph{Nodes: make([]Node, 6)}
	for _, e := range [][2]int{{0, 1}, {1, 2}, {2, 0}, {3, 4}, {4, 5}, {5, 3}, {2, 3}} {
		h.Edges = append(h.Edges, Edge{From: e[0], To: e[1]})
	}
	hm := h.Compute()
	if hm.Clusters != 2 || hm.Cluster[0] != hm.Cluster[1] || hm.Cluster[0] == hm.Cluster[4] {
		t.Errorf("clusters %d %v", hm.Clusters, hm.Cluster)
	}
	if again := h.Compute(); !slices.Equal(again.Cluster, hm.Cluster) {
		t.Error("clusters differ between runs")
	}
}

// TestNeighbours: depth 1 and 2 around a node, either direction.
func TestNeighbours(t *testing.T) {
	g := &Graph{Nodes: make([]Node, 5)}
	for _, e := range [][2]int{{0, 1}, {2, 1}, {2, 3}, {3, 4}} {
		g.Edges = append(g.Edges, Edge{From: e[0], To: e[1]})
	}
	if n := g.Neighbours(1, 1); len(n) != 3 || n[0] != 1 || n[2] != 1 {
		t.Errorf("depth 1: %v", n)
	}
	if n := g.Neighbours(1, 2); len(n) != 4 || n[3] != 2 {
		t.Errorf("depth 2: %v", n)
	}
	sub, ids := g.Sub(g.Neighbours(1, 1))
	if len(sub.Nodes) != 3 || len(sub.Edges) != 2 || !slices.Equal(ids, []int{0, 1, 2}) {
		t.Errorf("sub: %d nodes, %d edges, ids %v", len(sub.Nodes), len(sub.Edges), ids)
	}
}

// synthetic is a graph of n nodes and m edges with a fixed pattern.
func synthetic(n, m int) *Graph {
	g := &Graph{Nodes: make([]Node, n)}
	for i := range g.Nodes {
		g.Nodes[i].ID = "n" + strings.Repeat("x", i%7) + string(rune('A'+i%26)) + string(rune('0'+i%10))
	}
	x := uint64(88172645463325252)
	for range m {
		x ^= x << 13
		x ^= x >> 7
		x ^= x << 17
		a := int(x % uint64(n))
		b := int((x >> 20) % uint64(n))
		if a != b {
			g.Edges = append(g.Edges, Edge{From: a, To: b})
		}
	}
	return g
}

// TestLayoutDeterministic: the same graph and seed give the same positions; another seed does not.
func TestLayoutDeterministic(t *testing.T) {
	for _, n := range []int{60, 700} { // below and above the Barnes–Hut threshold
		g := synthetic(n, n*2)
		a, b, c := NewLayout(g, 7), NewLayout(g, 7), NewLayout(g, 8)
		for range 40 {
			a.Tick()
			b.Tick()
			c.Tick()
		}
		if !slices.Equal(a.X, b.X) || !slices.Equal(a.Y, b.Y) {
			t.Errorf("%d nodes: the same seed gave different layouts", n)
		}
		if slices.Equal(a.X, c.X) {
			t.Errorf("%d nodes: another seed gave the same layout", n)
		}
	}
}

// TestLayoutSeparatesAndPins: linked nodes end near their link length, unlinked ones apart; a
// pinned node does not move.
func TestLayoutSeparatesAndPins(t *testing.T) {
	g := synthetic(30, 40)
	l := NewLayout(g, 1)
	l.Move(0, 500, 500)
	l.Pin(0, true)
	l.Run(2000)
	if l.X[0] != 500 || l.Y[0] != 500 {
		t.Errorf("the pinned node moved to %f,%f", l.X[0], l.Y[0])
	}
	if !l.Settled() {
		t.Error("the layout did not settle in 2000 ticks")
	}
	for i := 1; i < len(l.X); i++ {
		for j := i + 1; j < len(l.X); j++ {
			if d := (l.X[i]-l.X[j])*(l.X[i]-l.X[j]) + (l.Y[i]-l.Y[j])*(l.Y[i]-l.Y[j]); d < 1 {
				t.Fatalf("nodes %d and %d overlap", i, j)
			}
		}
	}
}

// TestBudgets: a layout tick at 2,000 nodes and 6,000 edges under 8 ms, and the graph of this
// repository built in under a second (D23). Timing tests mean little under the race detector
// or on a loaded machine, so they take the best of several runs and skip under -race.
func TestBudgets(t *testing.T) {
	if raceEnabled {
		t.Skip("timings under the race detector are not the program's")
	}
	g := synthetic(2000, 6000)
	l := NewLayout(g, 1)
	// Twenty ticks timed together: Windows' clock is too coarse to time one.
	best := time.Hour
	for range 3 {
		start := time.Now()
		for range 20 {
			l.Tick()
		}
		best = min(best, time.Since(start)/20)
	}
	if best > 8*time.Millisecond {
		t.Errorf("a tick at 2,000 nodes / 6,000 edges takes %v", best)
	}
	t.Logf("tick at 2,000/6,000: %v", best)
	root := repoRoot(t)
	bestBuild := time.Hour
	var repo *Graph
	for range 3 {
		start := time.Now()
		var err error
		repo, err = Build(root, Options{})
		if err != nil {
			t.Fatal(err)
		}
		bestBuild = min(bestBuild, time.Since(start))
	}
	if bestBuild > time.Second {
		t.Errorf("building the repository's graph takes %v", bestBuild)
	}
	t.Logf("repository: %d nodes, %d edges, %d unresolved, built in %v", len(repo.Nodes), len(repo.Edges), len(repo.Unresolved), bestBuild)
}

func repoRoot(t *testing.T) string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Dir(filepath.Dir(filepath.Dir(file)))
}

func BenchmarkLayoutTick2000(b *testing.B) {
	l := NewLayout(synthetic(2000, 6000), 1)
	b.ResetTimer()
	for range b.N {
		l.Tick()
	}
}

func BenchmarkBuildRepository(b *testing.B) {
	root := filepath.Join("..", "..")
	for range b.N {
		if _, err := Build(root, Options{}); err != nil {
			b.Fatal(err)
		}
	}
}
