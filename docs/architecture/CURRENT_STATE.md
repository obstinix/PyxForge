# PyxForge Current State (verified 2026-10-08)

This is the state of `obstinix/PyxForge` `main` at `9d273f5` ("feat(desktop): set ink-and-paper
as default launch theme in main.ts", 2026-07-26), checked against the code, not the README.
The local snapshot `PyxForge-main (1)/` is byte-identical to `main` except for CRLF line endings.

## Summary

PyxForge 2.x is a Rust core (`pyxforge-core`) driven per request over a one-line JSON protocol on
stdio, with two frontends: a VS Code extension and a Tauri 2 desktop shell. The core is small,
tested and honest. The extension is the only frontend with a real debugger path. The desktop shell,
which the 2026-07-19 checkpoint made "the primary product", cannot launch QEMU, has no GDB
connection, and fabricates its CPU inspector data.

## What is verified working

| Area | Evidence |
|---|---|
| Core test suite | `cargo test` in `core/`: **59 passed, 0 failed** (toolchain `stable-x86_64-pc-windows-gnu`, this laptop, 2026-10-08). Counts per module: main 13, config 16, diagnostics 12, build 5, gdb 4, qemu 4, hex 3, scaffold 2, qmp 0, protocol 0. |
| `pyxforge.toml` parsing and validation | `core/src/config.rs:171-274`, 16 tests. |
| Build profile ordering with `depends_on`, cycle detection, stop on first failure | `core/src/build.rs:34-104`. |
| QEMU argument construction (`-s`/`-gdb tcp::N`, `-S`, `-qmp unix:` or `tcp:127.0.0.1:gdb_port+1`) | `core/src/qemu.rs:24-74`, 4 tests. |
| QMP handshake, `system_powerdown`, `human-monitor-command` | `core/src/qmp.rs:94-182`. Snapshots are HMP `savevm`/`loadvm`/`delvm`/`info snapshots` over QMP (`core/src/main.rs:53-64`). |
| Hex dump with 512-byte boot-sector and `0x55 0xAA` check | `core/src/hex.rs:24-63`, 3 tests. |
| Extension GDB attach and live inspector | DAP requests through the Native Debug adapter: `threads`, `stackTrace`, `scopes`, `variables`, `evaluate` (`extension/src/extension.ts:862-940`). |
| Extension diagnostics for rustc text, Cargo JSON, GCC/Clang, MSVC, GNU ld | `extension/src/diagnostics.ts:17-222`, tests at `extension/src/test/extension.test.ts:67-217`. |
| Desktop PTY terminal | `portable-pty` in `desktop/src-tauri/src/lib.rs` (`spawn_pty`, `write_to_pty`, `resize_pty`), rendered by xterm.js. One session. |
| Desktop file tree, read, save | Tauri commands `list_workspace_files`, `read_workspace_file`, `write_workspace_file`. |

## What is claimed but not true

| Claim (source) | Reality | Evidence |
|---|---|---|
| Desktop CPU inspector is "ported" from the extension (Checkpoint 1, `docs/architecture/CHECKPOINTS.md`) | Register values are fabricated by a "Simulate CPU Step" button; disassembly comes from a mock instruction table. No GDB connection exists in the desktop app. | `desktop/index.html:179`, `desktop/src/main.ts:95`, `main.ts:809-838`; `main.ts` has no reference to GDB. ADR 0003's own "Honesty Check" said the panels were not ported. |
| Desktop falls back to the `HexDump` RPC for binary files (README, ADR 0004) | The desktop sends `{"jsonrpc":"2.0","method":"HexDump",...}` and reads `res.result.dump`; the core only accepts `{"cmd":"hexDump","file_path":...}` and returns `data.lines`. The call always fails. | `desktop/src/main.ts:603-607` vs `core/src/protocol.rs:10-47`. |
| Desktop "Explain CPU" (README) | Prints two hardcoded log lines. | `desktop/src/main.ts:842-845`. |
| QEMU "launch, stop, status polling" in Desktop (README Quick Start) | The desktop never sends `launch`, `stop`, `qemuStatus` or `debugConfig`. Snapshot and monitor panels only work against a QEMU started elsewhere. | `desktop/src/main.ts:300-453` (the nine commands it does send). |
| QEMU status | `qemuStatus` is a process-alive check (`tasklist` / `kill -0`), not QMP state. `QmpClient::query_status` exists but is dead code. | `core/src/qemu.rs:169-190`, `core/src/qmp.rs:123`. |
| GDB attach for ARM via `gdb-multiarch` (README) | The Embedded preset writes `architecture = "arm"`, which core validation rejects, so every core command fails for that project. | `extension/src/presets.ts:229`, `core/src/config.rs:162`. |
| Core parses rustc text, MSVC and GNU ld (README "Build & Diagnostics") | Core parses only GNU-style `file:line[:col]: severity:` lines and Cargo JSON. The other formats are parsed only in the extension. | `core/src/diagnostics.rs:20-273`. |
| Phase 21 removed gradients, `backdrop-filter`, off-brand accents and runtime Google Fonts | The repo's own `scripts/lint-slop.sh` **fails on `main`**: `#cba6f7` in `themes/hybrid.css:14`, two `radial-gradient` and two `backdrop-filter` in `themes/ink-and-paper.css`. `desktop/index.html:8-11` loads Caveat, JetBrains Mono, Unbounded, Yellowtail and Material Symbols from `fonts.googleapis.com` at runtime. Emoji icons are gone from the desktop shell but remain in the extension (`✨` in `extension/src/aiPanel.ts:215`, `extension/src/inspectorPanel.ts:426`) and the sample plugin (`desktop/plugins/optimizer-plugin.js:6`). | `bash scripts/lint-slop.sh` run 2026-10-08, exit 1. |
| Assembly scaffold boots after build | `[qemu].boot_image = "build/os-image.bin"`, but no profile produces that file (only the generated Makefile `cat`s it). Build then launch fails. | `core/src/scaffold.rs:216-234`. |
| Theme switcher "wired to nothing" (master prompt Section 5) | Outdated: `styles.css` now has 81 `var(--` references. The finding applied before Phase 21. | grep of `desktop/src/styles.css`. |
| Tauri scaffold defaults (master prompt Section 5) | Mostly fixed: `productName` and window title are "PyxForge", window 1280x800, authors `["obstinix"]`. Remaining: crate name `desktop`, identifier `com.piyush.desktop`. | `desktop/src-tauri/tauri.conf.json`, `desktop/src-tauri/Cargo.toml`. |

## Other observations

- CI (`.github/workflows/ci.yml`) builds and tests the core and the extension on Ubuntu, macOS and
  Windows. It never builds the desktop app and never runs `lint-slop.sh`, which is how both
  regressions above went unnoticed.
- The core is invoked once per request and exits; nothing in it holds a QEMU or GDB session.
- Branch `feat/native-rust-stitch-frontend` (2026-08-24, unmerged, one commit, 96 files) replaced
  the Tauri desktop with an egui 0.29 / eframe frontend and turned the core into a library crate
  (`core/src/lib.rs`). It is prior art for decoupling the core; its UI stack is not the 3.0 stack.
- `docs/architecture/CHECKPOINTS.md` records "Antigravity AI & obstinix" as authors. 3.0 commits are
  authored only as `obstinix` (D7).
- The repo includes `pyforge_ui_kit/_unidentified/`, which holds five third-party images (a
  typographic poster, a cat illustration carrying another studio's mark, an editorial layout with
  photographs of people, two stock doodle sets) committed to a public Apache-2.0 repository. Only
  `gemini_generated_image_…png` (a PyxForge logo concept) is plausibly the owner's. See
  `docs/design/REFERENCE_UI_ANALYSIS.md`.

## Development environment (this laptop, 2026-10-08)

| Tool | Status | Needed for |
|---|---|---|
| Node 22.23.1, npm 10.9.8 | present | design tools only (never the app) |
| Chrome 154 | present | Chrome DevTools MCP, Impeccable URL scans |
| git 2.55 | present | everything |
| Rust stable (gnu, msvc), nightly msvc | present | running the 2.x core tests as the parity oracle |
| LLVM-MinGW (clang, gcc driver, llvm-objdump) | present | cgo for Fyne on Windows; disassembly |
| WSL2 Ubuntu | installed, stopped | Linux path (D5) |
| **Go** | **missing** | the entire 3.0 app (D1) |
| **Neovim** | **missing** | editor engine (D4) |
| **QEMU** (`qemu-system-x86_64`, `-i386`, `-arm`) | **missing** | Phase 5, integration tests |
| **GDB / gdb-multiarch** | **missing** | Phase 5 |
| **NASM** | **missing** | bootloader presets, Phase 4 build tests |
| make | missing | Custom preset, scaffold Makefile |
| gh | missing | PR workflow (optional) |
| `claude` CLI on PATH | missing (bundled copy at `%APPDATA%\Claude\claude-code\2.1.293\…\claude.exe`) | `claude mcp` management |
