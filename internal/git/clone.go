package git

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// Clone uses the same executable resolution, process containment and noninteractive
// network policy as Fetch/Pull. An existing destination is never modified.
func Clone(ctx context.Context, remote, destination string) (*Repo, error) {
	if strings.TrimSpace(remote) == "" || strings.HasPrefix(remote, "-") {
		return nil, errors.New("enter a repository URL or path")
	}
	if !filepath.IsAbs(destination) {
		return nil, errors.New("clone destination must be absolute")
	}
	if _, err := os.Lstat(destination); !errors.Is(err, os.ErrNotExist) {
		if err == nil {
			err = errors.New("clone destination already exists")
		}
		return nil, err
	}
	r := &Repo{Root: filepath.Dir(destination)}
	// No automatic submodules; refuse the ext transport even if global config enables it.
	_, err := r.network(ctx, "clone", true, "-c", "protocol.ext.allow=never", "clone", "--no-recurse-submodules", "--", remote, destination)
	if err != nil {
		return nil, err
	}
	return Open(ctx, destination)
}
