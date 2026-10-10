# PyxForge 3.0 roadmap

PyxForge 3.0 replaces the 2.x stack (a Rust core with a VS Code extension and a Tauri shell, now
in `legacy/`) with one native Go and Fyne program that embeds a real Neovim. Each phase lists the
evidence that it is done: a test, a command or a recorded check, never only the presence of code.
The detailed per-feature status is in [`architecture/FEATURE_PARITY.md`](architecture/FEATURE_PARITY.md);
the 2.x roadmap and its documents are in [`archive/2.x/`](archive/2.x/README.md).

Status as of 2026-10-11.

## Done

| Phase | Scope | Evidence |
|---|---|---|
| 1–2 | Design system, five themes with two accents, workbench shell, command palette, keyboard policy (Ctrl+Shift chords only) | `TestContrast`, `TestShellChordsLeaveKeysToNeovim`, `TestKeymapDocumentsEveryChord`; review renders (`tools/snapshot`) |
| 3 | Embedded Neovim: isolated configuration, buffers as tabs, save, clipboard, Tree-sitter, language servers, pinned plugins (`pyxforge setup editor`) | `internal/neovim` tests against real Neovim 0.9 and 0.12; ADR 0006 |
| 4 | Core tools: `pyxforge.toml` (2.x compatible), build profiles with streamed output and Stop, GNU, Cargo, SARIF and GCC JSON diagnostics, Problems with navigation and inline editor diagnostics, terminal sessions, Git (status, diff, stage, commit, branches, stash), workspace persistence, explorer that follows the disk | The 16 2.x config cases, the 12 2.x diagnostics cases, `TestBuildProblemsInTheEditor`, `TestTerminalSessions`, `TestGitBranchesAndStashes`, `TestWorkspaceStateRestores` |
| 5 | Systems tools: QEMU launch with serial output, QMP and the monitor, GDB/MI debugging, registers, flags, memory, real-mode disassembly, hex, boot-sector map, ELF summary, machine snapshots (qcow2 overlay) and diagnostic snapshots | `TestDebugSessionInTheShell` and `TestSnapshotsInTheShell` (real QEMU and GDB), `TestMap*`, `TestReadELF`; `examples/boot-sector` |
| 8 (part) | Hardening: process trees end with PyxForge after a forced exit, shutdown on signals, crash-safe workspace state, path containment, Git configuration that cannot run code, reproducible release builds, install guide | `TestChildrenDieWithAKilledParent`, `TestRepositoryConfigCannotRunCode`, `TestRunStaysInsideTheProject`, `tools/release`; [`INSTALL.md`](INSTALL.md) |

Verified platforms: Windows 11 x64 and Ubuntu 24.04 x64 (CI and WSL2, the desktop app under
X11). macOS is compile-checked only.

PyxisOS v7 has been built (`make build` as a profile), booted (`-kernel`, after an ELF32 copy) and
debugged (GDB on its symbols) through PyxForge; see
[`cross-project/pyxisos-integration.md`](cross-project/pyxisos-integration.md).

## Open

| Item | What finishes it |
|---|---|
| Git worktrees | Create, list and remove worktrees from the Git tab, tested on a real repository |
| rustc human-readable and MSVC diagnostics | Parsers with golden inputs from real compiler output (parity row 5) |
| Build presets and project scaffolding (2.x rows 7, 8) | Each preset writes a `pyxforge.toml` that loads and builds; the assembly scaffold boots in QEMU |
| QEMU serial and GDB as terminal sessions, QEMU PID and uptime | Sessions in the Terminal panel; status bar facts from QMP (rows N5, N6) |
| Agents (Phases 6–7) | Provider-neutral interface, isolated worktrees, diff approval, a debug loop with a versioned context bundle (rows 23, N10, N11) |
| Offline test job | A CI job that runs the suite with the network cut (row N13) |
| macOS | Run the app and the integration tests on macOS before claiming support (decision D5) |
| Accessibility | Screen readers: Fyne has no accessibility API; follow upstream, keep everything reachable by keyboard meanwhile |
| Release | Application icon and signed installers (owner decisions), then retire `legacy/` once every parity row is Complete or Dropped |

## Principles kept

Offline after setup, no telemetry, no web technology in the application (`tools/forbidcheck`),
real Neovim as the editor, every external tool run with argument arrays and never through a
shell, and nothing executed on opening a project.
