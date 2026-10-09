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
| Explorer that follows changes on disk; New File, New Folder, Reveal Active File | Working |
| Each workspace reopens as you left it: files, tabs, panels, explorer folders, build profile | Working |
| Command line: `help`, `version`, `doctor` (toolchain check), `info` (project and `pyxforge.toml`) | Working |
| `pyxforge.toml` loading and validation, compatible with 2.x ([reference](docs/reference/pyxforge-toml.md)) | Working |
| Five themes (Smoked Kraft, Ink & Paper, Ink & Glass, Verdigris Forge, Monochrome), Crimson and Amber accents, System mode, optional glass overlays | Working |
| Editing in an embedded Neovim: buffers as tabs, unsaved markers, Ctrl+S, close prompts, system clipboard | Working (needs Neovim 0.9+; clipboard 0.10+) |
| Language servers (clangd, gopls, rust-analyzer, asm-lsp…) and Tree-sitter highlighting; diagnostics in Problems | Working |
| Terminal panel: your shell in the project folder, theme colours, exit status, restart | Working (one session) |
| Build: `pyxforge.toml` profiles with dependencies, streamed output, Stop, errors in Problems (`pyxforge build` and the Build tab) | Working |
| Git: branch, changed files, diff against HEAD in the editor, stage, unstage, commit | Working |
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
go run ./cmd/pyxforge path/to/boot.asm   # the file's project, with the file open
```

PyxForge makes no network request at startup and needs no account. It remembers each
workspace's layout and open files in your configuration folder (`PyxForge/workspaces`), never
in the project.

## Command line

The same binary is a command-line tool. It shares its tool detection and project discovery
with the desktop app.

```sh
pyxforge help                 # commands and options
pyxforge version              # release, Go and Fyne versions, source revision
pyxforge doctor               # which tools PyxForge drives are installed, and how to get the rest
pyxforge info [folder]        # project root, pyxforge.toml and Git checkout of a folder
pyxforge build [profile]      # run build profiles, dependencies first (--list shows them)
pyxforge setup editor         # install the editor's pinned plugins and parsers (once, online)
pyxforge doctor --json        # machine-readable output, for scripts and CI
```

Exit status is 0 on success, 1 when a command finds a problem (for example a required tool is
missing), and 2 for a wrong command line. Ctrl+C stops a command and the tools it started.
Run and debug commands arrive with the QEMU integration.

## The editor

Files open in an embedded Neovim that runs PyxForge's own configuration (`nvim/`). It never
reads or changes your Neovim setup: PyxForge's Neovim keeps its settings, plugins and history in
its own folders (`NVIM_APPNAME=pyxforge`). Put personal additions in `user.lua` in that config
folder (`:echo stdpath("config")` in the editor shows where).

Language servers start on their own when installed; `pyxforge doctor` lists them. Neovim ships
Tree-sitter parsers for C, Lua and Markdown. For assembly, Rust, Go, linker scripts and more, run
once (needs Git, the network, a C compiler and the tree-sitter CLI):

```sh
pyxforge setup editor
```

It installs the plugin and parsers pinned in `nvim/pyxforge-lock.json`; after that the editor
needs no network.

## Building

Builds run the profiles in the project's `pyxforge.toml`
([reference](docs/reference/pyxforge-toml.md)): each profile names a tool and its arguments,
and `depends_on` orders them. `pyxforge build` with no profile builds the profiles nothing else
depends on. In the desktop app, the Build tab (Ctrl+Shift+B) saves open files, runs the chosen
profile, streams its output and lists the errors and warnings it prints (GCC, Clang, NASM, ld
and Cargo formats) in Problems, where selecting one opens the file at that line.

[`examples/boot-sector`](examples/boot-sector) is a complete 512-byte BIOS boot sector to try it
on.

## Git

The Git tab lists what the checkout has changed, grouped as Git groups it (conflicts, staged,
changes, untracked), with the branch and how far it is ahead of or behind its upstream; the status
bar shows the branch and the count. Selecting a file opens it beside its committed text in
Neovim's diff mode (`:q` in the left window closes the diff). The + and − buttons stage and
unstage, and Commit records the staged files with your Git identity. PyxForge runs only local
Git commands; push and pull stay in the Terminal.

## The terminal

The Terminal tab in the bottom panel runs your shell (Neovim's `'shell'`: `cmd.exe` on Windows,
`$SHELL` elsewhere) in the project folder. It is a terminal inside a second embedded Neovim, so
it uses ConPTY on Windows and a pseudo-terminal elsewhere, and takes the theme's colours. Shell
chords (Ctrl+Shift+…) still reach PyxForge; press Esc twice to scroll and copy in Neovim's
normal mode, and `i` to type again. When the shell exits the panel shows its exit status; Restart
(or **Terminal: Restart Terminal** in the palette) starts a new one. To use another shell, set
`vim.o.shell` in `user.lua`.

## Keyboard

Neovim owns every key while the editor has focus. The shell binds only Ctrl+Shift chords
(Cmd+Shift on macOS):

| Chord | Command |
|---|---|
| Ctrl+Shift+P | Show all commands |
| Ctrl+Shift+O | Go to file |
| Ctrl+Shift+E / J / I | Toggle explorer / panel / inspector |
| Ctrl+Shift+W | Close editor tab |
| Ctrl+Shift+S (and Ctrl+S in the editor) | Save |
| Ctrl+Shift+B | Build |
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
| `internal/build` | Build profiles: ordering, running tools, parsing their diagnostics |
| `internal/git` | Local Git: status, stage, unstage, commit, file text at a revision |
| `internal/neovim` | The embedded Neovim: process, RPC, screen grid, buffer events |
| `internal/ui/editor` | The editor view: draws Neovim's grid, maps keys and mouse |
| `internal/buildinfo` | Version and build information |
| `internal/command` | Command registry and fuzzy matching |
| `internal/ui` | Theme tokens, design-system widgets, workbench shell, palette, explorer, notifications, icons |
| `tools/forbidcheck` | Fails the build if web technology enters the application |
| `tools/snapshot` | Renders the real shell to PNG for design reviews |
| `tools/nvimspike` | The editor feasibility harness behind ADR 0006 |
| `nvim/` | PyxForge's Neovim configuration (Lua) and plugin lockfile, embedded in the binary |
| `examples/` | Example projects (a BIOS boot sector) |
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
