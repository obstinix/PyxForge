package shell

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/obstinix/PyxForge/internal/shipdocs"
)

// TestHelpOpensShippedDocs: Help > Documentation… lists every shipped document, found on disk
// (here in the source checkout, where the packagers read them), and opens the chosen one in the
// editor; nothing is fetched (D22).
func TestHelpOpensShippedDocs(t *testing.T) {
	s, _, _ := newEditorShell(t)
	for _, d := range shipdocs.List {
		if docPath(d.Name) == "" {
			t.Errorf("%s is not found", d.Name)
		}
	}
	if !s.Commands().Run("help.docs") || !s.palette.Visible() {
		t.Fatal("Help > Documentation… did not list the documents")
	}
	items := s.palette.Results()
	if len(items) != len(shipdocs.List) {
		t.Fatalf("%d documents listed", len(items))
	}
	for _, it := range items {
		if strings.Contains(it.Detail, "not installed") {
			t.Errorf("%s: %s", it.Title, it.Detail)
		}
		if it.Title == "Keyboard shortcuts" {
			it.Run()
		}
	}
	s.palette.Hide()
	if d := s.activeDoc(); d == nil || filepath.Base(d.path) != "KEYMAP.md" {
		t.Fatal("choosing Keyboard shortcuts did not open KEYMAP.md")
	}
}
