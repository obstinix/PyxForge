package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// TestIconsAreCurrent: the icon files in the repository are what icon.svg renders to, so the
// SVG stays the one source. After editing it, run go run ./tools/appicon.
func TestIconsAreCurrent(t *testing.T) {
	t.Chdir("../..")
	files, err := render(source)
	if err != nil {
		t.Fatal(err)
	}
	for path, want := range files {
		got, err := os.ReadFile(filepath.FromSlash(path))
		if err != nil || !bytes.Equal(got, want) {
			t.Errorf("%s is out of date (%v): run go run ./tools/appicon", path, err)
		}
	}
	ico := files[winICO]
	if len(ico) < 6 || ico[2] != 1 || int(ico[4]) != len(icoSizes) {
		t.Errorf("not an icon with %d images: % x", len(icoSizes), ico[:6])
	}
}
