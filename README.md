# PyxForge

Core tooling for from-scratch OS and systems development.

## Project Status
Scaffolding generator and presets implemented (Phase 5).
For details on the project scope and milestones, see [docs/PRD.md](docs/PRD.md).

## Getting Started: Scaffolding a New Project

PyxForge can automatically bootstrap a complete, runnable real-mode x86 operating system workspace for you:

1. Open an empty folder in VS Code.
2. Open the Command Palette (`Ctrl+Shift+P` or `Cmd+Shift+P`) and run `PyxForge: Initialize Project`.
3. Provide a project name when prompted.
4. PyxForge will generate:
   - `pyxforge.toml`: Pre-configured build profile and QEMU/GDB presets.
   - `boot.asm`: A minimal real-mode assembly bootloader that prints a hello message and loops.
   - `Makefile`: Commands to compile the boot sector.
   - `.vscode/tasks.json` and `.vscode/launch.json`: VS Code task and debugging shortcuts.
5. Hit `Ctrl+Shift+B` (or run `PyxForge: Build`) to compile `boot.asm` to `build/boot.bin`.
6. Run `PyxForge: Launch QEMU (Run)` to boot the image.

## Build Profiles

PyxForge uses a `pyxforge.toml` file at the root of your OS project to define build profiles.

### Example `pyxforge.toml`

```toml
[project]
name = "my-os"

[profiles.bootloader]
tool = "nasm"
description = "Assemble the bootloader"
source_dir = "boot"
output_dir = "build"
args = ["-f", "bin", "boot.asm", "-o", "boot.bin"]

[profiles.kernel]
tool = "make"
description = "Build the kernel"
args = ["all"]
depends_on = ["bootloader"]

[profiles.kernel.env]
CC = "x86_64-elf-gcc"
```

### Profile fields

| Field | Required | Description |
|---|---|---|
| `tool` | Yes | The build tool to invoke (e.g. `nasm`, `make`, `gcc`, `rustc`) |
| `description` | No | Human-readable description shown in the profile picker |
| `source_dir` | No | Working directory relative to project root (default: `.`) |
| `output_dir` | No | Output directory relative to project root (default: `build`) |
| `args` | No | Arguments to pass to the tool |
| `env` | No | Environment variables to set during the build |
| `depends_on` | No | Other profiles that must be built first |

### Using build profiles

1. Open your OS project (containing `pyxforge.toml`) in VS Code.
2. Open the Command Palette (`Ctrl+Shift+P`), run `PyxForge: Build`.
3. Select a profile from the picker.
4. Build output appears in the PyxForge output channel.

## QEMU Process Management

PyxForge provides one-click launching and stopping of QEMU from your editor. You can configure QEMU settings using the `[qemu]` section in your `pyxforge.toml`.

### Example `[qemu]` section

```toml
[qemu]
executable = "qemu-system-x86_64"  # Default
machine = "pc"                     # Default
memory = "128M"                    # Default
boot_image = "build/boot.bin"      # Required
extra_args = []                    # Default

[qemu.debug]
enabled = true                     # Default
gdb_port = 1234                    # Default
```

### QEMU configuration fields

| Field | Required | Default | Description |
|---|---|---|---|
| `executable` | No | `qemu-system-x86_64` | Path or command for the QEMU executable |
| `machine` | No | `pc` | The QEMU machine type to emulate |
| `memory` | No | `128M` | Amount of guest RAM to allocate |
| `boot_image` | Yes | - | Path to the OS boot sector/drive image |
| `extra_args` | No | `[]` | List of additional command line flags to pass to QEMU |
| `debug.enabled` | No | `true` | Launch with `-s -S` for GDB remote debugging |
| `debug.gdb_port` | No | `1234` | The port the GDB stub listens on (tcp) |

### Using QEMU commands

- **Launch QEMU (Debug)**: Run `PyxForge: Launch QEMU (Debug)` from the Command Palette. QEMU starts pre-paused with the GDB stub enabled.
- **Launch QEMU (Run)**: Run `PyxForge: Launch QEMU (Run)` for standard execution without GDB.
- **Stop QEMU**: Run `PyxForge: Stop QEMU` or click the status bar item.
- **Auto-Cleanup**: Closing VS Code automatically stops the running QEMU process.

## GDB Debugging

PyxForge provides one-click GDB remote attach via the [Native Debug](https://marketplace.visualstudio.com/items?itemName=webfreak.debug) extension. Configure GDB settings using the optional `[gdb]` section in `pyxforge.toml`.

### Prerequisites

- Install the **Native Debug** extension (`webfreak.debug`) in VS Code.
- Have GDB installed and accessible in your PATH (or specify its path in `[gdb]`).

### Example `[gdb]` section

```toml
[gdb]
executable = "gdb"        # Default; can be a cross-GDB like "x86_64-elf-gdb"
architecture = "i8086"    # "i8086" | "i386" | "i386:x86-64" | "auto"
```

### GDB configuration fields

| Field | Required | Default | Description |
|---|---|---|---|
| `executable` | No | `gdb` | Path or command for the GDB binary |
| `architecture` | No | `i8086` | CPU architecture for GDB (`i8086` = real mode, `i386` = protected mode, `i386:x86-64` = long mode, `auto` = defaults to `i8086`) |

### Using the debug command

1. Run `PyxForge: Debug (GDB Attach)` from the Command Palette.
2. If QEMU is not already running, PyxForge auto-launches it in Debug Mode.
3. GDB attaches to the QEMU stub with the correct architecture preset.
4. Use VS Code's built-in debug UI (breakpoints, stepping, variables) as normal.

## Verify it works

To manually verify the setup:
1. Build the Rust core:
   ```bash
   cd core && cargo build
   ```
2. Build the VS Code extension:
   ```bash
   cd ../extension && npm install && npm run build
   ```
3. Open the `extension/` folder in VS Code, press `F5` to launch the Extension Development Host.
4. Open the Command Palette (`Ctrl+Shift+P` or `Cmd+Shift+P`), run `PyxForge: Ping Core`.
5. Expect an info message showing `status: ok`, `message: pong`, and the core's version.
