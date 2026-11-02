// Package docs holds PyxForge's documentation; its test keeps FEATURES.md honest.
package docs

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestFeaturesNameRealTests: every test FEATURES.md gives as evidence exists in the
// repository, so a renamed or removed test cannot leave a feature's evidence pointing nowhere.
func TestFeaturesNameRealTests(t *testing.T) {
	doc, err := os.ReadFile("FEATURES.md")
	if err != nil {
		t.Fatal(err)
	}
	defined := map[string]bool{}
	decl := regexp.MustCompile(`(?m)^func (Test\w+)\(`)
	err = filepath.WalkDir("..", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && (d.Name() == "legacy" || d.Name() == "node_modules" || strings.HasPrefix(d.Name(), ".")) && p != ".." {
			return filepath.SkipDir
		}
		if !d.IsDir() && strings.HasSuffix(p, "_test.go") {
			b, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			for _, m := range decl.FindAllStringSubmatch(string(b), -1) {
				defined[m[1]] = true
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	cited := regexp.MustCompile("`"+`(Test\w+)`+"`").FindAllStringSubmatch(string(doc), -1)
	if len(cited) < 20 {
		t.Fatalf("FEATURES.md cites only %d tests", len(cited))
	}
	for _, m := range cited {
		if !defined[m[1]] {
			t.Errorf("FEATURES.md cites %s, which no test file defines", m[1])
		}
	}
}
