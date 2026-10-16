<div align="center">

# ⚒️ PyxForge

### A native, keyboard-first workbench for operating-system development

**Write assembly. Build kernels. Boot QEMU. Debug with GDB. Inspect binaries. All in one desktop workspace.**

[![CI](https://github.com/obstinix/PyxForge/actions/workflows/ci.yml/badge.svg)](https://github.com/obstinix/PyxForge/actions/workflows/ci.yml)
[![License: Apache 2.0](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)
![Go](https://img.shields.io/badge/Go-1.27.1%2B-00ADD8?logo=go&logoColor=white)
![Fyne](https://img.shields.io/badge/UI-Fyne-4B6B8A)
![Status](https://img.shields.io/badge/Status-Pre--release-orange)
![Platforms](https://img.shields.io/badge/Platforms-Windows%20%7C%20Linux-555)

[**Get Started**](#-quick-start) · [**Features**](#-features) · [**CLI**](#-command-line) · [**Documentation**](docs/INSTALL.md) · [**Roadmap**](docs/ROADMAP.md) · [**Contributing**](CONTRIBUTING.md)

</div>

---

## ✨ Why PyxForge?

Low-level development often means switching between an editor, compiler, terminal, emulator, debugger, and binary inspection tools. **PyxForge brings that workflow into one offline-first, native desktop environment** built for bootloaders, kernels, and bare-metal projects.

PyxForge **3.0** uses **Go + Fyne** for its desktop workbench and embeds **real Neovim**, configured independently in Lua. The previous Tauri/VS Code-based 2.x implementation remains available in [`legacy/`](legacy/) while 3.0 moves toward feature parity.

> [!IMPORTANT]
> **Pre-release software.** The editor, builds, terminal, local Git, and QEMU/GDB workflows are implemented and tested on Windows and Linux. **Agents are not implemented.** See [current verified state](docs/architecture/CURRENT_STATE.md) and the [roadmap](docs/ROADMAP.md) before relying on PyxForge for production work.

## 🚀 Features

| Workspace | Capabilities |
| :-- | :-- |
| 📝 **Neovim editor** | Embedded Neovim, Lua configuration, Tree-sitter, language-server support, tabs, diagnostics, and clipboard integration |
| 🏗️ **Build system** | `pyxforge.toml` profiles, dependency ordering, streaming output, cancellation, and compiler diagnostics |
| 🖥️ **Integrated terminal** | Named shell sessions in the project directory |
| 🌿 **Local Git** | Status, diffs, staging, commits, history, branches, and stashes |
| ▶️ **QEMU runner** | Build and boot images, view serial output, and issue QEMU monitor commands |
| 🐛 **GDB debugger** | Breakpoints, instruction stepping, registers, flags, memory, and disassembly |
| 🔎 **Binary inspector** | Hex views, BIOS boot-sector mapping, signature checks, ELF sections, and symbols |
| 📸 **Snapshots** | Compare diagnostic snapshots or save/restore supported QEMU machine states |
| 🎨 **Native workbench** | Explorer, command palette, inspector, status bar, persistent workspace layout, and themes |

### Built for low-level workflows

```text
Source code / Assembly
         │
         ▼
 Embedded Neovim ──────► Build profiles (pyxforge.toml)
                                  │
                                  ▼
                           Compiler / Assembler
                                  │
                                  ▼
                             Boot image / ELF
                                  │
                     ┌────────────┴────────────┐
                     ▼                         ▼
                  QEMU boot               Binary inspect
                     │
                     ▼
                 GDB debugging
                     │
                     ▼
            Registers · Memory · Disasm
```

## 🧰 Technology stack

| Layer | Technology |
| :-- | :-- |
| Core application and CLI | Go 1.27.1+ |
| Native desktop interface | Fyne v2 |
| Editor | Neovim, Lua, Tree-sitter, language servers |
| Build configuration | TOML (`pyxforge.toml`) |
| Virtualization / emulation | QEMU |
| Debugging | GDB/MI |
| Source control | Local Git |
| Supported development platforms | Windows 11 x86-64; Linux x86-64 |

## ⚡ Quick start

**Prerequisites:** Go **1.27.1+** and a C compiler for Fyne/OpenGL. Additional tools such as Neovim, QEMU, GDB, NASM, and language servers enable their corresponding features. See the [installation guide](docs/INSTALL.md) for platform-specific commands.

```bash
git clone https://github.com/obstinix/PyxForge.git
cd PyxForge
go build -o pyxforge ./cmd/pyxforge
```

Check which development tools are available:

```bash
./pyxforge doctor
./pyxforge doctor --commands
```

Open a project:

```bash
./pyxforge path/to/project
```

> **Windows:** Run the built executable as `pyxforge.exe` (for example, `./pyxforge.exe doctor` in PowerShell). The build output filename can be specified explicitly with `-o pyxforge.exe`.

To install the editor's pinned plugins and Tree-sitter parsers, run once while online:

```bash
./pyxforge setup editor
```

PyxForge makes **no network request on startup**, requires **no account**, and sends **no telemetry**. Editor setup is the documented network-using operation.

## 💻 Command line

The desktop application and CLI share project discovery and tool detection.

| Command | Purpose |
| :-- | :-- |
| `pyxforge help` | Show commands and options |
| `pyxforge version` | Show version, Go/Fyne versions, and source revision |
| `pyxforge doctor [--commands]` | Detect tools and show installation guidance |
| `pyxforge info [folder]` | Show project root, config, and Git checkout |
| `pyxforge build [profile]` | Execute a build profile and its dependencies |
| `pyxforge build --list` | List available build profiles |
| `pyxforge run [--debug]` | Build and boot in QEMU; optionally wait for GDB |
| `pyxforge inspect FILE [--disasm]` | Inspect boot sectors or ELF binaries |
| `pyxforge setup editor` | Install pinned editor dependencies |

Most commands support `--json`. Exit codes: `0` success, `1` detected problem, `2` invalid command line. Ctrl+C stops the command and its child tools.

## 🔧 Build, run & debug

### 1. Configure a build

Projects define build profiles and dependencies in [`pyxforge.toml`](docs/reference/pyxforge-toml.md). Build output streams into the workbench; recognized GCC, Clang, NASM, linker, rustc, Cargo, and MSVC-format diagnostics appear in **Problems** and the editor.

Try the included [`examples/boot-sector`](examples/boot-sector) project: a complete **512-byte BIOS boot sector**. For a larger example, see [PyxisOS integration](docs/cross-project/pyxisos-integration.md).

### 2. Run in QEMU

**Run in QEMU** builds the selected target and boots the configured image. The QEMU panel shows serial output and accepts monitor commands such as `info registers`.

### 3. Debug with GDB

**Debug in QEMU** starts the emulator paused and attaches GDB at the first instruction (for example, `0x7c00` for a BIOS boot sector or the entry point for an ELF kernel).

| Inspector view | What it shows |
| :-- | :-- |
| Registers | Current CPU registers and changed values |
| Flags | EFLAGS, bit by bit |
| Hex | Image bytes and boot-sector signature |
| Map | Boot-sector regions, padding, signature, and detected structures |
| Disasm | Instructions around the current address, including real-mode decoding |
| Memory | Stack contents or memory at a chosen address |
| Snapshots | Diagnostic comparisons and supported QEMU machine states |

**Snapshot distinction:** diagnostic snapshots compare paused machine observations but cannot restore execution. QEMU machine-state snapshots can restore supported machines; they require `snapshots = true` in `[qemu]` and `qemu-img`.

## ⌨️ Keyboard shortcuts

| Shortcut | Action |
| :-- | :-- |
| `Ctrl+Shift+P` / `Ctrl+Shift+O` | Command palette / Go to file |
| `Ctrl+S` (editor) / `Ctrl+Shift+S` | Save |
| `Ctrl+Shift+E` / `J` / `I` | Toggle explorer / bottom panel / inspector |
| `Ctrl+Shift+B` | Build |
| `Ctrl+Shift+M` | Problems panel |
| `Ctrl+Shift+F8` / `Ctrl+Shift+F7` | Next / previous problem |
| `Ctrl+Shift+R` / `D` | Run / debug in QEMU |
| `Ctrl+Shift+F2` | Stop QEMU |
| `Ctrl+Shift+F5` / `F10` / `F11` | Continue / step over / step instruction |
| `Ctrl+Shift+G` | Git panel |
| `Ctrl+Shift+W` | Close editor tab |
| `Ctrl+Shift+PageDown` / `PageUp` | Next / previous tab |
| `Ctrl+Shift+` + `` ` `` | New terminal |
| `Ctrl+Shift+,` | Settings |

Neovim handles keystrokes while the editor or terminal is focused. Shell-level shortcuts use `Ctrl+Shift` (or `Cmd+Shift` on macOS). See the [complete keymap](docs/architecture/KEYMAP.md).

## 📁 Repository structure

```text
PyxForge/
├── cmd/pyxforge/          # Desktop + CLI entry point
├── internal/
│   ├── ui/                # Fyne workbench and design system
│   ├── neovim/            # Embedded editor integration
│   ├── cli/               # Command-line interface
│   ├── build/             # Build orchestration and diagnostics
│   ├── config/            # TOML parsing and validation
│   ├── workspace/         # Project and session state
│   ├── git/               # Local Git operations
│   ├── qemu/              # Emulator control and snapshots
│   ├── gdb/               # Debugger integration
│   ├── inspect/           # Binary inspection
│   ├── snapshot/          # Diagnostic comparisons
│   ├── toolchain/         # Tool detection
│   └── ...                # Process, commands, build info
├── nvim/                  # Isolated Lua editor configuration
├── examples/boot-sector/  # Sample BIOS boot sector
├── tools/                 # Checks, snapshots, release tooling
├── docs/                  # Guides, architecture, roadmap
├── legacy/                # Previous 2.x implementation
└── go.mod
```

PyxForge isolates its editor configuration with `NVIM_APPNAME=pyxforge`; it does **not** read or overwrite your normal Neovim configuration. See [developer setup](docs/development/SETUP.md) and the [design system](docs/design/DESIGN_SYSTEM.md).

## 🖥️ Platform support

| Platform | Status |
| :-- | :-- |
| Windows 11 (x86-64) | ✅ Supported and tested |
| Linux (x86-64, Ubuntu 24.04) | ✅ Supported and tested (CI / WSL2) |
| macOS | ⚠️ Compiles in CI, **not runtime-tested or supported** |

## 🧪 Development & quality checks

```bash
go run ./tools/forbidcheck
gofmt -l .
go vet ./...
staticcheck ./...
go test -race ./...
```

CI exercises Linux and Windows, compiles on macOS, and produces release builds. Integration tests use real Neovim, Git, QEMU, `qemu-img`, GDB, and NASM where installed; otherwise they report skips.

Read [CONTRIBUTING.md](CONTRIBUTING.md) before submitting changes.

## 🗺️ Roadmap & current limitations

Development progress is tracked in the [roadmap](docs/ROADMAP.md), with verified implementation details in [CURRENT_STATE.md](docs/architecture/CURRENT_STATE.md).

**Current limitations:**

- Agents, Git worktrees, and in-app remote Git operations are not available. Use the terminal for Git push/pull.
- macOS support is not verified beyond CI compilation.
- Screen-reader access is not supported by the Fyne interface, although keyboard navigation is available.
- Machine-state snapshots are limited to `boot_image` projects and are invalidated when the image is rebuilt.
- MSVC-format diagnostics are tested with Clang's `-fdiagnostics-format=msvc` and documented `cl`/`link` lines, not with Microsoft's compiler itself.

## 🤝 Contributing

Bug reports, documentation improvements, and code contributions are welcome. Start with the [contribution guide](CONTRIBUTING.md), [development setup](docs/development/SETUP.md), and [design-system documentation](docs/design/DESIGN_SYSTEM.md).

## 📜 License

Distributed under the **Apache License 2.0**. See [LICENSE](LICENSE). Bundled fonts and icons have separate licenses documented in [FONT_LICENSES.md](docs/design/FONT_LICENSES.md).

---

<div align="center">

**PyxForge — from source code to the first instruction.**

[Back to top ↑](#️-pyxforge)

</div>
