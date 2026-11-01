package git

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// CloneTimeout bounds a clone while leaving large repositories more time than fetch/push.
const CloneTimeout = 30 * time.Minute

// Clone uses the same executable resolution, process containment and noninteractive
// network policy as Fetch/Pull. An existing destination is never modified.
func Clone(ctx context.Context, remote, destination string) (*Repo, error) {
	return CloneWithProgress(ctx, remote, destination, nil)
}

// CloneWithProgress reports Git's progress output while cloning.
func CloneWithProgress(ctx context.Context, remote, destination string, progress func(string)) (*Repo, error) {
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, CloneTimeout)
		defer cancel()
	}
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
	if err := os.Mkdir(destination, 0o755); err != nil {
		return nil, err
	}
	r := &Repo{Root: filepath.Dir(destination)}
	// No automatic submodules; refuse the ext transport even if global config enables it.
	_, err := r.networkWithProgress(ctx, "clone", true, progress, "-c", "protocol.ext.allow=never",
		"clone", "--progress", "--no-recurse-submodules", "--", remote, destination)
	if err != nil {
		if cleanupErr := os.RemoveAll(destination); cleanupErr != nil {
			return nil, fmt.Errorf("%w (could not remove partial clone %s: %v)", err, destination, cleanupErr)
		}
		return nil, r.explain([]string{"clone"}, err)
	}
	repo, err := Open(ctx, destination)
	if err != nil {
		if cleanupErr := os.RemoveAll(destination); cleanupErr != nil {
			return nil, fmt.Errorf("%w (could not remove partial clone %s: %v)", err, destination, cleanupErr)
		}
		return nil, err
	}
	return repo, nil
}
