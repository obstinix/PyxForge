# PyxForge

Core tooling for from-scratch OS and systems development.

## Project Status
QEMU process management implemented (Phase 2).
For details on the project scope and milestones, see [docs/PRD.md](docs/PRD.md).

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

- **Launch QEMU**: Run `PyxForge: Launch QEMU` from the Command Palette (`Ctrl+Shift+P`). This starts QEMU as a detached background process (pre-paused for debugging if enabled) and adds a status bar item.
- **Stop QEMU**: Run `PyxForge: Stop QEMU` from the Command Palette or click the status bar item to terminate the running instance.
- **Auto-Cleanup**: Closing VS Code automatically stops the running QEMU process.

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
