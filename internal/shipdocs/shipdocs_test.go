package shipdocs

import (
	"os"
	"path/filepath"
	"testing"
)

// TestEveryDocExists: each listed document is in the repository, where the packagers read it.
func TestEveryDocExists(t *testing.T) {
	for _, d := range List {
		if _, err := os.Stat(filepath.Join("..", "..", filepath.FromSlash(d.Repo))); err != nil {
			t.Errorf("%s: %v", d.Name, err)
		}
	}
}
