//go:build !windows

package trash

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
)

func move(abs string) error {
	dir, err := home()
	if err != nil {
		return err
	}
	return toTrash(dir, abs)
}

func isCrossDevice(err error) bool { return errors.Is(err, syscall.EXDEV) }

// home is the user's trash folder under the freedesktop.org specification:
// $XDG_DATA_HOME/Trash, or ~/.local/share/Trash.
func home() (string, error) {
	if d := os.Getenv("XDG_DATA_HOME"); filepath.IsAbs(d) {
		return filepath.Join(d, "Trash"), nil
	}
	h, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(h, ".local", "share", "Trash"), nil
}
