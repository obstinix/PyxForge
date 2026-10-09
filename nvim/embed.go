// Package nvim carries PyxForge's Neovim configuration (init.lua and lua/pyxforge) and its
// plugin lockfile inside the binary, and installs them where PyxForge's Neovim reads them.
//
// The files go to a folder PyxForge owns, named by their content hash, so a new PyxForge
// version never edits files an older one wrote, and the user's own Neovim configuration
// (~/.config/nvim, %LOCALAPPDATA%\nvim) is never touched.
package nvim

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
)

//go:embed init.lua lua pyxforge-lock.json
var files embed.FS

// AppName is the NVIM_APPNAME PyxForge's Neovim runs under: its stdpath("config"),
// stdpath("data") and stdpath("state") are separate from the user's own Neovim.
const AppName = "pyxforge"

// Hash identifies this version of the configuration.
func Hash() string {
	h := sha256.New()
	var names []string
	_ = fs.WalkDir(files, ".", func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			names = append(names, p)
		}
		return nil
	})
	sort.Strings(names)
	for _, n := range names {
		b, _ := files.ReadFile(n)
		h.Write([]byte(n))
		h.Write([]byte{0})
		h.Write(b)
	}
	return hex.EncodeToString(h.Sum(nil))[:16]
}

// Install writes the configuration under base, in a folder named by Hash, unless it is already
// there, and returns the path of its init.lua. Existing files are never overwritten.
func Install(base string) (string, error) {
	dir := filepath.Join(base, Hash())
	initPath := filepath.Join(dir, "init.lua")
	if _, err := os.Stat(initPath); err == nil {
		return initPath, nil
	}
	if err := os.MkdirAll(base, 0o755); err != nil {
		return "", err
	}
	tmp, err := os.MkdirTemp(base, ".tmp-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmp)
	err = fs.WalkDir(files, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		target := filepath.Join(tmp, filepath.FromSlash(p))
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		b, err := files.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(target, b, 0o644)
	})
	if err != nil {
		return "", err
	}
	// Publish atomically; if another PyxForge got there first, keep its copy.
	if err := os.Rename(tmp, dir); err != nil {
		if _, statErr := os.Stat(initPath); statErr == nil {
			return initPath, nil
		}
		return "", err
	}
	return initPath, nil
}

// DefaultBase is where Install puts configurations: PyxForge's folder in the user cache.
func DefaultBase() (string, error) {
	d, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "PyxForge", "nvim-runtime"), nil
}

// Plugin is one pinned plugin from the lockfile.
type Plugin struct {
	Name string `json:"name"`
	URL  string `json:"url"`
	Rev  string `json:"rev"`
}

// Lockfile is pyxforge-lock.json: the plugins `pyxforge setup editor` installs, each pinned to
// a commit, and the Tree-sitter parsers it builds.
type Lockfile struct {
	Plugins []Plugin `json:"plugins"`
	Parsers []string `json:"parsers"`
}

// Lock reads the embedded lockfile.
func Lock() (Lockfile, error) {
	var l Lockfile
	b, err := files.ReadFile("pyxforge-lock.json")
	if err != nil {
		return l, err
	}
	if err := json.Unmarshal(b, &l); err != nil {
		return l, err
	}
	for _, p := range l.Plugins {
		if p.Name == "" || p.URL == "" || len(p.Rev) != 40 {
			return l, errors.New("pyxforge-lock.json: every plugin needs a name, a URL and a full 40-character commit")
		}
	}
	return l, nil
}
