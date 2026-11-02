// Package templates makes new projects from presets (#72): an empty folder, the boot-sector
// example, or a C program, each with a pyxforge.toml that builds as it is.
package templates

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"

	"github.com/obstinix/PyxForge/examples"
	"github.com/obstinix/PyxForge/internal/config"
)

// Preset is a kind of project New Project can start.
type Preset struct {
	ID, Name, Description string
	files                 fs.FS // the project's files at its root; nil for an empty folder
}

// Presets are offered in this order; the first is the default.
var Presets = []Preset{
	{ID: "empty", Name: "Empty folder", Description: "A folder to fill yourself."},
	{ID: "boot-sector", Name: "Boot sector (NASM)", Description: "A BIOS boot sector that prints a line and halts: Build, then Run or Debug in QEMU.",
		files: sub(examples.BootSector, "boot-sector")},
	{ID: "c-program", Name: "C program", Description: "hello.c with a build profile for GCC: Build, then Run or Debug Current File.",
		files: cProgram},
}

// ByID finds a preset.
func ByID(id string) (Preset, bool) {
	for _, p := range Presets {
		if p.ID == id {
			return p, true
		}
	}
	return Preset{}, false
}

func sub(f fs.FS, dir string) fs.FS {
	s, err := fs.Sub(f, dir)
	if err != nil {
		panic(err) // the embedded folder is named in this file
	}
	return s
}

var projectName = regexp.MustCompile(`(?m)^name = "[^"]*"`)

// Create makes the project dir, which must not exist yet, from p. Its pyxforge.toml is given the
// project's name.
func Create(dir, name string, p Preset) error {
	if err := os.Mkdir(dir, 0o755); err != nil {
		return err
	}
	if p.files == nil {
		return nil
	}
	quoted, err := config.TOMLString(name)
	if err != nil {
		return err
	}
	return fs.WalkDir(p.files, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || path == "." {
			return err
		}
		target := filepath.Join(dir, filepath.FromSlash(path))
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		b, err := fs.ReadFile(p.files, path)
		if err != nil {
			return err
		}
		if path == config.FileName {
			if !projectName.Match(b) {
				return errors.New("the preset's pyxforge.toml has no project name")
			}
			b = projectName.ReplaceAll(b, []byte("name = "+quoted))
		}
		if err := os.WriteFile(target, b, 0o644); err != nil {
			return fmt.Errorf("writing %s: %w", path, err)
		}
		return nil
	})
}

//go:embed c-program/hello.c c-program/pyxforge.toml c-program/.gitignore
var cProgramFiles embed.FS

var cProgram = sub(cProgramFiles, "c-program")
