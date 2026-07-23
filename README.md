# PyxForge

[![CI](https://github.com/obstinix/PyxForge/actions/workflows/ci.yml/badge.svg)](https://github.com/obstinix/PyxForge/actions/workflows/ci.yml)
[![License: Apache 2.0](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)
[![Rust](https://img.shields.io/badge/rust-stable%20%2F%20edition%202024-orange.svg)](core/rust-toolchain.toml)

**PyxForge** is a developer platform for from-scratch operating system and bare-metal systems programming. It wraps the fragmented toolchain of an OS developer — assemblers, cross-compilers, QEMU, GDB, and raw hex dumps — into a single coherent workflow: build, boot, debug, and inspect, without hand-rolling shell scripts and GDB command files for every project.

PyxForge ships as **two frontends sharing one Rust core**:

- **PyxForge Desktop** — a standalone, cross-platform control shell built with [Tauri](https://tauri.app/). This is the primary product going forward.
- **VS Code Extension** — the original frontend, kept fully functional as a stable, actively-tested baseline.

Both talk to the same `pyxforge-core` Rust binary over a JSON-RPC protocol, so build logic, QEMU orchestration, and GDB configuration behave identically no matter which one you use. See [ADR 0003](docs/architecture/0003-desktop-ui-stack.md), [ADR 0004](docs/architecture/0004-native-editor-engine.md), and [`docs/PRD.md`](docs/PRD.md) for the reasoning behind the system design.

> **Note:** PyxForge Desktop includes an integrated **CodeMirror 6 native code editor** for editing `.asm`, `.c`, `.h`, `.ld`, `Makefile`, and `pyxforge.toml` files directly, backed by real filesystem RPCs and an interactive file tree explorer.

---

## Table of Contents

- [Key Features](#key-features)
- [Architecture](#architecture)
- [Directory Structure](#directory-structure)
- [Installation](#installation)
- [Quick Start](#quick-start)
- [Configuration](#configuration)
- [Build Profile Presets](#build-profile-presets)
- [VS Code Commands Reference](#vs-code-commands-reference)
- [Testing](#testing)
- [CI/CD Pipeline](#cicd-pipeline)
- [Project Status](#project-status)
- [Documentation](#documentation)
- [Contributing](#contributing)
- [Cross-Project Integration](#cross-project-integration)
- [License](#license)

---

## Key Features

**Code Editor & Workspace Navigation**
- **Native Code Editor (CodeMirror 6)**: Syntax-highlighted editing for bootloader assembly (`.asm`), C/C++ (`.c`, `.h`), JSON, and TOML with custom token styling.
- **Real Filesystem Explorer**: Real-time directory navigation powered by Tauri Rust RPCs (`list_workspace_files`, `read_workspace_file`, `write_workspace_file`).
- **File Save & Dirty Tracking**: Visual modified state indicator (`* modified`) and `Ctrl+S` / `Cmd+S` shortcut. Automatically falls back to binary hex inspection for binary files.

**Build & Diagnostics**
- Unified build pipeline wrapping `nasm`, `cargo`/`rustc`, `gcc`/`g++`/`clang`, `arm-none-eabi-gcc`, and linkers (`ld`, `link.exe`).
- Output parser understands Rustc human-readable text, Cargo's structured JSON diagnostics, GCC/Clang, MSVC, and GNU linker error formats, mapping every one into native `vscode.Diagnostic` entries in the **Problems panel** and editor gutter.

**QEMU Process Management**
- One-click launch (paused-for-debug or free-run), stop, and live status polling of guest VMs.
- QMP-based graceful shutdown, plus full snapshot lifecycle management — save, load, delete, and list VM snapshots by tag.
- Direct QEMU HMP monitor console access for ad-hoc machine inspection (`info block`, `info registers`, etc.) without leaving the tool.

**Architecture-Aware Debugging**
- Native GDB attach for real mode (16-bit i8086), protected mode (32-bit i386/x86), long mode (64-bit x86-64), and ARM (via `gdb-multiarch`) — no hand-written GDB init scripts.

**Inspection Tools**
- **CPU & Memory Inspector**: live registers, flags, stack viewer, arbitrary memory reads, and a pwndbg-inspired disassembly context view.
- **Hex Explorer**: maps compiled binaries to hex/ASCII offsets via the `HexDump` JSON-RPC engine, with boot-sector and boot-signature (`0xAA55`) detection built in.

**AI Copilot** *(VS Code extension)*
- Deep integration with the VS Code Language Model API (`vscode.lm`) to explain selected assembly line-by-line, interpret the current CPU/register state, or suggest fixes for a build error — all streamed inline.

**Desktop Shell Extras**
- **Cyan Accent Design System**: Token-driven styling (`#00D4FF`), self-hosted Space Grotesk and JetBrains Mono fonts, single-stroke Lucide SVG icons, and zero glassmorphism slop.
- Real integrated terminal backed by a native PTY (`portable-pty`) and rendered with `xterm.js`.
- Lightweight plugin loader for extending the workspace with custom JS panels and commands.

**Multi-Target Support**
- Presets cover x86 real-mode bootloaders, protected/long-mode kernels, freestanding Rust, hosted/embedded C and C++, and ARM Cortex-M4 (QEMU `lm3s6965evb`) — see [Build Profile Presets](#build-profile-presets).

**Testing & CI**
- 59 Rust unit tests across the core engine, 10 integration tests for the VS Code extension host, running on every push across Linux, macOS, and Windows.

---

## Architecture

```
+-------------------------------------------------------------------------+
|                      PyxForge Desktop Webview                           |
|                                                                         |
|  +--------------------+   +-------------------+   +------------------+  |
|  | Real File Tree     |   | CodeMirror 6      |   | xterm.js PTY     |  |
|  | Workspace Explorer |   | Code Editor       |   | Terminal         |  |
|  +---------+----------+   +---------+---------+   +--------+---------+  |
|            |                        |                      |            |
+------------|------------------------|----------------------|------------+
             | Tauri IPC (RPC)        | Tauri IPC (RPC)      | PTY Stream
             v                        v                      v
+-------------------------------------------------------------------------+
|                      Tauri Rust Backend (`src-tauri`)                   |
|                                                                         |
|  +-------------------------------------------------------------------+  |
|  | Filesystem RPCs: `list_workspace_files`, `read_workspace_file`,    |  |
|  | `write_workspace_file`, PTY Bridge, `call_core`                   |  |
|  +-----------------------------------+-------------------------------+  |
|                                      |                                  |
|                                      v                                  |
|                        `pyxforge-core` Rust Engine                      |
|                      (build, qemu, qmp, gdb, hex)                       |
+-------------------------------------------------------------------------+


          ┌─────────────────────────────────────────────────┐
          │   VS Code Extension (Stable Baseline Frontend)  │
          │   Same Core Binary over the same stdio protocol │
          │   Adds: vscode.lm AI Copilot, WebviewPanels      │
          └─────────────────────────────────────────────────┘
```

`pyxforge-core` is invoked per-request (each command spawns the binary, writes one JSON-RPC request to stdin, and reads the response from stdout) — it holds no long-lived server state of its own, so both frontends can drive it identically and it stays trivially testable in isolation.

---

## Directory Structure

```
├── .github/workflows/     # CI matrix build configuration
├── core/                  # Rust core engine (IDE-agnostic, zero VS Code deps)
│   ├── .cargo/            # Cargo build/link flags
│   ├── src/                # protocol, build, qemu, qmp, gdb, hex, scaffold, config, diagnostics
│   └── Cargo.toml
├── desktop/               # PyxForge Desktop (Tauri v2 app — primary product)
│   ├── src/                # Frontend: TypeScript, HTML, themes, xterm.js terminal
│   ├── src-tauri/          # Tauri Rust backend (IPC bridge + PTY management)
│   └── package.json
├── docs/                  # Design docs, ADRs, and progress tracking
│   ├── architecture/       # ADRs + formal checkpoint/decision gates
│   ├── cross-project/      # PyxisOS integration guide
│   ├── vision/              # Original product vision
│   ├── PRD.md               # Product requirements & scope decisions
│   ├── ARCHITECTURE_V2.md   # Current system architecture
│   ├── ROADMAP.md           # Phased roadmap and completion status
│   └── development-log.md   # Chronological engineering log
├── extension/              # VS Code Extension (stable baseline frontend)
│   ├── src/                  # Activation, panels, diagnostics, presets, AI helper
│   ├── themes/                # Webview theme CSS
│   └── package.json
├── CONTRIBUTING.md         # Branching, commit, and style conventions
└── README.md
```

---

## Installation

### Prerequisites

1. **Rust toolchain** — install via [`rustup`](https://rustup.rs/) (stable channel).
2. **Node.js** — v24 recommended, plus `npm`.
3. **QEMU** — `qemu-system-x86_64` (and/or `qemu-system-arm` for the Embedded preset) available on your `PATH`.
4. **GDB** — `gdb` or `gdb-multiarch` on your `PATH`.
5. **Tauri build prerequisites** — required only for building Desktop; see the [official Tauri setup guide](https://tauri.app/start/prerequisites/) for your OS.
6. **Native Debug** VS Code extension (`webfreak.debug`) — required only for GDB attach from the VS Code extension.

### Building PyxForge

```bash
git clone https://github.com/obstinix/PyxForge.git
cd PyxForge
```

**1. Compile the Rust core:**
```bash
cd core
cargo build
```

**2. Build PyxForge Desktop (primary product):**
```bash
cd ../desktop
npm install
npm run tauri dev
```

**3. *(Optional)* Build the VS Code Extension:**
```bash
cd ../extension
npm install
npm run build
```

---

## Quick Start

### PyxForge Desktop

1. Run `npm run tauri dev` from `desktop/` (see above) to launch the app.
2. Click **Ping Core Backend** to confirm the Tauri shell can reach `pyxforge-core`.
3. Enter a project name and click **Initialize Project** to scaffold a template.
4. Click **Fetch Profiles**, then run a build profile from the **Build Profiles** list.
5. Use the **QEMU Snapshot Manager** and **Monitor Console** to control and inspect the running guest.
6. Switch between the **Integrated Terminal** and **Hex Viewer** tabs, and use the **CPU & Memory Inspector** on the right while debugging.

### VS Code Extension

1. Open VS Code and load the `extension/` folder as a workspace.
2. Hit `F5` to launch the **Extension Development Host**.
3. In the new window, open an empty folder or existing project.
4. Run **`PyxForge: Initialize Project`** from the Command Palette (`Ctrl+Shift+P`) and choose the **Assembly** or **Rust** template.
5. Run **`PyxForge: Select Build Profile Preset`** to configure your build target.
6. Run **`PyxForge: Build`** — errors and warnings populate the **Problems panel**.
7. Run **`PyxForge: Launch QEMU (Debug)`** to start QEMU paused with a GDB stub.
8. Run **`PyxForge: Debug (GDB Attach)`** to connect the VS Code debugger and step through code.

---

## Configuration

PyxForge is configured per-project via a `pyxforge.toml` file at the project root.

```toml
[project]
name = "my-test-os"

[profiles.bootloader]
tool = "nasm"
description = "Assemble the stage 1 bootloader"
source_dir = "."
output_dir = "build"
args = ["-f", "bin", "boot.asm", "-o", "build/boot.bin"]
# Optional: env = { KEY = "value" }, depends_on = ["other_profile"]

[qemu]
executable = "qemu-system-x86_64"
machine = "pc"
memory = "128M"
boot_image = "build/boot.bin"

[qemu.debug]
enabled = true
gdb_port = 1234

[gdb]
executable = "gdb"
architecture = "i8086"
```

Profiles support per-profile environment variables (`env`), build ordering (`depends_on`), and per-profile GDB overrides — see [`core/src/config.rs`](core/src/config.rs) for the full schema.

### Visual Themes

Both frontends share the same theme stylesheets. In VS Code, set:
```json
"pyxforge.theme": "mono" // "auto" | "mono" | "contrast" | "hybrid"
```
In Desktop, use the **Theme** selector in the titlebar.

---

## Build Profile Presets

| Preset | Tool | Description |
|---|---|---|
| **Bootloader** | `nasm` | Assemble a BIOS bootloader (real mode, i8086 target) |
| **Kernel Debug** | `gcc` | Freestanding C kernel, debug symbols, no optimization |
| **Kernel Release** | `gcc` | Freestanding C kernel, optimized, symbols stripped |
| **Rust Application** | `cargo` | Bare-metal Rust kernel/app via Cargo |
| **C Application** | `gcc` | Standard host or embedded C application |
| **C++ Application** | `g++` | Standard host or embedded C++ application |
| **Embedded** | `arm-none-eabi-gcc` | ARM Cortex-M4, QEMU `lm3s6965evb` target |
| **Bare Metal** | `nasm` | Bare-metal stage binary, x86 protected mode |
| **Custom** | `make` | Skeleton configuration for fully custom pipelines |

Each preset generates a ready-to-edit `pyxforge.toml` — select one via **`PyxForge: Select Build Profile Preset`** (extension) or the **Build Profiles** panel (Desktop).

---

## VS Code Commands Reference

| Command ID | Title | Description |
|---|---|---|
| `pyxforge.ping` | `PyxForge: Ping Core` | Checks communication with the Rust core backend |
| `pyxforge.build` | `PyxForge: Build` | Lists profiles and builds the selected target |
| `pyxforge.selectPreset` | `PyxForge: Select Build Profile Preset` | Overwrites or updates `pyxforge.toml` with presets |
| `pyxforge.launch` | `PyxForge: Launch QEMU (Debug)` | Starts QEMU paused with GDB stub enabled |
| `pyxforge.launchNoDebug` | `PyxForge: Launch QEMU (Run)` | Runs QEMU normally without pausing |
| `pyxforge.stop` | `PyxForge: Stop QEMU` | Force kills active QEMU instances |
| `pyxforge.debug` | `PyxForge: Debug (GDB Attach)` | Launches debug session and attaches to QEMU GDB stub |
| `pyxforge.showInspector` | `PyxForge: Show Inspector Panel` | Opens the Register & Stack Viewer |
| `pyxforge.init` | `PyxForge: Initialize Project` | Bootstraps a fresh project template |
| `pyxforge.openHex` | `PyxForge: Open in Hex Viewer` | Renders the selected binary in hex layout |
| `pyxforge.explainAsm` | `PyxForge: Explain Assembly` | Streams line-by-line assembly explanation via AI |
| `pyxforge.explainBuild` | `PyxForge: Explain Build Error` | Prompts AI to suggest fixes for recent compile errors |

---

## Testing

**Core (Rust):**
```bash
cd core
cargo test
```
59 unit tests cover the JSON-RPC protocol, build orchestration, QEMU/QMP argument construction, GDB configuration, hex dumping, and project scaffolding — running identically on Linux, macOS, and Windows.

**Anti-Slop Linter:**
```bash
bash scripts/lint-slop.sh
```
Validates zero off-brand color violations, zero glassmorphism (`backdrop-filter`), zero runtime Google Fonts imports, and enforces token compliance.

**Extension (TypeScript):**
```bash
cd extension
npm test
```
10 integration tests boot a headless VS Code Extension Host (`vscode-test`), register commands, and exercise diagnostic parsing across Rustc, Cargo JSON, GCC/Clang, MSVC, and GNU linker output formats, plus preset and theme configuration handling.

---

## CI/CD Pipeline

GitHub Actions runs on every push and pull request to `main`, across a full OS matrix:

- **Operating Systems:** Ubuntu, macOS, Windows
- **Rust Core:** format check (`rustfmt`), static analysis (`clippy`), unit tests (`cargo test`), API docs (`cargo doc`)
- **VS Code Extension:** type checking (`tsc`), linting (`eslint`), bundling (`esbuild`), headless integration tests (`vscode-test`, via `xvfb-run` on Linux)

See [`.github/workflows/ci.yml`](.github/workflows/ci.yml) for the full workflow.

---

## Project Status

PyxForge began as a VS Code extension (Phases 0–12: core protocol, build pipeline, diagnostics, QEMU/QMP, CPU Inspector, Hex Viewer, AI panel, themes). A formal checkpoint decision on 2026-07-19 evaluated a standalone desktop shell against the extension baseline and resolved to make **PyxForge Desktop the primary product**, with the extension preserved as a fully working, fully tested baseline frontend — see [`docs/architecture/CHECKPOINTS.md`](docs/architecture/CHECKPOINTS.md) for the full decision record.

All master roadmap phases (**Phases 0–23**) are complete and merged into `main`:
- **Phase 21 (Design System De-Sloppification):** Cyan-accented design tokens (`#00D4FF`), self-hosted fonts (`@fontsource/*`), single-stroke Lucide icons, rebranded shell config, and anti-slop linter (`scripts/lint-slop.sh`).
- **Phase 22 (Editor Architecture Study):** [ADR 0004](docs/architecture/0004-native-editor-engine.md) approved Option A (CodeMirror 6 webview engine paired with Rust filesystem RPCs).
- **Phase 23 (Native Code Editor):** Integrated CodeMirror 6 code editor (`editor.ts`), real Rust filesystem RPCs (`list_workspace_files`, `read_workspace_file`, `write_workspace_file`), `Ctrl+S`/`Cmd+S` save workflow, dirty state tracking, and binary `HexDump` fallback.

For the detailed phase breakdown, see [`docs/ROADMAP.md`](docs/ROADMAP.md).

---

## Documentation

- [`docs/PRD.md`](docs/PRD.md) — Product requirements, scope, and open-question resolutions
- [`docs/ARCHITECTURE_V2.md`](docs/ARCHITECTURE_V2.md) — Current system architecture
- [`docs/ROADMAP.md`](docs/ROADMAP.md) — Phased roadmap and completion status
- [`docs/architecture/`](docs/architecture) — Architectural Decision Records (ADRs) and checkpoint gates
- [`docs/vision/PYXFORGE_VISION.md`](docs/vision/PYXFORGE_VISION.md) — Original long-term product vision
- [`docs/development-log.md`](docs/development-log.md) — Chronological engineering log

---

## Contributing

Please read [CONTRIBUTING.md](CONTRIBUTING.md) for branching, commit conventions, and style guides.

**Development workflow:**
1. Modify source on a feature branch (`pyxforge/<description>`).
2. Verify it builds and lints locally.
3. Commit using Conventional Commits:
   ```bash
   git commit -m "feat(core): add QEMU monitor command support"
   ```
4. Push after every commit.

---

## Cross-Project Integration

PyxForge is developed alongside its sibling operating system project, **PyxisOS**. Its build pipelines and debugging wrappers compile, launch, and inspect PyxisOS's freestanding `lunar-core` kernel directly, keeping compiler dependencies cleanly separated while enabling a seamless bare-metal guest OS workflow.

See the [PyxisOS Integration Study](docs/cross-project/pyxisos-integration.md) for a full walkthrough.

---

## License

Licensed under the [Apache License 2.0](LICENSE).
