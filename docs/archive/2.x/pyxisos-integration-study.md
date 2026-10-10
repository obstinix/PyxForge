# PyxisOS Integration Study & Developer Guide

This document presents a comprehensive integration and feature alignment study between **PyxForge** (the developer platform) and **PyxisOS** (the sibling operating system project).

---

## 1. Compiling PyxisOS with PyxForge Build Presets

PyxisOS's native OS kernel (`lunar-core`) is a freestanding, bare-metal `#![no_std]` Rust application. It defines a custom JSON target specification: `x86_64-pyxis.json`.

*Note: This target configuration excludes standard library dependencies (`no_std`) and specifies a custom memory layout tailored for virtualized x86_64 target systems.*

### How PyxForge compiles PyxisOS
PyxForge's `cargo` tool builder is natively equipped to compile custom freestanding Rust targets.
1. The **Rust Application** or a custom profile preset can be configured to invoke Cargo.
2. Under the hood, PyxForge compiles the project using:
   ```bash
   cargo build --target x86_64-pyxis.json --manifest-path native/lunar-core/Cargo.toml
   ```
3. The build output parser in PyxForge parses the compiler diagnostic messages (Rustc human-readable or Cargo JSON formats) and automatically maps compile/linker errors into the VS Code Problems panel, highlighting lines in files like `src/main.rs` or `src/memory/mod.rs`.

---

## 2. Launching and Debugging PyxisOS via PyxForge

PyxisOS compiles into an ELF executable (specifically `target/x86_64-pyxis/debug/lunar-core`). PyxForge manages the entire emulator and debugger lifecycle.

### QEMU Execution
PyxForge passes the compiled kernel to the QEMU process manager.
- **Boot Options**: Using the `kernel` property in `pyxforge.toml`, the guest virtual machine is launched with `-kernel target/x86_64-pyxis/debug/lunar-core`.
- **Status and Control**: PyxForge monitors the VM's PID, polls its live status via TCP/Unix sockets using the QMP (QEMU Monitor Protocol), and stops the VM gracefully using query/powerdown commands.

### Architecture-Aware GDB Attachment
- Since PyxisOS runs in 64-bit Long Mode (`x86_64`), the GDB config in PyxForge resolves the architecture to `i386:x86-64` or `auto`.
- When a debug session starts, PyxForge automatically triggers the GDB attach sequence, halts execution, verifies the QEMU-gdbstub connection using the `Qqemu.sstepbits` packet, and presents register/flag states inside the **CPU Inspector Webview**.

---

## 3. PyxisOS Feature Alignment & Mapping

We map PyxisOS's architectural features directly against PyxForge's developer tooling capabilities:

| PyxisOS Module | Subsystem Details | PyxForge Architectural Capability |
|---|---|---|
| **Abstract Boot Interface** | `BootInfo` struct defining physical address ranges of the memory map. | **Hex Explorer**: Allows developers to examine physical memory map binaries and boot sector sectors to verify offset correctness. |
| **CPU Architecture Abstraction** | `CpuArch` trait and `X86_64` implementation for CPU-specific structures (GDT, IDT, Paging). | **Inspector Panel & Flags**: Translates CPU register states (`eflags`/`rflags`) into flag status badges (e.g. Interrupt Flag IF, Zero Flag ZF). |
| **Memory Allocator Interface** | `MemoryAllocator` trait defining abstract `allocate` and `deallocate` methods. | **Memory Viewer**: Inspects specific stack/heap addresses by querying custom memory dumps directly via GDB during breakpoints. |
| **Panic and Loggers** | Custom `#![no_std]` panic handler and lightweight console output logger. | **Diagnostic Pipeline & Output**: Aggregates output logs and translates compiler/linker warnings directly to editor lines. |

---

## 4. Tutorial: Configuring PyxisOS in `pyxforge.toml`

To build and debug PyxisOS with PyxForge, create a `pyxforge.toml` configuration at the root of the workspace.

### Step-by-Step Configuration

1. **Define the Project Metadata**:
   Configure the project name to match the target.

2. **Add the Build Profile**:
   Point the profile `source_dir` to `native/lunar-core` and specify the custom target args.

3. **Configure the QEMU Emulator**:
   Point QEMU to load the compiled kernel ELF and enable the debugger stub.

4. **Configure GDB**:
   Specify the GDB executable and target architecture override.

### Copy-Pasteable `pyxforge.toml`

```toml
[project]
name = "pyxisos-lunar-core"
description = "Freestanding Rust kernel build configuration for PyxisOS"

[profiles.lunar-core-debug]
tool = "cargo"
description = "Build PyxisOS kernel in debug mode with custom target"
source_dir = "native/lunar-core"
output_dir = "native/lunar-core/target"
args = ["build", "--target", "x86_64-pyxis.json"]

[qemu]
executable = "qemu-system-x86_64"
machine = "pc"
memory = "128M"
kernel = "native/lunar-core/target/x86_64-pyxis/debug/lunar-core"

[qemu.debug]
enabled = true
gdb_port = 1234

[gdb]
executable = "gdb-multiarch"
architecture = "i386:x86-64"
```

---

## 5. Cross-Project Relationship Summary

PyxForge acts as the host-level compiler wrapper and emulator runtime controller for PyxisOS. This decoupled setup ensures PyxisOS remains an independent, lightweight guest operating system without any direct coupling to VS Code API logic.
