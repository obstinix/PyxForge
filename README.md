# PyxForge

PyxForge is a comprehensive, production-grade developer platform and VS Code toolset designed to simplify from-scratch operating system and systems development. It bridges the gap between raw compiler tools, emulators, debuggers, and modern developer interfaces by wrapping complex pipelines into unified commands and panels.

With PyxForge, OS developers can build, boot, debug, inspect, and analyze their custom BIOS bootloaders, kernels, and bare-metal systems directly within VS Code.

---

## Key Features

- 🛠️ **Unified Build & Diagnostic Pipeline**: Integrated parser maps output logs from `nasm`, `cargo`/`rustc`, `gcc`/`clang`, and linkers (`ld`, `link.exe`) into the standard VS Code **Problems panel** and editor gutter.
- ⚙️ **Build Profile Presets**: One-click configuration presets for Bootloaders, Kernels (Debug/Release), Rust Apps, C/C++ Apps, Embedded targets, and Bare Metal stages.
- 💻 **QEMU Process Manager**: One-click launch, run, stop, and status polling of guest virtual machines with automatic GDB server configurations.
- 🐛 **Architecture-Aware GDB Attach**: Connect VS Code debugger natively to QEMU real mode (16-bit i8086), protected mode (32-bit i386), or long mode (64-bit x86-64) without manually scripting GDB.
- 🔍 **Inspector Panel**: Dynamic webview panel providing visual rendering of CPU registers, flags, stack traces, and custom memory dumps while debugging.
- 🔢 **Hex Explorer**: Native file-level hex dump view mapping compiled binaries directly to hexadecimal/ASCII offsets.
- 🤖 **vscode.lm AI Copilot**: Deep integration with VS Code Language Model APIs to explain assembly instructions, CPU registers, or compile errors on the fly.
- 🧪 **Robust CI Validation**: Automated workflow building, formatting, linting, unit testing, and integration testing on Linux, macOS, and Windows.

---

## Directory Structure

```
├── .github/workflows/   # Github Actions CI matrix build configurations
├── core/                # Rust Native Engine backend
│   ├── .cargo/          # Cargo target and build flags
│   ├── src/             # Core commands (build, qemu, gdb, hex, scaffold, protocol)
│   └── Cargo.toml       # Cargo project specifications
├── docs/                # Developer guides and system design documentation
│   ├── architecture/    # Architectural decision records (ADRs)
│   ├── PRD.md           # Product Requirement Document & Progress Tracker
│   └── development-log.md # Chronological engineering log
├── extension/           # VS Code Extension frontend
│   ├── src/             # Extension scripts (activation, UI panels, diagnostics, presets)
│   ├── themes/          # Webview theme CSS definitions
│   ├── package.json     # Extension command and menu registrations
│   └── tsconfig.json    # TypeScript project configurations
└── README.md            # Project homepage and guides
```

---

## Architecture Overview

```
                      VS Code UI (Editor, Gutter, Problems)
                                 │
                         PyxForge Extension
             ┌───────────────────┼────────────────────┐
             ▼                   ▼                    ▼
       AI Helper Panel    Inspector Panel      Hex Viewer Panel
             │                   │                    │
             │           (Webview themes)             │
             ▼                   │                    ▼
      vscode.lm Chat             │             Core Hex Dump API
                                 ▼
                     GDB Debug Session / Tracker
                                 │
                                 ▼
                   Core Binary (JSON RPC stdin/out)
             ┌───────────────────┴────────────────────┐
             ▼                                        ▼
      Build Orchestrator                        QEMU Launcher
   (nasm, rustc, gcc, ld)                        (guest OS)
```

---

## Installation

### Prerequisites

1. **Rust Toolchain**: Install Rust stable (via `rustup`).
2. **Node.js**: Install Node.js (v24 recommended) and `npm`.
3. **Emulator**: Install QEMU (specifically `qemu-system-x86_64` or target architecture). Ensure it is in your system `PATH`.
4. **Debugger**: Install GDB or `gdb-multiarch` and ensure it is in your system `PATH`.
5. **VS Code Extension**: Install the **Native Debug** extension (`webfreak.debug`) in VS Code to enable debugger attach.

### Building PyxForge

1. Clone the repository:
   ```bash
   git clone https://github.com/obstinix/PyxForge.git
   cd PyxForge
   ```

2. Compile the Rust core backend:
   ```bash
   cd core
   cargo build
   ```

3. Build the VS Code Extension:
   ```bash
   cd ../extension
   npm install
   npm run build
   ```

---

## Quick Start

1. Open VS Code and load the `extension/` workspace.
2. Hit `F5` to open the **Extension Development Host**.
3. In the new window, open an empty folder or a test project.
4. Run `PyxForge: Initialize Project` from the Command Palette (`Ctrl+Shift+P`).
5. Choose **Assembly** or **Rust** template and enter a project name.
6. Run `PyxForge: Select Build Profile Preset` to configure your build targets.
7. Run `PyxForge: Build` to compile the system. Errors and warnings will be parsed into the **Problems panel**.
8. Run `PyxForge: Launch QEMU (Debug)` to start QEMU in a paused state.
9. Run `PyxForge: Debug (GDB Attach)` to connect the VS Code debugger and step through code!

---

## Configuration Settings

PyxForge manages configuration locally via a `pyxforge.toml` file generated at the project root.

### Example `pyxforge.toml`

```toml
[project]
name = "my-test-os"

[profiles.bootloader]
tool = "nasm"
description = "Assemble the stage 1 bootloader"
source_dir = "."
output_dir = "build"
args = ["-f", "bin", "boot.asm", "-o", "build/boot.bin"]

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

### Visual Themes

You can customize the UI panels (Inspector, Hex Explorer, AI panel) via VS Code settings. Navigate to **Extension Settings** or add to `.vscode/settings.json`:
```json
"pyxforge.theme": "mono" // "auto" | "mono" | "contrast" | "hybrid"
```

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

## Extension Testing

PyxForge features an extensive integration test suite running in a mocked VS Code Electron host.

To execute tests:
```bash
cd extension
npm test
```

This compiles tests, boots a headless VS Code instance, activates the extension, registers commands, parses various compiler diagnostic outputs (Rustc, MSVC, GCC, GNU Linker), loads presets, parses configuration trees, and confirms overall extension reliability.

---

## CI/CD Pipeline

The project uses GitHub Actions to run matrix validation tests on push or pull request to the `main` branch.

Validated items include:
- **Operating Systems**: Windows, Ubuntu Linux, macOS
- **Rust Core**: Format check (`rustfmt`), static analysis (`clippy`), unit tests (`cargo test`), API docs generation (`cargo doc`)
- **VS Code Extension**: TypeScript type checking (`tsc`), code linting (`eslint`), web packaging (`esbuild`), headless extension host integration tests (`vscode-test` with Xvfb on Linux)

---

## Contributing

Please read [CONTRIBUTING.md](CONTRIBUTING.md) for style guides, commits format, and branching rules.

### Development Workflow

1. Modify source file.
2. Verify it builds and lints locally.
3. Commit using conventional commit format:
   ```bash
   git commit -m "feat(diagnostics): support cargo JSON output"
   ```
4. Push changes immediately to remote branch.

---

## Cross-Project Sibling Integration

PyxForge is fully aligned with its sibling operating system project, **PyxisOS**. Developers can use PyxForge build pipelines and debugging wrappers to compile, launch, and inspect PyxisOS's freestanding `lunar-core` kernel.

For a detailed integration guide and step-by-step configuration tutorial, refer to the [PyxisOS Integration Study](docs/cross-project/pyxisos-integration.md).

---

## License

This project is licensed under the Apache 2.0 License - see the [LICENSE](LICENSE) file for details.
