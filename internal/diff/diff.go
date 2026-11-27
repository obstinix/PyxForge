// Package diff compares two texts line by line and lays the result out side by side, for the
// Git tab's diff view. It uses Myers' O(ND) algorithm ("An O(ND) Difference Algorithm and Its
// Variations", Algorithmica 1986), which finds a shortest edit script.
package diff

import "strings"

// Op is what happened to a line.
type Op uint8

const (
	Equal  Op = iota // in both texts
	Delete           // only in the old text
	Insert           // only in the new text
)

// Edit is one line of an edit script: the line's index in the old text (Delete, Equal) and
// in the new one (Insert, Equal); -1 where it has none.
type Edit struct {
	Op       Op
	Old, New int
}

// Lines splits a text into lines without their endings. A final newline does not start an
// empty last line, and CRLF counts as one ending.
func Lines(text string) []string {
	if text == "" {
		return nil
	}
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.TrimSuffix(text, "\n")
	return strings.Split(text, "\n")
}

// MaxEdits caps the search: when the texts differ by more lines than this, Compute stops
// looking for the shortest script and reports the rest of both texts as replaced, which is
// still correct, only less tidy. It keeps a huge rewrite from taking seconds and memory.
const MaxEdits = 4000

// Compute returns an edit script turning a into b: every line of a once (Equal or Delete) and
// every line of b once (Equal or Insert), in order.
func Compute(a, b []string) []Edit {
	// Common prefix and suffix are cheap and common; take them out of the search.
	pre := 0
	for pre < len(a) && pre < len(b) && a[pre] == b[pre] {
		pre++
	}
	suf := 0
	for suf < len(a)-pre && suf < len(b)-pre && a[len(a)-1-suf] == b[len(b)-1-suf] {
		suf++
	}
	var out []Edit
	for i := 0; i < pre; i++ {
		out = append(out, Edit{Equal, i, i})
	}
	out = append(out, myers(a[pre:len(a)-suf], b[pre:len(b)-suf], pre, pre)...)
	for i := 0; i < suf; i++ {
		out = append(out, Edit{Equal, len(a) - suf + i, len(b) - suf + i})
	}
	return out
}

// myers diffs a and b, whose first lines are at offA and offB in the whole texts.
func myers(a, b []string, offA, offB int) []Edit {
	n, m := len(a), len(b)
	if n == 0 || m == 0 {
		return replaced(n, m, offA, offB)
	}
	max := n + m
	if max > MaxEdits {
		max = MaxEdits
	}
	// v[k+max] is the furthest x reached on diagonal k; trace keeps a copy per step d.
	v := make([]int, 2*max+2)
	var trace [][]int
	found := -1
	for d := 0; d <= max && found < 0; d++ {
		trace = append(trace, append([]int(nil), v...))
		for k := -d; k <= d; k += 2 {
			var x int
			if k == -d || (k != d && v[k-1+max] < v[k+1+max]) {
				x = v[k+1+max] // down: an insertion
			} else {
				x = v[k-1+max] + 1 // right: a deletion
			}
			y := x - k
			for x < n && y < m && a[x] == b[y] {
				x++
				y++
			}
			v[k+max] = x
			if x >= n && y >= m {
				found = d
				break
			}
		}
	}
	if found < 0 {
		return replaced(n, m, offA, offB)
	}
	// Walk the trace back from (n, m), collecting the script in reverse.
	var rev []Edit
	x, y := n, m
	for d := found; d > 0; d-- {
		vp := trace[d]
		k := x - y
		var pk int
		if k == -d || (k != d && vp[k-1+max] < vp[k+1+max]) {
			pk = k + 1
		} else {
			pk = k - 1
		}
		px := vp[pk+max]
		py := px - pk
		for x > px && y > py {
			x--
			y--
			rev = append(rev, Edit{Equal, x + offA, y + offB})
		}
		if x == px {
			y--
			rev = append(rev, Edit{Insert, -1, y + offB})
		} else {
			x--
			rev = append(rev, Edit{Delete, x + offA, -1})
		}
	}
	for x > 0 && y > 0 {
		x--
		y--
		rev = append(rev, Edit{Equal, x + offA, y + offB})
	}
	out := make([]Edit, len(rev))
	for i, e := range rev {
		out[len(rev)-1-i] = e
	}
	return out
}

// replaced is the script that deletes all n old lines and inserts all m new ones.
func replaced(n, m, offA, offB int) []Edit {
	out := make([]Edit, 0, n+m)
	for i := 0; i < n; i++ {
		out = append(out, Edit{Delete, i + offA, -1})
	}
	for i := 0; i < m; i++ {
		out = append(out, Edit{Insert, -1, i + offB})
	}
	return out
}

// Row is one line of a side-by-side view: the old line on the left and the new one on the
// right, by index; -1 leaves that side blank.
type Row struct {
	Old, New int
	Op       Op // Equal; Delete or Insert for a one-sided row; Insert with both sides for a changed line
}

// Changed reports whether a row is part of a change.
func (r Row) Changed() bool { return r.Op != Equal }

// SideBySide pairs an edit script into rows: equal lines face each other, and within a run of
// changes the deleted lines face the inserted ones, the longer side continuing alone.
func SideBySide(edits []Edit) []Row {
	var rows []Row
	for i := 0; i < len(edits); {
		if edits[i].Op == Equal {
			rows = append(rows, Row{edits[i].Old, edits[i].New, Equal})
			i++
			continue
		}
		var del, ins []int
		for ; i < len(edits) && edits[i].Op != Equal; i++ {
			if edits[i].Op == Delete {
				del = append(del, edits[i].Old)
			} else {
				ins = append(ins, edits[i].New)
			}
		}
		for j := 0; j < len(del) || j < len(ins); j++ {
			r := Row{-1, -1, Insert}
			switch {
			case j < len(del) && j < len(ins):
				r.Old, r.New = del[j], ins[j]
			case j < len(del):
				r.Old, r.Op = del[j], Delete
			default:
				r.New = ins[j]
			}
			rows = append(rows, r)
		}
	}
	return rows
}

// Hunks returns the index of the first row of each run of changed rows.
func Hunks(rows []Row) []int {
	var out []int
	for i, r := range rows {
		if r.Changed() && (i == 0 || !rows[i-1].Changed()) {
			out = append(out, i)
		}
	}
	return out
}

// Stats counts the deleted and inserted lines of a script.
func Stats(edits []Edit) (deleted, inserted int) {
	for _, e := range edits {
		switch e.Op {
		case Delete:
			deleted++
		case Insert:
			inserted++
		}
	}
	return
}
