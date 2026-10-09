# PyxForge

[![CI](https://github.com/obstinix/PyxForge/actions/workflows/ci.yml/badge.svg)](https://github.com/obstinix/PyxForge/actions/workflows/ci.yml)
[![License: Apache 2.0](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)

PyxForge is a native desktop environment for bootloader, kernel and bare-metal development. It
brings the editor, build, QEMU, GDB and binary inspection that an operating-system developer
juggles into one keyboard-first workspace that works offline.

PyxForge 3.0 is a rewrite in **Go and Fyne**, with **real Neovim** as the editor. It replaces the
2.x Tauri desktop app and VS Code extension, which remain in [`legacy/`](legacy/) until 3.0
reaches feature parity.

## Status

**Pre-alpha.** The native shell and design system are built. The tools inside it are being
built now, in this order: Neovim editor, core tools (config, build, diagnostics, terminal, Git),
systems tools (QEMU, GDB, binary inspection), then agents.

| Area | State |
|---|---|
| Workbench: rail, file explorer, editor tabs, bottom panel, inspector, status bar | Working; panels for unbuilt tools say what arrives and when |
| Command palette, Go to File, keyboard navigation | Working |
| Command line: `help`, `version`, `doctor` (toolchain check), `info` (project and `pyxforge.toml`) | Working |
| `pyxforge.toml` loading and validation, compatible with 2.x ([reference](docs/reference/pyxforge-toml.md)) | Working |
| Five themes (Smoked Kraft, Ink & Paper, Ink & Glass, Verdigris Forge, Monochrome), Crimson and Amber accents, System mode, optional glass overlays | Working |
| Editing and saving files (Neovim) | Next |
| Build, diagnostics, terminal, Git | Planned |
| QEMU, QMP, GDB, registers, memory, hex, ELF | Planned |
| Agents in isolated worktrees | Planned |

Progress against every 2.x feature is tracked in
[`docs/architecture/FEATURE_PARITY.md`](docs/architecture/FEATURE_PARITY.md).

## Build and run

Requirements: Go 1.27.1 or newer and a C compiler for cgo (Fyne draws with OpenGL). Platform
details are in [`docs/development/SETUP.md`](docs/development/SETUP.md).

```sh
git clone https://github.com/obstinix/PyxForge.git
cd PyxForge
go run ./cmd/pyxforge            # opens the current folder
go run ./cmd/pyxforge path/to/project
```

PyxForge makes no network request at startup and needs no account.

## Command line

The same binary is a command-line tool. It shares its tool detection and project discovery
with the desktop app.

```sh
pyxforge help                 # commands and options
pyxforge version              # release, Go and Fyne versions, source revision
pyxforge doctor               # which tools PyxForge drives are installed, and how to get the rest
pyxforge info [folder]        # project root, pyxforge.toml and Git checkout of a folder
pyxforge doctor --json        # machine-readable output, for scripts and CI
```

Exit status is 0 on success, 1 when a command finds a problem (for example a required tool is
missing), and 2 for a wrong command line. Ctrl+C stops a command and the tools it started.
Build, run and debug commands arrive with the build service and QEMU integration.

## Keyboard

Neovim owns every key while the editor has focus. The shell binds only Ctrl+Shift chords
(Cmd+Shift on macOS):

| Chord | Command |
|---|---|
| Ctrl+Shift+P | Show all commands |
| Ctrl+Shift+O | Go to file |
| Ctrl+Shift+E / J / I | Toggle explorer / panel / inspector |
| Ctrl+Shift+W | Close editor tab |
| Ctrl+Shift+PageDown / PageUp | Next / previous tab |
| Ctrl+Shift+, | Settings |

Every command is also in the command palette. The full policy is in
[`docs/architecture/KEYMAP.md`](docs/architecture/KEYMAP.md).

## Repository layout

| Path | Contents |
|---|---|
| `cmd/pyxforge` | The application entry point: desktop app and command line |
| `internal/cli` | Command-line commands |
| `internal/toolchain` | Detection of Neovim, assemblers, compilers, linkers, QEMU, GDB and Git |
| `internal/workspace` | Project and Git checkout discovery |
| `internal/config` | `pyxforge.toml` parsing and validation |
| `internal/buildinfo` | Version and build information |
| `internal/command` | Command registry and fuzzy matching |
| `internal/ui` | Theme tokens, design-system widgets, workbench shell, palette, explorer, notifications, icons |
| `tools/forbidcheck` | Fails the build if web technology enters the application |
| `tools/snapshot` | Renders the real shell to PNG for design reviews |
| `docs/` | Architecture, decisions, design system and component specs |
| `legacy/` | The 2.x Rust core, VS Code extension and Tauri app, kept until parity |

## Development checks

CI runs these on Linux and Windows, and builds on macOS:

```sh
go run ./tools/forbidcheck
gofmt -l .
go vet ./...
staticcheck ./...
go test -race ./...
```

Contributing guidelines: [`CONTRIBUTING.md`](CONTRIBUTING.md). Design system:
[`docs/design/DESIGN_SYSTEM.md`](docs/design/DESIGN_SYSTEM.md).

## The 2.x stack

The Rust core (`legacy/core`) is the reference for porting behaviour: its 59 tests are the
parity oracle for the Go implementation. Build and test it with `cargo test` in `legacy/core`.
The VS Code extension builds with `npm ci && npm run compile` in `legacy/extension`.

## License

Apache License 2.0. See [`LICENSE`](LICENSE). Bundled fonts and icons keep their own licenses,
listed in [`docs/design/FONT_LICENSES.md`](docs/design/FONT_LICENSES.md).
