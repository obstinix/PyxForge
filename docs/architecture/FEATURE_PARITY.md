# PyxForge 3.0 Feature Parity Matrix

Every 2.x feature (see `FEATURE_INVENTORY.md`) and every new 3.0 commitment, with its 3.0 home.
Each row must end as **Complete**, **Deferred** (with reason) or **Dropped** (with reason) before
the Phase 8 gate. "Open" means not started. "2.x status" is what was verified on 2026-10-08.

## Ported 2.x features

| # | Existing feature | 2.x status | 3.0 subsystem | Implementation | Test | Phase | Status |
|---|---|---|---|---|---|---|---|
| 1 | `pyxforge.toml` schema, defaults, validation | Works | ConfigService (`internal/config`) | Go TOML decode + validation with the same messages | 16 Rust config tests as golden cases | 4 | **Complete**: `internal/config`; the 16 2.x tests pass as golden cases (`config_test.go`); `pyxforge info` reports it |
| 2 | ARM GDB architecture | Broken (rejected by validation) | ConfigService, GdbService | Accept `arm` (and `auto` per target) | New test: Embedded preset loads and resolves | 4 | **Complete**: `arm` accepted; `TestArmArchitecture` |
| 3 | Build profiles with `depends_on`, cycle detection, stop on first failure | Works | BuildService (`internal/build`) | Owned processes, streamed output, per-profile results | 5 Rust build tests + streaming test | 4 | **Complete**: `internal/build` orders profiles depth-first with cycle detection, streams output, stops at the first failure and stops whole process trees on cancel; `pyxforge build` and the Build tab share it (`build_test.go`, `TestBuildPanel`, `cli/build_test.go`) |
| 4 | Diagnostics: GNU-style, Cargo JSON | Works (core) | DiagnosticsService | One parser package | 12 Rust diagnostics tests | 4 | **Complete**: all 12 Rust diagnostics tests ported (`internal/build/diagnostics_test.go`), plus Windows paths and GCC context lines |
| 5 | Diagnostics: rustc text, MSVC, GNU ld | Works (extension only) | DiagnosticsService | Same package as row 4 | 5 extension parsing tests as golden inputs | 4 | Open |
| 6 | Problems list and editor diagnostics | Works (X) / absent (D) | DiagnosticsService → Problems dock + Neovim `vim.diagnostic` | Push diagnostics into Neovim namespaces | UI test + Neovim integration test | 4 | **Complete**: Neovim and build diagnostics fill one Problems list, errors first, and open at their position (`problems.go`, `TestEditorTabsFollowNeovimBuffers`, `TestBuildPanel`); build errors are not yet drawn inline in the editor |
| 7 | Nine build presets | Works except Embedded | BuildService (preset registry) | Data table in Go | One test per preset: writes valid TOML that loads | 4 | Open |
| 8 | Project scaffolding (assembly, Rust) | Broken (asm boot image) / suspect (Rust) | Project service (`internal/workspace`) | Fix `os-image.bin` via a profile; drop `.vscode/*`; verify Rust boot | Scaffold → build → QEMU boot integration test | 4–5 | Open |
| 9 | QEMU argument construction | Works | QemuService (`internal/qemu`) | Byte-for-byte port | 4 Rust qemu tests | 5 | **Complete**: `qemu.Args`; the 4 Rust cases ported (`TestArgs`). QMP is always loopback TCP on a free port (2.x: a Unix socket in `build/`, TCP on Windows) |
| 10 | QEMU launch (debug/run) and stop | Works (X) | QemuService | Owned child with context, captured serial, reaped on exit | Integration: start, QMP status, stop; no orphan after exit | 5 | **Partial**: `qemu.Launch` owns the process, streams its output (the serial port with `-serial stdio`), reports early exits with QEMU's message, and Stop quits over QMP then kills (`TestLaunchRunsTheMachine`, `pyxforge run`); desktop Run/Debug next |
| 11 | QEMU status | Partial (process alive) | QemuService | Long-lived QMP with `query-status` and events | Fake QMP server unit tests + real QEMU integration | 5 | **Complete** in the engine: long-lived QMP client with `query-status` and events (`TestQMPClient` against a fake server, real QEMU in `TestLaunchRunsTheMachine`) |
| 12 | QMP graceful shutdown | Works (untested) | QemuService | `system_powerdown`, timeout, then kill | Fake QMP server test | 5 | **Complete**: Stop sends QMP `quit` (a halted boot sector ignores `system_powerdown`), waits two seconds, then kills |
| 13 | Snapshots save/load/delete/list | Untested end-to-end | QemuService | HMP over QMP (or `snapshot-save` job API, decided in Phase 5) | Integration with real QEMU and a qcow2 disk | 5 | Open |
| 14 | HMP monitor console | Untested end-to-end | QEMU console panel | `human-monitor-command` over the live QMP session | Integration test | 5 | **Partial**: `QMP.HMP` runs monitor commands (`info registers` tested on real QEMU); the console panel is next |
| 15 | GDB attach by architecture | Works (X, via third-party adapter) | GdbService (`internal/gdb`) | GDB/MI session owned by PyxForge | 4 Rust gdb tests + attach-to-QEMU integration | 5 | **Partial**: `internal/gdb` owns a GDB/MI session: attach (connect, then set the architecture), breakpoints, continue, interrupt, instruction steps (`TestDebugBootSectorInQEMU` against real QEMU); desktop views next |
| 16 | Registers with change highlight, memory read | Works (X) / simulated (D) | GdbService + Inspector panel | `-data-list-register-values`, `-data-read-memory-bytes` | GDB/MI record parser tests + integration | 5 | **Partial**: `Registers` and `ReadMemory` read the live machine (`TestDebugBootSectorInQEMU`); inspector views next |
| 17 | Disassembly context with PC highlight | Simulated (D) | GdbService / BinaryService | `-data-disassemble` or `llvm-objdump` | Integration test against a known boot sector | 5 | **Partial**: x86 disassembly in real, protected and long mode by `inspect.Disassemble` (`golang.org/x/arch`), since GDB decodes real-mode code as 32-bit against QEMU's x86-64 description; `pyxforge inspect --disasm` |
| 18 | Hex viewer, boot sector, `0xAA55` check | Works (X) / broken (D) | BinaryService (`internal/binary`) | Port `hex.rs` | 3 Rust hex tests | 5 | **Complete** in the engine: `inspect.Dump` and `inspect.BootSector` (the 3 Rust hex tests ported, plus disk images and the footprint); `pyxforge inspect`; inspector view next |
| 19 | PTY terminal | Works (D, one session) | TerminalService | Native PTY (ConPTY on Windows), named sessions | Spawn, write, resize, exit tests | 4 | **Partial**: Terminal panel runs the user's shell in the project folder through Neovim's `:terminal` (ConPTY on Windows, a PTY elsewhere), resizes with the panel, reports exit status, restarts (`shell/terminal.go`, `TestTerminalPanelLifecycle`, `TestTerminalRestart`); one session so far |
| 20 | File tree, read, save, dirty state | Works (D) | Explorer + Neovim buffers | Filesystem service; Neovim owns buffers | UI test: open, edit, save | 3–4 | **Complete**: explorer opens files into Neovim buffers, follows changes on disk, creates files and folders and reveals the active file; tabs mark unsaved changes, Save writes, closing asks (`editorhost.go`, `explorer/watch.go`, `TestEditorTabsFollowNeovimBuffers`, `TestWatchCreateExpandReveal`) |
| 21 | Code editor | Works (D, CodeMirror 6) | EditorService (Neovim) | D4 | Phase 3 spike criteria | 3 | **Complete**: embedded Neovim with PyxForge's isolated configuration (`nvim/`), Ctrl+S, system clipboard, LSP and Tree-sitter; ADR 0006; `internal/neovim` tests against real Neovim |
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
| N2 | Sector footprint (bytes of 512) from the build artifact | BuildService metrics | 4 | **Complete** in the engine: `inspect.BootSector` reports bytes used and free of 510; shown by `pyxforge inspect` |
| N3 | 512-byte sector map | BinaryService | 5 | Open |
| N4 | ELF inspection (`debug/elf`) | BinaryService | 5 | **Complete** in the engine: `inspect.ReadELF` (class, machine, entry, sections, loaded segments, symbols) and `CodeAt`; `pyxforge inspect` on ELF files (`TestReadELF`) |
| N5 | Named terminal sessions: shell, qemu-serial, gdb-server | TerminalService | 4–5 | Open |
| N6 | QEMU state, PID and uptime in the status bar from QMP | QemuService → StatusBar | 5 | Open |
| N7 | Local Git: status, diff, stage, commit, branch, log, stash, worktree | GitService | 4 | **Partial**: status with branch and ahead/behind, diff against HEAD in Neovim, stage, unstage, commit and recent log (`internal/git`, `shell/gitpanel.go`, `TestRepository`, `TestGitPanel`, `TestGitDiffOpensInTheEditor`); branch switching, stash and worktrees pending |
| N8 | Workspace persistence and restore | WorkspaceService | 4 | **Complete**: open files in order, active tab, panels, panel tab, explorer folders, build profile and window size are restored per workspace from the user configuration folder (`internal/workspace/state.go`, `shell/session.go`, `TestWorkspaceStateRestores`, `TestReopenedFilesKeepTheirOrderInNeovim`) |
| N9 | Tree-sitter and LSP through Neovim | EditorService / `nvim/` | 3 | **Complete**: language servers start per file type when installed (`nvim/lua/pyxforge/lsp.lua`, clangd tested); `pyxforge setup editor` builds pinned Tree-sitter parsers |
| N10 | Agents: providers, isolated worktrees, sessions, diff review | AgentService | 6 | Open |
| N11 | Agent debug loop with versioned context bundle | AgentService + Qemu + Gdb | 7 | Open |
| N12 | Theme–editor–terminal synchronization boundary | Theme + EditorService + TerminalService | 2–3 | **Complete**: the editor and the terminal take the theme as a Neovim colorscheme, the terminal's 16 ANSI colours come from the theme (`editor.TerminalColors`), and both follow theme and OS changes |
| N13 | Offline test matrix | `tests/offline` | every milestone | Open |
