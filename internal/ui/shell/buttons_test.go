package shell

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestShellUsesKitButtons: every button in the shell is a kit.Button (identity prompt, Phase
// 1), so they share one look, one focus ring and Return/Enter activation. Fyne's widget.Button
// may not come back: neither constructed nor named as a type.
func TestShellUsesKitButtons(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	forbidden := regexp.MustCompile(`widget\.NewButton|widget\.Button\b`)
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for i, line := range strings.Split(string(data), "\n") {
			if forbidden.MatchString(line) {
				t.Errorf("%s:%d uses Fyne's button: %s", f, i+1, strings.TrimSpace(line))
			}
		}
	}
}
