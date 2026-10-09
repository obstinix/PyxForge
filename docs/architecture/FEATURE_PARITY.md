# PyxForge 3.0 Feature Parity Matrix

Every 2.x feature (see `FEATURE_INVENTORY.md`) and every new 3.0 commitment, with its 3.0 home.
Each row must end as **Complete**, **Deferred** (with reason) or **Dropped** (with reason) before
the Phase 8 gate. "Open" means not started. "2.x status" is what was verified on 2026-10-08.

## Ported 2.x features

| # | Existing feature | 2.x status | 3.0 subsystem | Implementation | Test | Phase | Status |
|---|---|---|---|---|---|---|---|
| 1 | `pyxforge.toml` schema, defaults, validation | Works | ConfigService (`internal/config`) | Go TOML decode + validation with the same messages | 16 Rust config tests as golden cases | 4 | **Complete**: `internal/config`; the 16 2.x tests pass as golden cases (`config_test.go`); `pyxforge info` reports it |
| 2 | ARM GDB architecture | Broken (rejected by validation) | ConfigService, GdbService | Accept `arm` (and `auto` per target) | New test: Embedded preset loads and resolves | 4 | **Complete**: `arm` accepted; `TestArmArchitecture` |
| 3 | Build profiles with `depends_on`, cycle detection, stop on first failure | Works | BuildService (`internal/build`) | Owned processes, streamed output, per-profile results | 5 Rust build tests + streaming test | 4 | Open |
| 4 | Diagnostics: GNU-style, Cargo JSON | Works (core) | DiagnosticsService | One parser package | 12 Rust diagnostics tests | 4 | Open |
| 5 | Diagnostics: rustc text, MSVC, GNU ld | Works (extension only) | DiagnosticsService | Same package as row 4 | 5 extension parsing tests as golden inputs | 4 | Open |
| 6 | Problems list and editor diagnostics | Works (X) / absent (D) | DiagnosticsService → Problems dock + Neovim `vim.diagnostic` | Push diagnostics into Neovim namespaces | UI test + Neovim integration test | 4 | Partial: Neovim diagnostics fill the Problems panel and open at their position (`problems.go`, `TestEditorTabsFollowNeovimBuffers`); build diagnostics pending |
| 7 | Nine build presets | Works except Embedded | BuildService (preset registry) | Data table in Go | One test per preset: writes valid TOML that loads | 4 | Open |
| 8 | Project scaffolding (assembly, Rust) | Broken (asm boot image) / suspect (Rust) | Project service (`internal/workspace`) | Fix `os-image.bin` via a profile; drop `.vscode/*`; verify Rust boot | Scaffold → build → QEMU boot integration test | 4–5 | Open |
| 9 | QEMU argument construction | Works | QemuService (`internal/qemu`) | Byte-for-byte port | 4 Rust qemu tests | 5 | Open |
| 10 | QEMU launch (debug/run) and stop | Works (X) | QemuService | Owned child with context, captured serial, reaped on exit | Integration: start, QMP status, stop; no orphan after exit | 5 | Open |
| 11 | QEMU status | Partial (process alive) | QemuService | Long-lived QMP with `query-status` and events | Fake QMP server unit tests + real QEMU integration | 5 | Open |
| 12 | QMP graceful shutdown | Works (untested) | QemuService | `system_powerdown`, timeout, then kill | Fake QMP server test | 5 | Open |
| 13 | Snapshots save/load/delete/list | Untested end-to-end | QemuService | HMP over QMP (or `snapshot-save` job API, decided in Phase 5) | Integration with real QEMU and a qcow2 disk | 5 | Open |
| 14 | HMP monitor console | Untested end-to-end | QEMU console panel | `human-monitor-command` over the live QMP session | Integration test | 5 | Open |
| 15 | GDB attach by architecture | Works (X, via third-party adapter) | GdbService (`internal/gdb`) | GDB/MI session owned by PyxForge | 4 Rust gdb tests + attach-to-QEMU integration | 5 | Open |
| 16 | Registers with change highlight, memory read | Works (X) / simulated (D) | GdbService + Inspector panel | `-data-list-register-values`, `-data-read-memory-bytes` | GDB/MI record parser tests + integration | 5 | Open |
| 17 | Disassembly context with PC highlight | Simulated (D) | GdbService / BinaryService | `-data-disassemble` or `llvm-objdump` | Integration test against a known boot sector | 5 | Open |
| 18 | Hex viewer, boot sector, `0xAA55` check | Works (X) / broken (D) | BinaryService (`internal/binary`) | Port `hex.rs` | 3 Rust hex tests | 5 | Open |
| 19 | PTY terminal | Works (D, one session) | TerminalService | Native PTY (ConPTY on Windows), named sessions | Spawn, write, resize, exit tests | 4 | Open |
| 20 | File tree, read, save, dirty state | Works (D) | Explorer + Neovim buffers | Filesystem service; Neovim owns buffers | UI test: open, edit, save | 3–4 | **Complete**: explorer opens files into Neovim buffers; tabs mark unsaved changes, Save writes, closing asks (`editorhost.go`, `TestEditorTabsFollowNeovimBuffers`) |
| 21 | Code editor | Works (D, CodeMirror 6) | EditorService (Neovim) | D4 | Phase 3 spike criteria | 3 | Partial: embedded Neovim editing passes the D4 spike (ADR 0006); PyxForge Lua configuration, Tree-sitter and LSP pending (N9) |
| 22 | Themes | Works (D, CSS) | Theme tokens (`internal/ui/theme`) | D2: five themes × accents + System | Theme switch UI test; contrast checks | 1–2 | **Complete**: `internal/ui/theme`, `TestContrast`, `TestPalettesAreDistinct`, `TestSelectionPersists` |
| 23 | AI: explain assembly, explain build error, explain CPU state | Works (X, `vscode.lm`) / simulated (D) | AgentService + editor actions | Provider interface, proposed change + explicit apply | Fake provider binary test | 6 | Open |
| 24 | JS plugin loader | Works, unsafe (D) | — | — | — | — | **Dropped**: JS is forbidden in 3.0 (Section 2.1); Lua via Neovim after Phase 5 (D9) |
| 25 | Stdio JSON protocol and its 13 missing-field tests | Works | — | — | — | — | **Dropped**: 3.0 calls services in-process; the envelope no longer exists |
| 26 | VS Code extension frontend | Works | — | moved to `legacy/` (D8) | — | 8 | Open (Dropped at parity gate) |
| 27 | Tauri desktop frontend | Partial | — | moved to `legacy/` (D8) | — | 8 | Open (Dropped at parity gate) |
| 28 | Anti-slop linter | Fails on `main` | Forbidden-technology check + design review | Go or shell script in CI and pre-commit (Section 16.4) | Script self-test with a planted violation | 1 | **Complete**: `tools/forbidcheck` (CI and `.githooks/pre-commit`), `TestCheck`; visual anti-patterns go through the design review |

## New in 3.0

| # | Feature | Subsystem | Phase | Status |
|---|---|---|---|---|
| N1 | Command palette with fuzzy search over real commands | `internal/ui/commandpalette` | 2 | **Complete**: palette over the command registry and workspace files, keyboard tests |
| N2 | Sector footprint (bytes of 512) from the build artifact | BuildService metrics | 4 | Open |
| N3 | 512-byte sector map | BinaryService | 5 | Open |
| N4 | ELF inspection (`debug/elf`) | BinaryService | 5 | Open |
| N5 | Named terminal sessions: shell, qemu-serial, gdb-server | TerminalService | 4–5 | Open |
| N6 | QEMU state, PID and uptime in the status bar from QMP | QemuService → StatusBar | 5 | Open |
| N7 | Local Git: status, diff, stage, commit, branch, log, stash, worktree | GitService | 4 | Open |
| N8 | Workspace persistence and restore | WorkspaceService | 4 | Open |
| N9 | Tree-sitter and LSP through Neovim | EditorService / `nvim/` | 3 | Open |
| N10 | Agents: providers, isolated worktrees, sessions, diff review | AgentService | 6 | Open |
| N11 | Agent debug loop with versioned context bundle | AgentService + Qemu + Gdb | 7 | Open |
| N12 | Theme–editor–terminal synchronization boundary | Theme + EditorService + TerminalService | 2–3 | Partial: the editor takes the theme as a Neovim colorscheme and follows theme and OS changes (`editor/colorscheme.go`, `TestEditorTabsFollowNeovimBuffers`); terminal pending |
| N13 | Offline test matrix | `tests/offline` | every milestone | Open |
