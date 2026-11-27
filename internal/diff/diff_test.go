package diff

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"
)

// check verifies that a script accounts for every line of a and b once, in order, and that
// its Equal lines really are equal.
func check(t *testing.T, a, b []string, edits []Edit) {
	t.Helper()
	i, j := 0, 0
	for _, e := range edits {
		switch e.Op {
		case Equal:
			if e.Old != i || e.New != j || a[i] != b[j] {
				t.Fatalf("equal %+v at %d,%d", e, i, j)
			}
			i++
			j++
		case Delete:
			if e.Old != i || e.New != -1 {
				t.Fatalf("delete %+v at %d", e, i)
			}
			i++
		case Insert:
			if e.New != j || e.Old != -1 {
				t.Fatalf("insert %+v at %d", e, j)
			}
			j++
		}
	}
	if i != len(a) || j != len(b) {
		t.Fatalf("script covers %d/%d old and %d/%d new lines", i, len(a), j, len(b))
	}
}

func script(edits []Edit, a, b []string) string {
	var sb strings.Builder
	for _, e := range edits {
		switch e.Op {
		case Equal:
			sb.WriteString(" " + a[e.Old])
		case Delete:
			sb.WriteString("-" + a[e.Old])
		case Insert:
			sb.WriteString("+" + b[e.New])
		}
	}
	return sb.String()
}

func TestCompute(t *testing.T) {
	cases := []struct{ a, b, want string }{
		{"", "", ""},
		{"a\nb\n", "a\nb\n", " a b"},
		{"", "x\ny\n", "+x+y"},
		{"x\ny\n", "", "-x-y"},
		{"a\nb\nc\n", "a\nc\n", " a-b c"},
		{"a\nc\n", "a\nb\nc\n", " a+b c"},
		{"a\nb\nc\n", "a\nB\nc\n", " a-b+B c"},
		// Myers' own example: a shortest script has five edits.
		{"A\nB\nC\nA\nB\nB\nA\n", "C\nB\nA\nB\nA\nC\n", ""},
		{"a\r\nb\r\n", "a\nb\n", " a b"}, // line endings alone are not a change
	}
	for _, c := range cases {
		a, b := Lines(c.a), Lines(c.b)
		e := Compute(a, b)
		check(t, a, b, e)
		got := script(e, a, b)
		if c.want != "" && got != c.want {
			t.Errorf("%q -> %q: %s, want %s", c.a, c.b, got, c.want)
		}
		if c.a == "A\nB\nC\nA\nB\nB\nA\n" {
			if d, i := Stats(e); d+i != 5 {
				t.Errorf("Myers' example took %d edits (%s), want 5", d+i, got)
			}
		}
	}
}

// TestComputeRandom: random texts over a small alphabet, so lines repeat, always give a valid
// script of the shortest length, and an unchanged text gives no edits.
func TestComputeRandom(t *testing.T) {
	r := rand.New(rand.NewSource(1))
	gen := func() []string {
		out := make([]string, r.Intn(40))
		for i := range out {
			out[i] = fmt.Sprint(r.Intn(5))
		}
		return out
	}
	for range 2000 {
		a, b := gen(), gen()
		e := Compute(a, b)
		check(t, a, b, e)
		if d, i := Stats(e); d+i != len(a)+len(b)-2*lcs(a, b) {
			t.Fatalf("%v -> %v: %d edits, shortest is %d", a, b, d+i, len(a)+len(b)-2*lcs(a, b))
		}
		if d, i := Stats(Compute(a, a)); d+i != 0 {
			t.Fatal("an unchanged text has edits")
		}
	}
}

// lcs is the length of a longest common subsequence, by dynamic programming.
func lcs(a, b []string) int {
	t := make([][]int, len(a)+1)
	for i := range t {
		t[i] = make([]int, len(b)+1)
	}
	for i := len(a) - 1; i >= 0; i-- {
		for j := len(b) - 1; j >= 0; j-- {
			if a[i] == b[j] {
				t[i][j] = t[i+1][j+1] + 1
			} else {
				t[i][j] = max(t[i+1][j], t[i][j+1])
			}
		}
	}
	return t[0][0]
}

// TestLargeRewrite: texts that differ everywhere stay correct and quick past MaxEdits.
func TestLargeRewrite(t *testing.T) {
	var a, b []string
	for i := range 6000 {
		a = append(a, fmt.Sprint("old ", i))
		b = append(b, fmt.Sprint("new ", i))
	}
	e := Compute(a, b)
	check(t, a, b, e)
	if d, i := Stats(e); d != 6000 || i != 6000 {
		t.Errorf("stats %d %d", d, i)
	}
}

func TestSideBySide(t *testing.T) {
	a := Lines("keep\nold1\nold2\nold3\nkeep2\ngone\n")
	b := Lines("keep\nnew1\nkeep2\nadded\n")
	rows := SideBySide(Compute(a, b))
	var got []string
	for _, r := range rows {
		l, rt := "_", "_"
		if r.Old >= 0 {
			l = a[r.Old]
		}
		if r.New >= 0 {
			rt = b[r.New]
		}
		got = append(got, fmt.Sprintf("%s|%s|%d", l, rt, r.Op))
	}
	want := "[keep|keep|0 old1|new1|2 old2|_|1 old3|_|1 keep2|keep2|0 gone|added|2]"
	if fmt.Sprint(got) != want {
		t.Errorf("rows %v\nwant %s", got, want)
	}
	if h := Hunks(rows); fmt.Sprint(h) != "[1 5]" {
		t.Errorf("hunks %v", h)
	}
}
