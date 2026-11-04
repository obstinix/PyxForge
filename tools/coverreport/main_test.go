package main

import (
	"strings"
	"testing"
)

const profile = `mode: atomic
example.com/m/a/a.go:3.10,5.2 2 1
example.com/m/a/a.go:7.10,9.2 3 0
example.com/m/b/b.go:1.1,2.2 4 0
example.com/m/a/a.go:7.10,9.2 3 5
example.com/m/b/b.go:1.1,2.2 4 0
example.com/m/c.go:1.1,1.9 1 1
`

func TestParseMergesBlocksAcrossTestBinaries(t *testing.T) {
	pkgs, err := parse(strings.NewReader(profile))
	if err != nil {
		t.Fatal(err)
	}
	got := map[string][2]int{}
	for _, p := range pkgs {
		got[p.name] = [2]int{p.covered, p.stmts}
	}
	// a.go's second block is uncovered in one binary and covered in another: covered.
	want := map[string][2]int{
		"example.com/m/a": {5, 5},
		"example.com/m/b": {0, 4},
		"example.com/m":   {1, 1},
	}
	for name, w := range want {
		if got[name] != w {
			t.Errorf("%s: covered/statements = %v, want %v", name, got[name], w)
		}
	}
	if pkgs[0].name != "example.com/m/b" {
		t.Errorf("the least covered package is not first: %v", pkgs)
	}
}

func TestWriteTrimsTheModuleAndTotals(t *testing.T) {
	pkgs, err := parse(strings.NewReader(profile))
	if err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	write(&b, pkgs, "example.com/m")
	out := b.String()
	for _, want := range []string{
		"**Statement coverage: 60.0%** (6 of 10 statements, 3 packages)",
		"| b | 0.0% | 0 / 4 |",
		"| a | 100.0% | 5 / 5 |",
		"| . | 100.0% | 1 / 1 |",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("the report lacks %q:\n%s", want, out)
		}
	}
}

func TestParseRejectsOtherFiles(t *testing.T) {
	for _, in := range []string{"hello\n", "mode: set\nnot a block\n", "mode: set\nf.go:1.1,2.2 x 1\n"} {
		if _, err := parse(strings.NewReader(in)); err == nil {
			t.Errorf("parse(%q) succeeded", in)
		}
	}
}
