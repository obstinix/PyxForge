// Package examples holds the example projects. New Project offers them as presets, so the
// files a new project starts from are the ones this folder shows and CI builds.
package examples

import "embed"

// BootSector is examples/boot-sector: a BIOS boot sector in NASM that builds and boots in QEMU.
//
//go:embed boot-sector/boot.asm boot-sector/pyxforge.toml boot-sector/.gitignore boot-sector/README.md
var BootSector embed.FS
