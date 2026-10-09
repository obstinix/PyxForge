package icons

import (
	"image/color"
	"strings"
	"testing"
)

var all = []Name{
	Binary, Bot, Bug, Check, ChevronDown, ChevronRight, Circle, CircleCheck, CircleX, Command, CPU,
	Ellipsis, File, FileCode, FileText, Flag, Folder, FolderOpen, FolderTree, GitBranch, Hammer, Hash,
	Info, List, ListTree, MemoryStick, Palette, PanelBottom, PanelLeft, PanelRight, Play, Plus, FileDiff, GitCommit, Minus,
	Refresh, Search, Server, Settings, Square, SunMoon, Terminal, TriangleAlert, X,
}

func TestExplicitArcs(t *testing.T) {
	cases := map[string]string{
		// Two arcs after one command become two commands.
		"M9 4a2 2 0 0 1 4 0 2 2 0 0 0 3 1": "M9 4a2 2 0 0 1 4 0a2 2 0 0 0 3 1",
		// Compact numbers are split; other commands keep implicit repetition.
		"M1-2.5.5L3 4 5 6": "M1 -2.5 .5L3 4 5 6",
		"A1 1 0 0 1 2 2":   "A1 1 0 0 1 2 2",
	}
	for in, want := range cases {
		if got := explicitArcPath(in); got != want {
			t.Errorf("explicitArcPath(%q) = %q, want %q", in, got, want)
		}
	}
	for _, n := range all {
		src, _ := files.ReadFile("svg/" + string(n) + ".svg")
		for _, m := range pathData.FindAllStringSubmatch(explicitArcs(string(src)), -1) {
			if strings.ContainsAny(m[1], "aA") && arcNumbersAfterCommand(m[1]) > 7 {
				t.Errorf("%s still has an implicit arc: %s", n, m[1])
			}
		}
	}
}

// arcNumbersAfterCommand is the largest count of numbers following a single arc command.
func arcNumbersAfterCommand(d string) int {
	most, n, inArc := 0, 0, false
	for _, t := range pathToken.FindAllString(d, -1) {
		if c := t[0]; (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') {
			inArc, n = t == "a" || t == "A", 0
			continue
		}
		if inArc {
			n++
			most = max(most, n)
		}
	}
	return most
}

func TestEveryIconLoads(t *testing.T) {
	entries, err := files.ReadDir("svg")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != len(all) {
		t.Errorf("%d SVG files bundled, %d constants declared", len(entries), len(all))
	}
	for _, n := range all {
		svg := string(Get(n, color.NRGBA{R: 0x12, G: 0x34, B: 0x56, A: 0xff}).Content())
		if strings.Contains(svg, "currentColor") || !strings.Contains(svg, "#123456") {
			t.Errorf("%s was not recoloured", n)
		}
	}
}

func TestTranslucentColourKeepsStrokeOpacity(t *testing.T) {
	svg := string(Get(X, color.NRGBA{R: 0xff, A: 0x80}).Content())
	if !strings.Contains(svg, `stroke-opacity="0.502"`) {
		t.Errorf("translucent colour lost its alpha:\n%s", svg)
	}
}

func TestCacheReturnsSameResource(t *testing.T) {
	c := color.NRGBA{R: 1, G: 2, B: 3, A: 0xff}
	if Get(Folder, c) != Get(Folder, c) {
		t.Error("same name and colour produced two resources")
	}
}
