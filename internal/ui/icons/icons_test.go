package icons

import (
	"image/color"
	"strings"
	"testing"
)

var all = []Name{
	Binary, Bot, Bug, Check, ChevronDown, ChevronRight, Circle, CircleCheck, CircleX, Command, CPU,
	File, FileCode, FileText, Flag, Folder, FolderOpen, FolderTree, GitBranch, Hammer, Hash, Info,
	List, ListTree, MemoryStick, Palette, PanelBottom, PanelLeft, PanelRight, Play, Plus, Search,
	Server, Settings, Square, SunMoon, Terminal, TriangleAlert, X,
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
