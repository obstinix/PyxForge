package graph

import "sort"

// Metrics are what the graph view sizes and colours nodes by.
type Metrics struct {
	Degree     []int     // links either way
	PageRank   []float64 // sums to 1
	Component  []int     // connected component, numbered from 0 by first node
	Cluster    []int     // label-propagation community, numbered from 0 by first node
	Components int
	Clusters   int
}

// PageRankRounds is the number of power-iteration rounds.
const PageRankRounds = 30

// Compute works out every metric.
func (g *Graph) Compute() Metrics {
	n := len(g.Nodes)
	m := Metrics{Degree: make([]int, n)}
	adj := g.Adjacency()
	for i := range adj {
		m.Degree[i] = len(adj[i])
	}
	m.PageRank = pageRank(n, g.Edges)
	m.Component, m.Components = components(adj)
	m.Cluster, m.Clusters = clusters(adj)
	return m
}

// pageRank is PageRank by power iteration with damping 0.85; a node without outgoing links
// shares its rank with every node.
func pageRank(n int, edges []Edge) []float64 {
	if n == 0 {
		return nil
	}
	const d = 0.85
	out := make([]int, n)
	for _, e := range edges {
		out[e.From]++
	}
	pr := make([]float64, n)
	for i := range pr {
		pr[i] = 1 / float64(n)
	}
	next := make([]float64, n)
	for range PageRankRounds {
		sink := 0.0
		for i := range pr {
			if out[i] == 0 {
				sink += pr[i]
			}
		}
		base := (1-d)/float64(n) + d*sink/float64(n)
		for i := range next {
			next[i] = base
		}
		for _, e := range edges {
			next[e.To] += d * pr[e.From] / float64(out[e.From])
		}
		pr, next = next, pr
	}
	return pr
}

// components labels connected components, ignoring direction.
func components(adj [][]int) ([]int, int) {
	comp := make([]int, len(adj))
	for i := range comp {
		comp[i] = -1
	}
	k := 0
	for i := range adj {
		if comp[i] >= 0 {
			continue
		}
		stack := []int{i}
		comp[i] = k
		for len(stack) > 0 {
			v := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			for _, w := range adj[v] {
				if comp[w] < 0 {
					comp[w] = k
					stack = append(stack, w)
				}
			}
		}
		k++
	}
	return comp, k
}

// ClusterSeed fixes the order label propagation visits nodes in, so the same graph always
// gives the same clusters.
const ClusterSeed = 0x9e3779b97f4a7c15

// clusters is label propagation: every node starts as its own label and takes the label most
// of its neighbours have, keeping its own on a tie and otherwise breaking ties by a hash of the
// label, nodes visited in a shuffled order, until nothing changes (at most 50 rounds). The
// shuffle and the hash come from ClusterSeed: no randomness at run time, and no sweep in index
// order, which lets one label flood a whole component.
func clusters(adj [][]int) ([]int, int) {
	n := len(adj)
	label := make([]int, n)
	for i := range label {
		label[i] = i
	}
	order := make([]int, n)
	for i := range order {
		order[i] = i
	}
	x := uint64(ClusterSeed)
	next := func() uint64 { // xorshift64*
		x ^= x >> 12
		x ^= x << 25
		x ^= x >> 27
		return x * 2685821657736338717
	}
	for i := n - 1; i > 0; i-- {
		j := int(next() % uint64(i+1))
		order[i], order[j] = order[j], order[i]
	}
	rank := func(l int) uint64 { // a fixed pseudo-random order of labels for ties
		h := uint64(l)*0x9e3779b97f4a7c15 + ClusterSeed
		h ^= h >> 31
		return h * 0xbf58476d1ce4e5b9
	}
	count := map[int]int{}
	for range 50 {
		changed := false
		for _, v := range order {
			if len(adj[v]) == 0 {
				continue
			}
			clear(count)
			for _, w := range adj[v] {
				count[label[w]]++
			}
			best, bestN := label[v], count[label[v]]
			for l, c := range count {
				if c > bestN || c == bestN && l != label[v] && best != label[v] && rank(l) < rank(best) {
					best, bestN = l, c
				}
			}
			if best != label[v] {
				label[v] = best
				changed = true
			}
		}
		if !changed {
			break
		}
	}
	// Number the clusters 0… by their first node.
	ids := map[int]int{}
	out := make([]int, n)
	for i, l := range label {
		id, ok := ids[l]
		if !ok {
			id = len(ids)
			ids[l] = id
		}
		out[i] = id
	}
	return out, len(ids)
}

// ByPageRank lists node indexes from the highest PageRank down, ties by ID (Tab's order).
func (g *Graph) ByPageRank(m Metrics) []int {
	idx := make([]int, len(g.Nodes))
	for i := range idx {
		idx[i] = i
	}
	sort.SliceStable(idx, func(a, b int) bool {
		pa, pb := m.PageRank[idx[a]], m.PageRank[idx[b]]
		if pa != pb {
			return pa > pb
		}
		return g.Nodes[idx[a]].ID < g.Nodes[idx[b]].ID
	})
	return idx
}
