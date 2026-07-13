# PyxForge

Core tooling for from-scratch OS and systems development.

## Project Status
Build profile manager implemented (Phase 1).
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
