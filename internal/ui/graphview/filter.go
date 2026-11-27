package graphview

import (
	"fmt"
	"sort"
	"strings"

	"github.com/obstinix/PyxForge/internal/graph"
)

// Settings are everything the controls panel sets; a preset saves them by name.
type Settings struct {
	Query       string  `json:"query"`
	Orphans     bool    `json:"orphans"`     // show nodes without links
	Attachments bool    `json:"attachments"` // show files that are not source or notes
	NotesOnly   bool    `json:"notesOnly"`
	GroupBy     string  `json:"groupBy"` // "folder", "extension", "cluster", "query"
	Groups      []Group `json:"groups,omitempty"`
	Scheme      Scheme  `json:"scheme"`
	NodeSize    string  `json:"nodeSize"` // "uniform", "degree", "pagerank"
	SizeFactor  float64 `json:"sizeFactor"`
	LinkWidth   float64 `json:"linkWidth"`
	LinkOpacity float64 `json:"linkOpacity"`
	Arrows      bool    `json:"arrows"`
	LabelAt     float64 `json:"labelAt"` // the screen radius a node's label shows from
	Glow        string  `json:"glow"`    // "none", "hubs", "all"
	Forces      graph.Forces
}

// Group is a query whose matches take a colour (Groups > query).
type Group struct {
	Query  string `json:"query"`
	Colour int    `json:"colour"` // an index into the scheme's colours
}

// DefaultSettings are a new view's.
func DefaultSettings() Settings {
	return Settings{Orphans: true, Attachments: false, GroupBy: "folder", Scheme: SchemeTheme, NodeSize: "pagerank",
		SizeFactor: 1, LinkWidth: 1, LinkOpacity: 0.45, LabelAt: 5, Glow: "hubs", Forces: graph.DefaultForces}
}

// term is one word of a query.
type term struct {
	field, value string
	not          bool
}

// parseQuery splits a query into its words: path:x, ext:x, tag:x, a plain word (the name), and
// any of them with a leading - to exclude.
func parseQuery(q string) []term {
	var out []term
	for _, w := range strings.Fields(strings.ToLower(q)) {
		t := term{}
		if strings.HasPrefix(w, "-") && len(w) > 1 {
			t.not, w = true, w[1:]
		}
		if f, v, ok := strings.Cut(w, ":"); ok && (f == "path" || f == "ext" || f == "tag" || f == "file") {
			t.field, t.value = f, v
		} else {
			t.value = w
		}
		if t.field == "ext" {
			t.value = strings.TrimPrefix(t.value, ".")
		}
		if t.field == "tag" {
			t.value = strings.TrimPrefix(t.value, "#")
		}
		out = append(out, t)
	}
	return out
}

// matches reports whether a node fits every word of a query.
func matches(n graph.Node, terms []term) bool {
	for _, t := range terms {
		var hit bool
		switch t.field {
		case "path":
			hit = strings.Contains(strings.ToLower(n.ID), t.value)
		case "ext":
			hit = n.Ext == t.value
		case "tag":
			for _, tg := range n.Tags {
				if tg == t.value || strings.HasPrefix(tg, t.value+"/") {
					hit = true
				}
			}
		default: // the file name
			hit = strings.Contains(strings.ToLower(n.Name()), t.value)
		}
		if hit == t.not {
			return false
		}
	}
	return true
}

// sourceExt are files that state links (graph's parsers) or are text a project is made of:
// everything else is an attachment (images, binaries, archives), hidden unless asked for.
var sourceExt = map[string]bool{"c": true, "h": true, "cc": true, "cpp": true, "cxx": true, "hpp": true, "hh": true,
	"asm": true, "nasm": true, "s": true, "inc": true, "mac": true, "go": true, "rs": true, "py": true, "pyi": true,
	"js": true, "jsx": true, "ts": true, "tsx": true, "mjs": true, "cjs": true, "md": true, "markdown": true,
	"toml": true, "json": true, "yaml": true, "yml": true, "ld": true, "lds": true, "mod": true, "sum": true,
	"sh": true, "ps1": true, "txt": true, "html": true, "css": true, "lua": true, "zig": true, "java": true, "kt": true}

// Attachment reports whether a node is an attachment.
func Attachment(n graph.Node) bool { return n.Kind == graph.File && !sourceExt[n.Ext] }

// visible works out which nodes the settings show.
func visible(g *graph.Graph, m graph.Metrics, s Settings) []bool {
	terms := parseQuery(s.Query)
	out := make([]bool, len(g.Nodes))
	for i, n := range g.Nodes {
		switch {
		case s.NotesOnly && n.Kind != graph.Note:
		case !s.Attachments && Attachment(n):
		case !s.Orphans && m.Degree[i] == 0:
		case !matches(n, terms):
		default:
			out[i] = true
		}
	}
	return out
}

// groups gives each node its colour index under the settings. Folders, extensions and
// clusters are numbered by size, largest first, so the biggest groups get the scheme's first,
// most distinct colours; ties go by name.
func groups(g *graph.Graph, m graph.Metrics, s Settings) []int {
	out := make([]int, len(g.Nodes))
	if s.GroupBy == "query" {
		var queries [][]term
		for _, gr := range s.Groups {
			queries = append(queries, parseQuery(gr.Query))
		}
		for i, n := range g.Nodes {
			out[i] = -1 // no group: the link colour
			for qi, q := range queries {
				if len(q) > 0 && matches(n, q) {
					out[i] = s.Groups[qi].Colour
					break
				}
			}
		}
		return out
	}
	keyOf := func(i int, n graph.Node) string {
		switch s.GroupBy {
		case "extension":
			return n.Ext
		case "cluster":
			return fmt.Sprintf("%09d", m.Cluster[i])
		}
		top, _, ok := strings.Cut(n.ID, "/")
		if !ok {
			return "."
		}
		return top
	}
	size := map[string]int{}
	keys := make([]string, len(g.Nodes))
	for i, n := range g.Nodes {
		keys[i] = keyOf(i, n)
		size[keys[i]]++
	}
	names := make([]string, 0, len(size))
	for k := range size {
		names = append(names, k)
	}
	sort.Slice(names, func(a, b int) bool {
		if size[names[a]] != size[names[b]] {
			return size[names[a]] > size[names[b]]
		}
		return names[a] < names[b]
	})
	rank := map[string]int{}
	for i, k := range names {
		rank[k] = i
	}
	for i := range g.Nodes {
		out[i] = rank[keys[i]]
	}
	return out
}
