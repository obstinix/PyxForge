# What PyxForge 3.0 does, by platform

Each feature, on each platform, with how it is known to work. Status as of 2026-11-02.

- **CI**: a test that drives the real thing (Neovim, GDB, QEMU, Git, a compiler) passes on
  GitHub Actions on every push: `windows-latest` (Windows Server, x64) and `ubuntu-latest`
  (Ubuntu 24.04, x64).
- **Local**: the same tests pass on the maintainers' Windows 11 x64 machine but skip on CI,
  because the tool is not installed there.
- **Release CI**: run by the release workflow when a version is tagged, not on every push.
- **Not tried**: built, but never run on that platform.
- **No**: not available on that platform.

macOS is compile-checked on CI only: nothing has been run there, so every macOS cell is
**Not tried**. Tests named here are checked to exist by `docs/docs_test.go`.

## Editor and workspace

| Feature | Windows | Linux | Tests |
|---|---|---|---|
| Embedded Neovim editor, files as tabs, save, Save As, Close All | CI | CI | `TestEditorTabsFollowNeovimBuffers`, `TestSaveAsAndCloseAll` |
| Find and replace bar | CI | CI | `TestFindBar` |
| A file changed on disk under unsaved changes asks first | CI | CI | `TestChangedOnDiskAsksBeforeLosingChanges` |
| Workspace layout, open files and breakpoints restored | CI | CI | `TestBreakpointsAreKept`, `TestTabOrigins` |
| Explorer: follows the disk, Quick Open, preview with Space | CI | CI | `TestWatchCreateExpandReveal`, `TestListingAndQuickOpen`, `TestOpenPreviewAndActiveRow` |
| Explorer: rename (F2), move to Recycle Bin or Trash (Delete), context menu | CI | CI | `TestRename`, `TestRenameAndTrashKeys`, `TestExplorerContextMenu`; the real trash: `TestMove`, Local on Windows only (opt-in) |
| Terminal panel: sessions, restart, transcripts of exited shells | CI | CI | `TestKeepTranscript`, `TestRunInTerminal` |
| Command palette, menus, keyboard policy | CI | CI | `TestKeyboardPolicy`, `TestKeymapDocumentsEveryChord` |
| Themes (five, two accents) | CI | CI | `TestContrast` |
| Language servers: C and C++ (clangd) | CI | CI | `TestLanguageServers` |
| Language servers: Python (pyright) | Not tried | CI | `TestPythonLanguageServer` |
| Language servers: Go, Rust, assembly, Lua, JavaScript/TypeScript, Java, TOML, Markdown, shell | Not tried | Not tried | Configured in `nvim/lua/pyxforge/lsp.lua`; `TestServerCommands` checks how a server is chosen |

## Build, run and debug

| Feature | Windows | Linux | Tests |
|---|---|---|---|
| Build profiles from `pyxforge.toml`, streamed output, Stop | CI | CI | `TestBuildPanel` |
| Problems from GCC, Clang, Cargo and SARIF output, in the editor too | CI | CI | `TestBuildProblemsInTheEditor` |
| Run Current File: C, C++, Go, Rust, Python, JavaScript, Java | CI | CI | `TestRunsForReal`, `TestRunCurrentFile` |
| Debug Current File under GDB, with editor breakpoints | CI | CI | `TestBreakAtALine` |
| Breakpoints: F9 and a click in the gutter | CI | CI | `TestBreakpoints`, `TestGutterClickTogglesBreakpoint` |
| The line GDB stopped at marked in the editor | CI | CI | `TestStopLine`, `TestBreakAtALine` |
| Run and Debug in QEMU: serial output, QMP, registers, memory, disassembly | Local | CI | `TestDebugSessionInTheShell`, `TestDebugBootSectorInQEMU` |
| Editor breakpoints in a NASM boot sector, through its listing | Local | CI | `TestBreakpointInABootSector`, `TestBootSectorListing` |
| Editor breakpoints and the stopped line in an ELF kernel with debug information | Local | CI | `TestBreakpointInAnELFKernel` |
| Machine snapshots (qcow2 overlay) | Local | CI | `TestSnapshotsInTheShell`, `TestMachineSnapshots` |
| Set Up Machine: write or edit `[qemu]` and `[gdb]` | Local | CI | `TestSetUpMachine`, `TestMachineFile` |
| Git tab: status, diff, stage, commit, branches, stash | CI | CI | `TestGitPanel`, `TestGitBranchesAndStashes` |
| Git: merge or rebase a diverged branch; resolve, Continue or Abort | CI | CI | `TestCombineByMerge`, `TestCombineByRebaseWithAConflict`, `TestGitConflictInProgress`, `TestGitRemoteWorkflow` |
| Git fetch, pull and push | CI (local remote) | CI (local remote) | `TestGitRemoteWorkflow`; never against a hosted server |

## Installing and keeping up to date

| Feature | Windows | Linux | Tests |
|---|---|---|---|
| Doctor: finds tools and says how to install them | CI | CI | `TestDoctor` |
| Tool versions cached between launches | CI | CI | `TestVersionCache` |
| Managed toolchains (GCC, GDB, NASM, binutils, Git) | CI | No | `internal/managed` tests, against local test servers |
| QEMU through its own installer, elevated | Not tried | No | `internal/managed` tests stop short of running it |
| Windows installer and portable archive | Release CI | No | `release-windows.yml` builds them with `tools/winpkg`, installs, checks and uninstalls on a runner (last on 2026-10-25); never tried on a clean machine |
| `.deb` package | No | CI | CI builds it, installs and removes it with apt |
| AppImage | No | CI | CI builds it, runs it and checks its SHA-256 |
| Update check against GitHub Releases, SHA-256 verified download | CI | CI | `internal/update` tests, against local test servers |
| `pyxforge clean`, old editor runtimes pruned | CI | CI | `TestClean`, `TestPrune` |
