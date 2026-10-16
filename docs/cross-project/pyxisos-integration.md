# Building, running and debugging PyxisOS with PyxForge

PyxisOS (the sibling operating-system project) builds with its own Makefile: Clang and ld.lld for
the C and assembly parts, Cargo for the Rust kernel core (`x86_64-unknown-none`), linked into an
x86-64 ELF kernel with a Multiboot header. This page records what was verified with PyxForge 3.0
and PyxisOS v7 on 2026-10-11 (Windows 11, QEMU 11.1, GDB 17.2, LLVM-MinGW 22), on a copy of the
PyxisOS checkout. The 2.x plan this page replaces is in
[`archive/2.x/pyxisos-integration-study.md`](../archive/2.x/pyxisos-integration-study.md).

## `pyxforge.toml` for PyxisOS

```toml
[project]
name = "pyxisos"
description = "PyxisOS v7 kernel, built with its own Makefile"

[profiles.kernel]
tool = "mingw32-make"          # "make" on Linux
description = "make build: assembles, compiles and links bin/pyxis-kernel.elf"
args = ["build"]

[profiles.boot-elf]
tool = "llvm-objcopy"          # or GNU objcopy
description = "QEMU's -kernel loader takes 32-bit ELF: the Multiboot entry is 32-bit code"
args = ["-I", "elf64-x86-64", "-O", "elf32-i386", "bin/pyxis-kernel.elf", "bin/pyxis-kernel32.elf"]
depends_on = ["kernel"]

[qemu]
memory = "128M"
kernel = "bin/pyxis-kernel32.elf"
extra_args = ["-serial", "stdio", "-display", "none", "-no-reboot", "-no-shutdown"]

[gdb]
architecture = "i386:x86-64"
```

## What works

| Step | Result |
|---|---|
| `pyxforge build` | Runs `make build` in the project, streams its output (CC, AS, CARGO, LD lines), and reports success; compiler errors would be listed in Problems with their file and line |
| `pyxforge inspect bin/pyxis-kernel.elf --disasm` | ELF64, entry 0x100028, sections, segments and symbols (`_start`, `long_mode_start`, `kernel_main`…). PyxForge notes the Multiboot header and that the entry runs in 32-bit mode; `--arch i386` decodes it correctly (`mov dword ptr [0x13f000], eax`) |
| `pyxforge run` | Builds both profiles, boots the 32-bit copy with `-kernel`, and prints the serial console: the PyxisOS banner, GDT and IDT, memory manager, paging, heap, timer, keyboard, file system and Rust core, then the shell prompt |
| Debugging | With QEMU started by `pyxforge run --debug`, GDB loads the symbols of `bin/pyxis-kernel.elf` and a hardware breakpoint stops at `kernel_main`, with a backtrace through `long_mode_start` |
| Debug in QEMU in the desktop app (2026-10-16) | Builds both profiles, boots paused, attaches GDB and stops at the Multiboot entry 0x100028 with the inspector in protected mode (`eax`, `mov dword ptr [0x13f000], eax`); `hbreak kernel_main` typed in the GDB tab and Continue stop at 0x1018c0 in long mode, `bt` names `kernel_main`. Driven through the shell's own Debug, GDB-console and inspector code, on Windows and on Ubuntu 24.04 (GDB 15, QEMU 8.2) |
| Linux with GNU `objcopy` (2026-10-16) | The `boot-elf` profile with `tool = "objcopy"` makes the 32-bit copy, and `pyxforge run` boots it to the PyxisOS banner; the kernel ELF was built on Windows |

## Why the extra profile

QEMU's built-in Multiboot loader (`-kernel`) accepts only 32-bit ELF files and refuses an x86-64
one ("Cannot load x86-64 image, give a 32bit one"). PyxisOS's own `scripts/run.sh` passes the
64-bit ELF and fails the same way with QEMU 11. Because the Multiboot entry is 32-bit code that
switches to long mode itself, a class-converted copy (`objcopy -O elf32-i386`) boots unchanged.
A GRUB-based ISO would also work and would replace the `boot-elf` profile.

## Debugging notes

- Use the 64-bit ELF (`bin/pyxis-kernel.elf`) for symbols; the converted copy is only for QEMU's
  loader.
- A hardware breakpoint (`hbreak kernel_main`, typed in the GDB tab) is what was verified for
  code QEMU's loader copies into memory after reset.
- Set `[gdb] architecture` to `i386:x86-64` so the inspector shows 64-bit registers once the
  kernel is in long mode.

Before 2026-10-16 the inspector decoded the entry as 64-bit code, the mode `[gdb] architecture`
implies; it now follows the processor's CR0 and EFER.

## Not verified

- Building PyxisOS on Linux with GNU Make: the machine used had no Clang, ld.lld or nightly Rust,
  so only the boot and debug steps above ran on Linux, from a kernel built on Windows.
- PyxisOS's own Rust compiler errors in Problems: its builds had none to show. PyxForge reads
  rustc's and Cargo's human-readable output (`TestRealCompilerDiagnostics` with rustc 1.97), which
  is what PyxisOS's Makefile prints.
- The desktop flow above was driven through the shell's code, not by clicking in a window.
