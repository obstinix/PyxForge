# PyxForge 2.x Feature Inventory

Every feature found in `main` at `9d273f5`, with where it lives and whether it was verified
on 2026-10-08. Status values:

- **Works**: code path is real and covered by a passing test or a direct code reading.
- **Partial**: real but incomplete in a way that matters for 3.0.
- **Simulated**: renders as if working but uses fabricated or hardcoded data.
- **Broken**: real code that cannot succeed as shipped.
- **Untested**: real code with no test and no end-to-end path a user can reach.

Frontends: **C** = core, **X** = VS Code extension, **D** = Tauri desktop.

## 1. Core protocol (`core/src/protocol.rs`, `core/src/main.rs`)

One JSON object per process on stdin, one JSON line on stdout, then exit. Envelope:
`{"cmd":"<camelCase>", <snake_case fields>}`. Success: `{"status":"ok","message",…,"data"}`;
error: `{"status":"error","message"}` with exit code 1.

| Command | Fields | Used by | Status |
|---|---|---|---|
| `ping` | none | X, D | Works (returns crate version) |
| `build` | `project_root`, `profile` | X, D | Works; diagnostics from last profile only |
| `listProfiles` | `project_root` | X, D | Works; sorted by name |
| `launch` | `project_root`, `debug?` (default true) | X | Works; QEMU detached with stdio to null |
| `stop` | `pid`, `project_root?` | X | Works; QMP `system_powerdown`, 2 s wait, then `taskkill /F` or `kill -9` |
| `qemuStatus` | `pid` | X (polling) | Partial: process-alive only, no QMP state |
| `debugConfig` | `project_root`, `profile?` | X | Works for i8086, i386, i386:x86-64; `auto` maps to i8086 |
| `init` | `project_root`, `project_name`, `template?` | X, D | Partial: see scaffolding |
| `hexDump` | `file_path` | X | Works; D calls it with the wrong envelope (Broken in D) |
| `qemuSnapshotSave/Load/Delete/List` | `project_root`, `tag` | D | Untested: D cannot launch QEMU with the QMP socket these need |
| `qemuMonitorCommand` | `project_root`, `command` | D | Untested, same reason |

## 2. Configuration (`core/src/config.rs`)

| Feature | Status | Notes |
|---|---|---|
| `[project]` name (required, non-empty), description | Works | |
| `[profiles.<name>]`: `tool`, `description`, `source_dir` (default "."), `output_dir` (default "build", created if missing), `args`, `env`, `depends_on`, `[profiles.<name>.gdb]` overrides | Works | 16 tests |
| `[qemu]`: `executable` (default `qemu-system-x86_64`), `machine` ("pc"), `memory` ("128M"), `boot_image` or `kernel` (one required), `extra_args`, `[qemu.debug]` `enabled` (true), `gdb_port` (1234) | Works | |
| `[gdb]`: `executable` ("gdb"), `architecture` ("i8086") | Partial | Valid values are only `i8086`, `i386`, `i386:x86-64`, `auto` (`config.rs:162`). ARM is rejected. |

## 3. Build and diagnostics

| Feature | Where | Status | Notes |
|---|---|---|---|
| Profile execution in `depends_on` order, cycle detection, stop on first failure | C `build.rs` | Works | 5 tests |
| GNU-style diagnostics (`file:line[:col]: error|warning|note|fatal error:`), Windows drive letters | C `diagnostics.rs` | Works | 6 tests |
| Cargo/rustc JSON diagnostics (primary span) | C `diagnostics.rs`, X `diagnostics.ts` | Works | C 4 tests, X 1 test |
| rustc human-readable (`error[E0425]:` then `--> file:l:c`) | X only | Works | X test `extension.test.ts:125` |
| MSVC `file(line) : error C2143:` and `file : error LNK2019:` | X only | Works | X test `extension.test.ts:97` |
| GNU ld `undefined reference`, `multiple definition` | X only | Works | X test `extension.test.ts:193` |
| Problems panel and gutter markers | X (VS Code), D none | Partial | D shows no diagnostics UI |

## 4. Presets and scaffolding

Presets live only in the extension (`extension/src/presets.ts`); each writes a `pyxforge.toml`.

| Preset | Tool | QEMU | GDB arch | Status |
|---|---|---|---|---|
| Bootloader | nasm | x86_64 pc | i8086 | Works |
| Kernel Debug | gcc | x86_64 pc | i386:x86-64 | Works (config only) |
| Kernel Release | gcc | x86_64 pc | i386:x86-64 | Works (config only) |
| Rust Application | cargo | x86_64 pc | i386:x86-64 | Works (config only) |
| C Application | gcc | x86_64 pc | auto | Works (config only) |
| C++ Application | g++ | x86_64 pc | auto | Works (config only) |
| Embedded | arm-none-eabi-gcc | qemu-system-arm lm3s6965evb | arm (gdb-multiarch) | **Broken**: `arm` fails core validation |
| Bare Metal | nasm | x86_64 pc | i386 | Works (config only) |
| Custom | make | — | — | Works (skeleton) |

Scaffold templates live in the core (`core/src/scaffold.rs`) and also write `.vscode/tasks.json`
and `.vscode/launch.json`.

| Template | Status | Notes |
|---|---|---|
| `assembly` (default): 2-stage BIOS loader, `boot.asm`, `kernel.asm`, Makefile | **Broken** after build | `boot_image = "build/os-image.bin"` is produced only by the Makefile, not by any profile. |
| `rust`: `no_std` kernel, multiboot header, `linker.ld`, `-kernel` boot | Suspect | Targets `x86_64-unknown-none` (ELF64) and boots with QEMU `-kernel`, whose multiboot loader expects a 32-bit image. Verify in Phase 5. |

## 5. QEMU

| Feature | Where | Status |
|---|---|---|
| Launch paused with GDB stub (`-s -S` or `-gdb tcp::N -S`) or free-run | C, X | Works |
| QMP endpoint on every launch (`unix:<root>/build/qemu-qmp.sock` or `tcp:127.0.0.1:<gdb_port+1>`) | C | Works (arg construction tested; socket I/O untested) |
| Graceful stop over QMP with kill fallback | C, X | Works (no test) |
| Status polling | C, X | Partial (process alive only) |
| Snapshots save/load/delete/list via HMP over QMP | C, D | Untested end-to-end |
| HMP monitor console | C, D | Untested end-to-end |
| Serial console capture | none | Absent: QEMU stdio goes to null |

## 6. Debugging and inspection

| Feature | Where | Status | Notes |
|---|---|---|---|
| GDB attach with architecture set (`set architecture …`) | C `gdb.rs` + X via `webfreak.debug` Native Debug adapter | Works for x86 modes | Requires the Native Debug VS Code extension. |
| Register view with change highlight | X `inspectorPanel.ts` (DAP `variables`) | Works | |
| Register view, flags, stack, disassembly "pwndbg-inspired" | D | **Simulated** | `main.ts:95`, `main.ts:809-838`, `index.html:179` |
| Memory read at address | X (`readMemory` → DAP `evaluate`) | Works | |
| Hex viewer with boot-sector and `0xAA55` callout | C + X `hexPanel.ts` | Works | |
| Hex viewer for binary files in desktop | D | **Broken** | wrong RPC envelope, `main.ts:603` |

## 7. AI

| Feature | Where | Status | Notes |
|---|---|---|---|
| Explain selected assembly | X `aiHelper.ts` | Works when a VS Code chat model is available | Tries `gemini-1.5-pro`, then `copilot-gpt-4o`, then any model (`aiHelper.ts:55-60`). |
| Explain build error | X | Works under the same condition | |
| Explain CPU state | X inspector button | Works under the same condition | |
| Explain CPU state | D | **Simulated** | two hardcoded log lines, `main.ts:842` |

## 8. Desktop shell (Tauri)

| Feature | Status | Notes |
|---|---|---|
| CodeMirror 6 editor for asm, C, JSON, TOML; `Ctrl+S`; dirty marker | Works | Replaced by Neovim in 3.0 (D4). |
| File tree from `list_workspace_files` | Works | |
| PTY terminal (one session, `portable-pty` + xterm.js) | Works | |
| Build profile list and run | Works | No diagnostics UI. |
| Project init | Partial | inherits scaffold defects |
| Theme selector: `auto`, `mono`, `contrast`, `hybrid`, `ink-and-paper` | Works (wired via `var(--…)`) | `ink-and-paper.css` violates the repo's own rules. |
| JS plugin loader | Works, unsafe | `new Function("ctx", code)` on any file path, `main.ts:504`. Dropped in 3.0 (D9). |
| Runtime Google Fonts | Present | `index.html:8-11`; breaks offline use. |

## 9. Tooling and process

| Item | Status |
|---|---|
| CI matrix (Ubuntu, macOS, Windows): core build, test, clippy, fmt, doc; extension type-check, lint, build, headless tests | Works |
| Desktop app in CI | Absent |
| `scripts/lint-slop.sh` anti-slop linter | Fails on `main`; not run in CI |
| `docs/antislop.md` visual and functional slop rules | Useful; its functional-slop rules carry into 3.0 (Section 22 of the master prompt) |
