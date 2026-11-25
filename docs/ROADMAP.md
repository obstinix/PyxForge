# PyxForge 3.0 roadmap

PyxForge 3.0 replaced the 2.x stack (a Rust core with a VS Code extension and a Tauri shell,
removed from the tree by decision D19; it is in git history up to `fd193a2`) with one native Go
and Fyne program. Each phase lists the evidence that it is done: a test, a command or a recorded
check, never only the presence of code. Per-feature status by platform is in
[`FEATURES.md`](FEATURES.md); where things stand day to day is in
[`PROJECT_STATE.md`](PROJECT_STATE.md); the 2.x roadmap and its documents are in
[`archive/2.x/`](archive/2.x/README.md).

Status as of 2026-11-25.

## Done

| Phase | Scope | Evidence |
|---|---|---|
| 1–2 | Design system, workbench shell, command palette, keyboard policy (K2), themes: four palettes with two accents and a System mode, flat surfaces (D18) | `TestContrast`, `TestKeyboardPolicy`, `TestKeymapDocumentsEveryChord`, `TestFlatOverlays`; review renders (`tools/snapshot`) |
| 3 | The editor. First an embedded Neovim (2026-10); since D16 PyxForge's own: a document model with undo and encodings (`internal/textbuf`), Chroma highlighting (`internal/syntax`), the editor widget (`internal/ui/codeedit`) and a language-server client (`internal/lsp`). Neovim was removed on 2026-11-24 | `TestNativeEditorInTheShell`, `TestNativeEditorLanguageServer`, `TestClangd`, `TestUTF16RoundTrip`, `TestLargeFileFrameTime` |
| 4 | Core tools: `pyxforge.toml` (2.x compatible), build profiles with streamed output and Stop, GNU, Cargo, SARIF and GCC JSON diagnostics, Problems with navigation and inline editor diagnostics, the Terminal panel on PyxForge's own pseudo-terminal and emulator (D17), Git (status, diff, stage, commit, branches, stash), workspace persistence, an explorer that follows the disk | `TestBuildProblemsInTheEditor`, `TestTerminalSessions`, `TestShellSession`, `TestGitBranchesAndStashes`, `TestWorkspaceStateRestores` |
| 5 | Systems tools: QEMU launch with serial output, QMP and the monitor, GDB/MI debugging, registers, flags, memory, real-mode disassembly, hex, boot-sector map, ELF summary, machine snapshots (qcow2 overlay) and diagnostic snapshots | `TestDebugSessionInTheShell`, `TestSnapshotsInTheShell` (real QEMU and GDB), `TestMap*`, `TestReadELF`; `examples/boot-sector` |
| 8 (part) | Hardening: process trees end with PyxForge after a forced exit, shutdown on signals, crash-safe workspace state, path containment, Git configuration that cannot run code, reproducible release builds | `TestChildrenDieWithAKilledParent`, `TestRepositoryConfigCannotRunCode`, `TestRunStaysInsideTheProject`, `tools/release`; [`INSTALL.md`](INSTALL.md) |
| 9 | Everyday workflow: Run and Debug Current File for C, C++, Rust, Go, Python, JavaScript, Java and NASM boot sectors; editor breakpoints (F9 or a gutter click) and the stopped line; explorer rename, delete to the Recycle Bin or Trash, context menu; tool versions cached between launches; `pyxforge clean`; merge or rebase a diverged branch, with Continue; New Project presets; Git worktrees | `TestRunCurrentFile`, `TestStopLine`, `TestBreakAtALine`, `TestExplorerContextMenu`, `TestVersionCache`, `TestClean`, `TestCombineByRebaseWithAConflict`, `TestPresetsBuild`, `TestWorktrees` |
| 10 | Distribution: Windows installer and portable archive with MinGit, managed toolchains (GCC, GDB, NASM, binutils, Git; QEMU through its own installer), `.deb` and AppImage, an update check against GitHub Releases that verifies SHA-256 | CI builds the `.deb`, installs and removes it with apt, and runs the AppImage (Ubuntu); `internal/managed` and `internal/update` tests. Not yet tried: any installer on a clean machine |

Verified platforms: Windows 11 x64 and Ubuntu 24.04 x64 (CI and WSL2). macOS is
compile-checked only.

PyxisOS v7 has been built (`make build` as a profile), booted (`-kernel`, after an ELF32 copy) and
debugged (GDB on its symbols) through PyxForge; see
[`cross-project/pyxisos-integration.md`](cross-project/pyxisos-integration.md).

## Open

| Item | What finishes it |
|---|---|
| Git tab diff | A side-by-side view of a changed file against its committed text in PyxForge's editor (Neovim's diff mode did this before D16; the tab now opens the file) |
| Language servers beyond clangd | Run the Python, Go, Rust, TypeScript and other servers in `internal/lsp/servers.go` against the editor in CI, as clangd is |
| QEMU serial and GDB as terminal sessions, QEMU PID and uptime | Sessions in the Terminal panel; status bar facts from QMP |
| Offline test job | A CI job that runs the suite with the network cut |
| macOS | Run the app and the integration tests on macOS before claiming support (decision D5) |
| Accessibility | Screen readers: Fyne has no accessibility API; follow upstream, keep everything reachable by keyboard meanwhile |
| Release | Code signing for the Windows installer and programs (needs a certificate: owner decision); Linux packages tested beyond Ubuntu; an installer tried on a clean machine |

## Not planned

| Item | Why |
|---|---|
| AI agents or a coding assistant | PyxForge is an editor and development environment, not an agent-based coding tool (owner, 2026-11-25). The earlier design, [ADR 0007](architecture/0007-agents.md), is kept as a record only |

## Principles kept

Offline after setup, no telemetry, no web technology in the application (`tools/forbidcheck`),
every external tool run with argument arrays and never through a shell, and nothing executed on
opening a project.
