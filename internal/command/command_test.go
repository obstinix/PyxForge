package command

import "testing"

func TestMatch(t *testing.T) {
	if _, ok := Match("xyz", "Change Theme"); ok {
		t.Error("non-subsequence matched")
	}
	if _, ok := Match("THM", "Change Theme"); !ok {
		t.Error("match should ignore case")
	}
	better := func(q, a, b string) {
		t.Helper()
		sa, oka := Match(q, a)
		sb, okb := Match(q, b)
		if !oka || !okb || sa <= sb {
			t.Errorf("Match(%q): %q scored %d, %q scored %d; want the first higher", q, a, sa, b, sb)
		}
	}
	better("ct", "Change Theme", "Select Accent")      // word starts beat a run inside a word
	better("set", "Settings", "Reset Layout")          // prefix beats a match inside a word
	better("insp", "Toggle Inspector", "Insert Space") // a longer consecutive run wins
	better("dock", "Toggle Dock", "Open Documentation Lock")
}

func TestSearch(t *testing.T) {
	var r Registry
	noop := func() {}
	for _, c := range []Command{
		{ID: "palette", Title: "Show All Commands", Category: "View", Run: noop},
		{ID: "theme", Title: "Change Theme", Category: "Preferences", Run: noop},
		{ID: "accent", Title: "Change Accent", Category: "Preferences", Run: noop},
		{ID: "inspector", Title: "Toggle Inspector", Category: "View", Run: noop},
		{ID: "explorer", Title: "Toggle Explorer", Category: "View", Run: noop},
	} {
		r.Add(c)
	}
	if got := r.Search(""); len(got) != 5 || got[0].ID != "palette" {
		t.Errorf("empty query should list everything in order, got %v", ids(got))
	}
	if got := r.Search("insp"); len(got) == 0 || got[0].ID != "inspector" {
		t.Errorf("insp: got %v", ids(got))
	}
	if got := r.Search("pref acc"); len(got) == 0 || got[0].ID != "accent" {
		t.Errorf("category match: got %v", ids(got))
	}
	// A title match beats a match that needs the category: "theme" finds the chooser, not
	// the first theme it would apply.
	r.Add(Command{ID: "theme.system", Title: "System", Category: "Theme", Run: noop})
	r.Add(Command{ID: "theme.mono", Title: "Monochrome", Category: "Theme", Run: noop})
	if got := r.Search("theme"); len(got) < 3 || got[0].ID != "theme" {
		t.Errorf("theme: got %v, want the Change Theme command first", ids(got))
	}
	if got := r.Search("theme mono"); len(got) == 0 || got[0].ID != "theme.mono" {
		t.Errorf("theme mono: got %v", ids(got))
	}
	if got := r.Search("zzz"); len(got) != 0 {
		t.Errorf("zzz: got %v", ids(got))
	}
	ran := false
	r.Add(Command{ID: "x", Title: "X", Run: func() { ran = true }})
	if !r.Run("x") || !ran || r.Run("missing") {
		t.Error("Run by ID")
	}
}

func TestPositions(t *testing.T) {
	got := Positions("ct", "Change Theme")
	if len(got) != 2 || got[0] != 0 || got[1] != 7 {
		t.Errorf("Positions(ct) = %v, want [0 7]", got)
	}
	if Positions("xyz", "Change Theme") != nil {
		t.Error("Positions of a non-match should be nil")
	}
}

func TestGet(t *testing.T) {
	var r Registry
	r.Add(Command{ID: "go.file", Title: "Go to File", Keys: "Ctrl+Shift+O", Run: func() {}})
	if c, ok := r.Get("go.file"); !ok || c.Keys != "Ctrl+Shift+O" {
		t.Errorf("Get(go.file) = %v, %v", c, ok)
	}
	if _, ok := r.Get("missing"); ok {
		t.Error("Get found a command that was never added")
	}
}

func TestAddRejectsDuplicates(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("duplicate ID did not panic")
		}
	}()
	var r Registry
	r.Add(Command{ID: "a", Title: "A", Run: func() {}})
	r.Add(Command{ID: "a", Title: "A again", Run: func() {}})
}

func ids(cs []Command) []string {
	out := make([]string, len(cs))
	for i, c := range cs {
		out[i] = c.ID
	}
	return out
}
