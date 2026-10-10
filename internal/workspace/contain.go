package workspace

import (
	"os"
	"path/filepath"
	"strings"
)

// Within reports whether p is root or under it, comparing the paths as written.
func Within(root, p string) bool {
	r, err := filepath.Rel(root, p)
	return err == nil && !filepath.IsAbs(r) && r != ".." && !strings.HasPrefix(r, ".."+string(filepath.Separator))
}

// Contains reports whether p stays under root both as written and once the symbolic links in
// the part of p that exists are followed, so nothing reaches outside the workspace through a
// linked folder. p need not exist.
func Contains(root, p string) bool {
	if !Within(root, p) {
		return false
	}
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return false
	}
	existing := p
	for {
		if _, err := os.Lstat(existing); err == nil {
			break
		}
		parent := filepath.Dir(existing)
		if parent == existing {
			return false
		}
		existing = parent
	}
	real, err := filepath.EvalSymlinks(existing)
	return err == nil && Within(realRoot, real)
}
