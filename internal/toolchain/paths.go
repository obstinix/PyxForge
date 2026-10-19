package toolchain

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
)

// Where a tool was found, from first to last tried.
const (
	SourceConfigured = "set in Doctor"  // a path the user chose (Overrides)
	SourceBundled    = "bundled"        // shipped beside PyxForge: Neovim in the Windows installer
	SourcePath       = "PATH"           // found on PATH
	SourceFolder     = "install folder" // a folder its installer uses, such as QEMU's on Windows
)

// Overrides are executable paths the user set for tools, by tool ID. They are kept per user,
// beside PyxForge's other settings, never in a project: a project cannot choose the programs
// PyxForge runs.
type Overrides map[string]string

var (
	overridesMu sync.RWMutex
	current     Overrides
	// bundleDir is the folder holding the running executable; tools ship in folders under it.
	bundleDir = sync.OnceValue(func() string {
		exe, err := os.Executable()
		if err != nil {
			return ""
		}
		if real, err := filepath.EvalSymlinks(exe); err == nil {
			exe = real
		}
		return filepath.Dir(exe)
	})
)

// OverridesFile is where Overrides are saved: PyxForge/tools.json in the user configuration
// folder (%APPDATA% on Windows), which an installer's upgrade or removal does not touch.
func OverridesFile() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "PyxForge", "tools.json"), nil
}

// LoadOverrides reads the saved overrides and makes them the ones lookups use. A missing file
// means none.
func LoadOverrides() (Overrides, error) {
	file, err := OverridesFile()
	if err != nil {
		return nil, err
	}
	o := Overrides{}
	data, err := os.ReadFile(file)
	switch {
	case errors.Is(err, fs.ErrNotExist):
	case err != nil:
		return nil, err
	default:
		if err := json.Unmarshal(data, &o); err != nil {
			return nil, fmt.Errorf("%s: %w", file, err)
		}
	}
	SetOverrides(o)
	return o, nil
}

// SetOverrides makes o the overrides lookups use, without saving them.
func SetOverrides(o Overrides) {
	clean := Overrides{}
	for id, p := range o {
		if p != "" {
			clean[id] = p
		}
	}
	overridesMu.Lock()
	current = clean
	overridesMu.Unlock()
}

// CurrentOverrides returns a copy of the overrides in use.
func CurrentOverrides() Overrides {
	overridesMu.RLock()
	defer overridesMu.RUnlock()
	o := Overrides{}
	for k, v := range current {
		o[k] = v
	}
	return o
}

// SaveOverride sets (or, with an empty path, clears) one tool's path, saves the overrides and
// uses them from now on. A path must be absolute.
func SaveOverride(id, path string) error {
	if path != "" && !filepath.IsAbs(path) {
		return fmt.Errorf("%q is not an absolute path", path)
	}
	o := CurrentOverrides()
	if path == "" {
		delete(o, id)
	} else {
		o[id] = filepath.Clean(path)
	}
	file, err := OverridesFile()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(o, "", "  ")
	if err != nil {
		return err
	}
	tmp := file + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, file); err != nil {
		os.Remove(tmp)
		return err
	}
	SetOverrides(o)
	return nil
}
