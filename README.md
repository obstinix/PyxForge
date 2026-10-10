# PyxForge

[![CI](https://github.com/obstinix/PyxForge/actions/workflows/ci.yml/badge.svg)](https://github.com/obstinix/PyxForge/actions/workflows/ci.yml)
[![License: Apache 2.0](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)

PyxForge is a native desktop environment for bootloader, kernel and bare-metal development. It
brings the editor, build, QEMU, GDB and binary inspection that an operating-system developer
juggles into one keyboard-first workspace that works offline.

PyxForge 3.0 is written in **Go** with **Fyne** for the interface and embeds **real Neovim** as
its editor, configured in **Lua**. It replaces the 2.x Tauri desktop app and VS Code extension,
which remain in [`legacy/`](legacy/) until 3.0 reaches feature parity.

## Status

**Pre-release.** The editor, build, Git, terminal, and QEMU and GDB debugging work and are tested
against the real tools on Windows and Linux. Agents are not started. What is verified, and how, is
in [`docs/architecture/CURRENT_STATE.md`](docs/architecture/CURRENT_STATE.md); what is left, in
the [roadmap](docs/ROADMAP.md).

| Area | What works |
|---|---|
| Workbench | Explorer that follows changes on disk (New File, New Folder, Reveal), editor tabs, bottom panel, inspector, status bar, command palette and Go to File; each workspace reopens as you left it |
| Editor | An embedded Neovim with PyxForge's own Lua configuration: buffers as tabs, unsaved markers, Ctrl+S, system clipboard, Tree-sitter, language servers (clangd, gopls, rust-analyzer, asm-lsp…) |
| Build | `pyxforge.toml` profiles with dependencies, streamed output, Stop; errors and warnings from GCC, Clang (text, SARIF, JSON), NASM, ld and Cargo in Problems and inside the editor |
| Terminal | Named sessions running your shell in the project folder |
| Git | Status, diff against HEAD, stage, commit, log, branches (create, switch, delete) and stash, all local |
| Run and debug | QEMU with its serial port and monitor, GDB attached at the program's first instruction, registers, flags, memory, real-mode disassembly, breakpoints, stepping |
| Binary inspection | Hex, boot-sector map and signature check, ELF headers, sections and symbols |
| Snapshots | Diagnostic snapshots compared between stops; QEMU machine states saved and restored |
| Themes | Smoked Kraft, Ink & Paper, Ink & Glass, Verdigris Forge, Monochrome; Crimson and Amber accents; System mode; optional glass overlays |
| Agents | Not started |

| Platform | Status |
|---|---|
| Windows 11, x86-64 | Supported and tested |
| Linux, x86-64 (Ubuntu 24.04) | Supported and tested (CI, WSL2) |
| macOS | Compiles in CI; never run, not supported yet |

## Install

[`docs/INSTALL.md`](docs/INSTALL.md) lists the tools PyxForge uses and the feature each enables,
with install commands for Windows and Ubuntu, and where PyxForge keeps its files. In short:

```sh
git clone https://github.com/obstinix/PyxForge.git
cd PyxForge
go build -o pyxforge ./cmd/pyxforge   # Go 1.27.1+ and a C compiler (Fyne uses OpenGL)
./pyxforge doctor                     # which tools were found; --commands prints install commands
./pyxforge path/to/project            # the desktop app
```

PyxForge makes no network request at startup, needs no account and sends no telemetry. Only
`pyxforge setup editor` uses the network, once, for the editor's pinned Tree-sitter parsers.

## Command line

The same binary is a command-line tool, sharing tool detection and project discovery with the app.

```sh
pyxforge help                    # commands and options
pyxforge version                 # release, Go and Fyne versions, source revision
pyxforge doctor [--commands]     # which tools are installed; the commands that install the rest
pyxforge info [folder]           # project root, pyxforge.toml and Git checkout of a folder
pyxforge build [profile]         # run build profiles, dependencies first (--list shows them)
pyxforge run [--debug]           # build, boot in QEMU, print the serial port (--debug waits for GDB)
pyxforge inspect FILE [--disasm] # boot-sector map and hex, or ELF sections and symbols
pyxforge setup editor            # once, online: the editor's pinned plugin and parsers
```

Most commands take `--json`. Exit status is 0 on success, 1 when a command finds a problem, and
2 for a wrong command line. Ctrl+C stops a command and every tool it started.

## The editor

Files open in an embedded Neovim that runs PyxForge's own configuration (`nvim/`). It never
reads or changes your Neovim setup: PyxForge's Neovim keeps its settings, plugins and history in
its own folders (`NVIM_APPNAME=pyxforge`). Put personal additions in `user.lua` in that config
folder (`:echo stdpath("config")` in the editor shows where).

Language servers start on their own when installed; their notices go to the Log. Build errors
appear in the editor as Neovim diagnostics, in a namespace of their own that the next build
replaces.

## Building

Builds run the profiles in the project's `pyxforge.toml`
([reference](docs/reference/pyxforge-toml.md)): each profile names a tool and its arguments, and
`depends_on` orders them. The Build tab (Ctrl+Shift+B) saves open files, runs the chosen profile,
streams its output, and lists errors and warnings in Problems (Ctrl+Shift+M), with Next and
Previous Problem (Ctrl+Shift+F8, F7) opening each at its line. Messages without a source line,
such as the linker's, are listed under the tool that printed them. Build steps stay inside the
project folder.

[`examples/boot-sector`](examples/boot-sector) is a complete 512-byte BIOS boot sector to try it
on; PyxisOS builds, boots and debugs too ([how](docs/cross-project/pyxisos-integration.md)).

## Running and debugging

Run in QEMU (Ctrl+Shift+R) builds, boots what `[qemu]` names, and shows QEMU's output (the
serial port, with `-serial stdio`) in the QEMU tab, whose prompt takes monitor commands such as
`info registers`. Debug in QEMU (Ctrl+Shift+D) starts QEMU paused, attaches GDB and stops at the
program's first instruction: 0x7c00 for a boot sector, the entry point of an ELF kernel. The
inspector then follows every stop:

| Tab | Shows |
|---|---|
| Registers | The registers of the current mode (real, protected, long), changes since the last stop in amber |
| Flags | EFLAGS bit by bit |
| Hex | The boot image's bytes, with the 55 aa check and the bytes used |
| Map | The 512-byte sector by region: code and data, padding, signature; FAT parameter blocks and partition tables when detected |
| Disasm | The code around the current instruction, decoded by PyxForge so real mode reads right |
| Memory | The stack, or any address |

Step Instruction (Ctrl+Shift+F11), Step Over Instruction (Ctrl+Shift+F10), Continue
(Ctrl+Shift+F5), Stop QEMU (Ctrl+Shift+F2) and Add Breakpoint… are in the palette and the QEMU
tab; the GDB tab takes any GDB command.

The Snapshots tab keeps two different things apart. A **diagnostic snapshot** records the
registers, flags, code, stack and memory of a paused machine, to compare with another stop or
with the machine now; it cannot restore anything. A **machine state** is QEMU's own snapshot of the
whole machine, which Restore brings back; it needs `snapshots = true` under `[qemu]` and
`qemu-img`.

## Git

The Git tab lists what the checkout has changed, grouped as Git groups it, with the branch and its
upstream; the status bar shows the branch and the count. Selecting a file opens it beside its
committed text in Neovim's diff mode. The branch button switches, creates and deletes branches:
switching with uncommitted changes asks first (switch, stash and switch, or cancel), and deleting
a branch whose commits exist nowhere else asks twice. Stashes are listed beside the commit box
with Apply, Pop and Drop. PyxForge runs only local Git commands; push and pull stay in the
Terminal.

## The terminal

The Terminal tab holds named sessions (New Terminal: Ctrl+Shift+`), each your shell (Neovim's
`'shell'`: `cmd.exe` on Windows, `$SHELL` elsewhere) in the project folder, with its own output.
Sessions are Neovim terminals, so they use ConPTY on Windows and a pseudo-terminal elsewhere and
take the theme's colours. Esc twice leaves terminal mode for scrolling and copying; `i` returns.
When a shell exits the session shows its status and Restart starts a new one. Processes started
in a session end with PyxForge.

## Keyboard

Neovim owns every key while the editor or a terminal has focus. The shell binds only Ctrl+Shift
chords (Cmd+Shift on macOS):

| Chord | Command |
|---|---|
| Ctrl+Shift+P / O | Show all commands / Go to file |
| Ctrl+Shift+S (and Ctrl+S in the editor) | Save |
| Ctrl+Shift+W | Close editor tab |
| Ctrl+Shift+E / J / I | Toggle explorer / panel / inspector |
| Ctrl+Shift+PageDown / PageUp | Next / previous tab |
| Ctrl+Shift+B | Build |
| Ctrl+Shift+M, F8, F7 | Show Problems, next / previous problem |
| Ctrl+Shift+R / D / F2 | Run / Debug / Stop QEMU |
| Ctrl+Shift+F5 / F10 / F11 | Continue / Step over instruction / Step instruction |
| Ctrl+Shift+` , ] , [ | New / next / previous terminal |
| Ctrl+Shift+G | Show Git |
| Ctrl+Shift+, | Settings |

Every command is also in the command palette. The full list, kept true by a test, is in
[`docs/architecture/KEYMAP.md`](docs/architecture/KEYMAP.md).

## Repository layout

| Path | Contents |
|---|---|
| `cmd/pyxforge` | The entry point: desktop app and command line |
| `internal/cli` | Command-line commands |
| `internal/ui` | The desktop app: workbench shell, editor view, explorer, palette, design-system widgets, themes, icons |
| `internal/neovim` | The embedded Neovim: process, RPC, screen grid, buffer and terminal events |
| `internal/config` | `pyxforge.toml` parsing and validation |
| `internal/workspace` | Project discovery, saved workspace state, path containment |
| `internal/build` | Build profiles: ordering, running tools, parsing their diagnostics |
| `internal/git` | Local Git: status, stage, commit, branches, stash |
| `internal/qemu` | QEMU launch, serial output, QMP, machine snapshots |
| `internal/gdb` | GDB/MI: attach, registers, memory, breakpoints, stepping |
| `internal/inspect` | Hex dumps, boot-sector map, x86 disassembly, ELF summaries |
| `internal/snapshot` | Diagnostic snapshots and their comparison |
| `internal/toolchain` | Detection of the tools PyxForge drives |
| `internal/proc` | Child processes that end with PyxForge |
| `internal/command`, `internal/buildinfo` | Command registry; version and build information |
| `nvim/` | PyxForge's Neovim configuration (Lua) and plugin lockfile, embedded in the binary |
| `tools/` | `forbidcheck` (no web technology), `snapshot` (review renders), `release` (reproducible builds), `nvimspike` |
| `examples/` | A BIOS boot sector project |
| `docs/` | Install guide, roadmap, architecture and decisions, design system, references; 2.x documents in `docs/archive/2.x` |
| `legacy/` | The 2.x Rust core, VS Code extension and Tauri app, kept until parity |

## Development checks

CI runs these on Linux and Windows, compiles on macOS, and keeps a release build of each push:

```sh
go run ./tools/forbidcheck
gofmt -l .
go vet ./...
staticcheck ./...
go test -race ./...
```

The integration tests drive real Neovim, Git, QEMU, `qemu-img`, GDB and NASM when installed and
skip, visibly, when not. Contributing guidelines: [`CONTRIBUTING.md`](CONTRIBUTING.md); developer
setup: [`docs/development/SETUP.md`](docs/development/SETUP.md); design system:
[`docs/design/DESIGN_SYSTEM.md`](docs/design/DESIGN_SYSTEM.md).

## Known limitations

- macOS is not supported yet: it compiles, nothing more is verified.
- Screen readers cannot read the app: Fyne has no accessibility API. Everything is reachable by
  keyboard.
- Machine snapshots work for `boot_image` projects and are lost when the image is rebuilt.
- rustc human-readable and MSVC compiler messages are not parsed yet.
- No agents, Git worktrees or remote Git operations in the app.

## The 2.x stack

The Rust core (`legacy/core`) is the reference for porting behaviour; its tests became golden
cases for the Go implementation. Build and test it with `cargo test` in `legacy/core`. The
VS Code extension builds with `npm ci && npm run compile` in `legacy/extension`. 2.x design and
planning documents are in [`docs/archive/2.x`](docs/archive/2.x/README.md).

## License

Apache License 2.0. See [`LICENSE`](LICENSE). Bundled fonts and icons keep their own licenses,
listed in [`docs/design/FONT_LICENSES.md`](docs/design/FONT_LICENSES.md).
