package workflowcheck

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var fuzzFunc = regexp.MustCompile(`(?m)^func (Fuzz\w+)\(f \*testing\.F\)`)

// TestEveryFuzzTargetRuns checks that each Fuzz function in the module is fuzzed by the nightly
// Fuzz workflow and by CI's short pass (#43), so a new target can't be left out of both.
func TestEveryFuzzTargetRuns(t *testing.T) {
	root := filepath.Join("..", "..")
	targets := map[string]string{} // target -> package directory, slash-separated
	walk := func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, "_test.go") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, filepath.Dir(path))
		for _, m := range fuzzFunc.FindAllStringSubmatch(string(data), -1) {
			targets[m[1]] = filepath.ToSlash(rel)
		}
		return nil
	}
	for _, dir := range []string{"cmd", "internal", "tools"} { // the module's Go code
		if err := filepath.WalkDir(filepath.Join(root, dir), walk); err != nil {
			t.Fatal(err)
		}
	}
	if len(targets) == 0 {
		t.Fatal("found no fuzz targets")
	}
	nightly, err := os.ReadFile(filepath.Join(root, ".github", "workflows", "fuzz.yml"))
	if err != nil {
		t.Fatal(err)
	}
	ci, err := os.ReadFile(filepath.Join(root, ".github", "workflows", "ci.yml"))
	if err != nil {
		t.Fatal(err)
	}
	for target, dir := range targets {
		if !strings.Contains(string(nightly), "{ package: ./"+dir+", target: "+target+" }") {
			t.Errorf("fuzz.yml doesn't fuzz %s in ./%s", target, dir)
		}
		if !strings.Contains(string(ci), strings.TrimPrefix(dir, "internal/")+":"+target) {
			t.Errorf("ci.yml's short fuzzing pass doesn't run %s in ./%s", target, dir)
		}
	}
}
