# pyxforge.toml

A project's `pyxforge.toml` sits at its root folder. PyxForge finds it from any folder inside the
project (`pyxforge info` shows which file is in use). The format is unchanged from PyxForge 2.x,
so existing projects open as they are.

```toml
[project]
name = "pyxos"                        # required, not empty
description = "A teaching kernel"     # optional

[profiles.bootloader]                 # one table per build step
tool = "nasm"                         # required: the program to run
args = ["-f", "bin", "boot.asm", "-o", "../build/boot.bin"]
source_dir = "boot"                   # working folder, relative to the root; default "."
output_dir = "build"                  # default "build"
description = "Assemble the boot sector"

[profiles.kernel]
tool = "make"
args = ["all"]
depends_on = ["bootloader"]           # built first; every name must exist

[profiles.kernel.env]                 # environment for this step
CC = "x86_64-elf-gcc"

[profiles.kernel.gdb]                 # optional per-profile debugger overrides
architecture = "i386:x86-64"

[qemu]                                # optional
executable = "qemu-system-x86_64"     # default
machine = "pc"                        # default
memory = "128M"                       # default
boot_image = "build/boot.bin"         # boot_image or kernel is required, and not empty
# kernel = "build/kernel.elf"
extra_args = ["-serial", "mon:stdio"]

[qemu.debug]
enabled = true                        # default: start paused with a GDB stub
gdb_port = 1234                       # default

[gdb]                                 # optional
executable = "gdb"                    # default
architecture = "i8086"                # default
```

## Building

`pyxforge build <profile>` (or the Build tab) runs a profile's dependencies first, each once,
then the profile; it stops at the first tool that fails. Without a profile name it builds the
profiles nothing else depends on. Each tool runs in its `source_dir` with the profile's `env`
added to PyxForge's environment; `output_dir` is created first. A dependency cycle is reported
as `Circular dependency detected involving 'kernel'`.

Errors and warnings in the tools' output are read in two formats: `file:line[:column]: error|warning|note: message`
(NASM, GCC, Clang, ld) and Cargo's `--message-format=json`. Relative file names are relative to
the profile's `source_dir`.

## Validation

`pyxforge info` and `pyxforge build` exit with status 1 and prints the reason when the file is invalid:

| Rule | Message |
|---|---|
| `project.name` is empty | `project.name must not be empty` |
| A `depends_on` entry names no profile | `Profile 'kernel' depends on 'boot', which does not exist` |
| `[qemu]` has neither `boot_image` nor `kernel` | `Either qemu.boot_image or qemu.kernel must be specified inside [qemu]` |
| `boot_image` or `kernel` is present but empty | `qemu.boot_image must not be empty if configured` |
| An unknown GDB architecture | `gdb.architecture 'arm64' is invalid. Valid values: i8086, i386, i386:x86-64, auto, arm` |

Keys PyxForge does not know are ignored, as in 2.x, and listed as warnings by `pyxforge info`.

## Changes from 2.x

- `architecture = "arm"` is accepted. 2.x wrote it from its Embedded preset but rejected it, so
  those projects could not build or debug.
- Profiles keep the order they have in the file.
