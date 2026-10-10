# PyxForge 2.x Architecture (legacy)

How 2.x is built, what each part owns, and what carries into the Go rewrite (D1). Facts here are
from the code at `main` `9d273f5`; see `../archive/2.x/CURRENT_STATE-2026-10-08.md` for where the docs and the code disagree.

## Process model

```text
 VS Code extension (TypeScript)            Tauri desktop (TypeScript + Rust)
 ┌──────────────────────────────┐          ┌───────────────────────────────────┐
 │ commands, webview panels,    │          │ webview: CodeMirror 6, xterm.js,   │
 │ DiagnosticCollection, DAP    │          │ panels (inspector is simulated)    │
 │ via webfreak.debug, vscode.lm│          ├───────────────────────────────────┤
 └──────────────┬───────────────┘          │ src-tauri: call_core, fs RPCs,     │
                │                          │ portable-pty, read_plugin_file     │
                │ spawn per request        └──────────────┬────────────────────┘
                ▼                                         ▼ spawn per request
        ┌──────────────────────────────────────────────────────────┐
        │ pyxforge-core (Rust): read one JSON line, act, print one   │
        │ JSON line, exit. No long-lived state.                      │
        │ build · config · diagnostics · gdb · hex · qemu · qmp ·    │
        │ scaffold · protocol                                        │
        └───────┬──────────────────────┬──────────────────┬─────────┘
                │ Command::output      │ spawn detached   │ TCP/Unix socket
                ▼                      ▼                  ▼
          nasm/gcc/cargo/…        qemu-system-*  ◄──── QMP (per request)
                                      ▲
                                      │ GDB remote (:1234)
                         gdb via VS Code Native Debug adapter (extension only)
```

Consequences of the per-request model:

- QEMU is a detached orphan. The frontends remember its PID; nothing owns it, nothing reaps it
  if the frontend crashes, and its stdout/stderr go to null, so the serial console is lost.
- Every QMP operation reconnects and renegotiates capabilities. Async QMP events (`STOP`,
  `RESUME`, `SHUTDOWN`) are read and discarded (`qmp.rs:214-217`), so no frontend can react to a
  guest pause, a crash or a triple-fault reset.
- GDB is never driven by PyxForge itself. The extension hands a launch config to a third-party
  debug adapter; the desktop has no debugger path at all.

## Core modules and the Go port

| Rust module | Lines | Responsibility | Tests | 3.0 Go home | Port notes |
|---|---|---|---|---|---|
| `protocol.rs` | 190 | Request enum (`cmd` tag, camelCase), response envelopes, `DiagnosticEntry`, `BuildResultData`, `QemuLaunchData`, `DebugConfigData` | 0 | `pkg/model` | Becomes plain Go types. The stdio envelope disappears: 3.0 calls services in-process. |
| `main.rs` | 554 | Dispatch, per-command handlers, stdin/stdout entry | 13 | `internal/app` (no port) | The 13 tests check missing-field errors of the stdio protocol; they become irrelevant once the protocol goes. Record as Dropped with reason in the parity matrix. |
| `config.rs` | 598 | `pyxforge.toml` schema, defaults, validation | 16 | `internal/config` | Port every default and every validation message. Add ARM architectures (fixes the Embedded preset). Schema must stay compatible (Section 10.2). |
| `build.rs` | 292 | DFS build order, cycle detection, run tool, stop on failure | 5 | `internal/build` | Stream output instead of buffering; keep results for every profile, not only the last. |
| `diagnostics.rs` | 410 | GNU-style and Cargo JSON parsers | 12 | `internal/diagnostics` | Merge with the extension's rustc-text, MSVC and GNU ld parsers and their test inputs. |
| `qemu.rs` | 347 | QMP address, QEMU args, detached launch, stop, alive check | 4 | `internal/qemu` | Keep arg construction byte-for-byte (tests). Replace detached launch with an owned child process with a cancellation context and captured serial. |
| `qmp.rs` | 224 | QMP client: greeting, `qmp_capabilities`, `system_powerdown`, `query-status`, `human-monitor-command` | 0 | `internal/qemu` | Make the connection long-lived and deliver events. Add tests against a fake QMP server. |
| `gdb.rs` | 115 | GDB launch config (`set architecture`, `:port`) | 4 | `internal/gdb` | Grows into a GDB/MI session (Section 13.4); the 4 tests still pin architecture resolution. |
| `hex.rs` | 125 | 16-byte rows, ASCII, 512-byte boot sector, `0x55 0xAA` | 3 | `internal/binary` | Add the sector map and ELF via `debug/elf`. |
| `scaffold.rs` | 450 | Assembly and Rust templates, `.vscode/*` | 2 | `internal/workspace` (project service) | Fix the `os-image.bin` defect; drop `.vscode/*` output; verify the Rust template boots. |

Windows build quirk: `core/.cargo/config.toml` adds `-lunwind` for `x86_64-pc-windows-gnu`, and
`core/libgcc-compat/` holds stub `libgcc.a`/`libgcc_eh.a` archives with a `linker-wrapper.bat`.
None of this carries over; it is noted only so the Rust tests can still be run as the oracle.

## Extension (`extension/`, TypeScript, VS Code `^1.125.0`)

- `extension.ts` (37 KB): command registration, core spawning (resolves the binary relative to
  `__dirname/../core/target/debug/`), QEMU status polling, DAP queries for the inspector.
- `diagnostics.ts`: the most complete diagnostics parser in the repo (see inventory §3).
- `presets.ts`: the nine presets.
- `inspectorPanel.ts`, `hexPanel.ts`, `aiPanel.ts`: webview panels with an embedded HTML/CSS UI
  (Catppuccin-derived palette, `#cba6f7` accent).
- `aiHelper.ts`: `vscode.lm` model selection and streaming.
- Tests: 10 extension-host tests (`src/test/extension.test.ts`).

Carry-over: the diagnostics parser and its fixtures, the preset definitions, the DAP-based
inspector's *data model* (registers, change flags, memory windows). Nothing else.

## Desktop (`desktop/`, Tauri 2)

- `src-tauri/src/lib.rs`: Tauri commands `call_core`, `spawn_pty`, `write_to_pty`,
  `resize_pty`, `read_plugin_file`, `list_workspace_files`, `read_workspace_file`,
  `write_workspace_file`, plus the scaffold's `greet`.
- `src/main.ts` (38 KB): one file holding all UI logic: status, profiles, build, snapshots,
  monitor, plugin loader, file tree, hex, simulated inspector, PTY wiring.
- `src/editor.ts`: CodeMirror 6 setup.
- `src/styles.css`, `src/tokens.css`, `src/themes/*.css`: token-driven styles and five themes.

Carry-over: nothing as code. Interaction lessons only (see below).

## Prior native attempt

`origin/feat/native-rust-stitch-frontend` (2026-08-24, unmerged) replaced Tauri with egui 0.29 /
eframe (glow backend, `syntect` highlighting, `rfd` file dialogs). It split the core into a
library (`core/src/lib.rs`, 419 lines) so the UI called it in-process, and organized views as
`views/{workspace,build,debug,hex,qemu,scaffold,theme_gallery}.rs` with a `state.rs` and a
`theme.rs`. It shows that the in-process service model is the right direction; D1 moves that
model to Go.

## Lessons for 3.0

1. **Own every child process.** QEMU and GDB must be children of a service with a context, not
   detached orphans, or the status bar can never be truthful.
2. **Keep sessions open.** QMP and GDB/MI are stateful protocols; a per-request process model
   throws away the events 3.0 needs (pause, breakpoint hit, crash).
3. **One parser, one place.** Diagnostics drifted into two implementations with different
   coverage. 3.0 has one Diagnostics service fed by golden fixtures from both.
4. **CI must build what ships and run the slop check.** The desktop app and `lint-slop.sh` were
   both outside CI, and both regressed.
5. **Functional slop is the bigger risk.** The simulated inspector survived a checkpoint review
   that called it "ported". 3.0 placeholders must be labeled on screen (Section 17, Phase 2).
