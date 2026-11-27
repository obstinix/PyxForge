# What PyxForge 3.0 does, by platform

Each feature, on each platform, with how it is known to work. Status as of 2026-11-02.

- **CI**: a test that drives the real thing (a language server, GDB, QEMU, Git, a compiler) passes on
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
| PyxForge's editor (D16): files as tabs, save, Save As, Close All, UTF-8 and UTF-16, binary files read-only | CI | CI | `TestNativeEditorInTheShell`, `TestSaveAsAndCloseAll`, `TestUTF16RoundTrip`, `TestBinaryFilesAreNotEditedAsText` |
| Editor tabs: right-click menu (Close Others, to the Right, Saved, All; Copy Path, Reveal), pinned tabs, moving by drag or the palette; pins and order restored | CI | CI | `TestTabsPinAndMove`, `TestEditorTabManagement`, `TestPinnedTabsRestore` |
| Find References (Shift+F12) and Go to Symbol in File (Ctrl+Shift+O), listed in the palette | Local (clangd) | No | `TestNativeEditorLanguageServer`, `TestClangd`, `TestSymbolForms` |
| Split editor groups (up to three, a file in one group at a time), restored with the workspace | Local | Local (WSL) | `TestEditorGroups`, `TestEditorGroupsRestore`; render `editor-groups-*` |
| Encoding and line endings in the status bar | Local | Local (WSL) | `TestEncodingAndLineEndingNames`, `TestNativeEditorInTheShell` |
| Find and replace bar | CI | CI | `TestFindBar` |
| A file changed on disk under unsaved changes asks first | CI | CI | `TestChangedOnDiskAsksBeforeLosingChanges` |
| Workspace layout, open files and breakpoints restored | CI | CI | `TestBreakpointsAreKept`, `TestStartupOpensOnlyTheSavedFiles` |
| Explorer: follows the disk, Quick Open, preview with Space | CI | CI | `TestWatchCreateExpandReveal`, `TestListingAndQuickOpen`, `TestOpenPreviewAndActiveRow` |
| Explorer: rename (F2), move to Recycle Bin or Trash (Delete), context menu | CI | CI | `TestRename`, `TestRenameAndTrashKeys`, `TestExplorerContextMenu`; the real trash: `TestMove`, Local on Windows only (opt-in) |
| Terminal panel (D17): sessions, restart, output kept after exit, history | CI | CI | `TestTerminalSessions`, `TestShellSession`, `TestProgramOutputAndStatus` |
| Graph view (identity prompt Phase 5, D23): the project's files and notes linked by real includes and imports (C/C++, NASM, Go, Rust, Python, JS/TS, Markdown links and wikilinks), PageRank, clusters, a deterministic force layout in one raster; filters (`path:`, `ext:`, `tag:`, `-`), groups, display and force controls, presets; Theme, Spectrum and Galaxy colours from the theme; Local Graph of the current file at depth 1–3 | Local | Local (WSL) | `internal/graph` and `internal/ui/graphview` tests, `TestGraphView`, `TestLocalGraph`, `TestGraphFollowsChanges`, `TestGraphPresetsPersist`; renders `graph-*` |
| Terminal rework (identity prompt Phase 4, D24): session rail (kind, status dot, exit and duration; 40 px icons under 700 px), two panes with a sash, QEMU serial and GDB as sessions with Send input, command blocks through shell integration (bash, zsh, fish, PowerShell; never your rc files), block header with Copy Command, Copy Output, Rerun, Collapse and Send to Problems, sticky command, search in output, Ctrl+Shift+Up/Down between commands, `path:line` links inside the project | Local | Local (WSL) | `TestIntegratedShells`, `TestTerminalCommandBlocks`, `TestSessionRail`, `TestTerminalSplit`, `TestSerialSession`, `TestTerminalLinkResolution`, `TestSendBlockToProblems`, `internal/vt` and `internal/ui/termview` block tests; renders `terminal-*` |
| Command palette, menus, keyboard policy | CI | CI | `TestKeyboardPolicy`, `TestKeymapDocumentsEveryChord` |
| Themes (four, two accents) | CI | CI | `TestContrast` |
| Language servers: C and C++ (clangd): diagnostics, completion, hover, definition, rename | CI | CI | `TestClangd`, `TestNativeEditorLanguageServer` |
| Language servers: Python (pyright, basedpyright) | Not tried | Not tried | `TestServerTable` checks how the server is chosen |
| Language servers: Go, Rust, assembly, Lua, JavaScript/TypeScript, TOML, Markdown, shell | Not tried | Not tried | Configured in `internal/lsp/servers.go`; `TestServerTable` checks how a server is chosen |

## Interface components (3.1, identity prompt)

| Feature | Windows | Linux | Tests |
|---|---|---|---|
| Kit controls: Button (5 variants), Switch, SettingRow, SettingGroup, SearchField, NavList, Dropdown, Slider; keyboard-operable with focus rings | Local | Local (WSL) | `TestButton`, `TestSwitch`, `TestSettingRowsAndGroups`, `TestSearchField`, `TestNavList`, `TestDropdown`, `TestSlider`, `TestComponentContrast`; render `kit-gallery-*` |
| Every shell button is a `kit.Button` | Local | Local (WSL) | `TestShellUsesKitButtons` |
| Settings as a modal (D22): search across pages, page list, General, Editor, Appearance, Terminal, Hotkeys (display only), Toolchains, Trusted Folders, About; Esc, close button or a click outside closes it | Local | Local (WSL) | `TestSettingsOpensAndCloses`, `TestSettingsKeyboard`, `TestSettingsSearch`, `TestEverySettingHasARow`; renders `settings-{general,appearance,search,hotkeys}-*` |
| Liquid Glass (D21), opt-in in Settings > Appearance > Material: palette, Settings, notifications, tooltips and completion boxes as blurred, tinted glass with a specular rim and refraction band; text-dense content on opaque plates; Blur strength and a live preview | Local | Local (WSL) | `TestGlassContrast`, `TestFlatIsDefault`, `TestGlassIsOptIn`, kit glass tests; renders `palette-glass-*`, `settings-glass-*`; the GPU blur measured on Windows 11 only |
| Terminal > Shell: the shells found (pwsh, Windows PowerShell, cmd.exe, Git Bash; `$SHELL` and `/etc/shells`), PowerShell preferred on Windows (D24) | Local | Local (WSL) | `TestWindowsShells`, `TestUnixShells`, `TestShellSetting`, `TestTerminalUsesTheChosenShell` |
| Help > Documentation… opens the documents shipped with PyxForge, offline (D22) | Local | Local (WSL) | `TestHelpOpensShippedDocs`, `TestEveryDocExists`; the clean Windows and Ubuntu checks look for them in an installed copy |

## Build, run and debug

| Feature | Windows | Linux | Tests |
|---|---|---|---|
| New Project from a preset: empty, boot sector (builds and boots), C program (builds and runs) | CI (booting: Local) | CI | `TestPresetsBuild`, `TestNewProjectTemplates` |
| Build profiles from `pyxforge.toml`, streamed output, Stop | CI | CI | `TestBuildPanel` |
| Problems from GCC, Clang, Cargo and SARIF output, in the editor too | CI | CI | `TestBuildProblemsInTheEditor` |
| Run Current File: C, C++, Go, Rust, Python, JavaScript, Java | CI | CI | `TestRunsForReal`, `TestRunCurrentFile` |
| Debug Current File under GDB, with editor breakpoints | CI | CI | `TestBreakAtALine` |
| Debug Current File for Rust under GDB | Local (GNU toolchain; with MSVC, needs the GNU target) | CI | `TestDebugsForReal`, `TestRustDebugPlan` |
| Debug Current File for Java under jdb | CI | CI | `TestDebugsForReal`, `TestJavaDebugPlan` |
| Debug Current File for JavaScript under node inspect, stopping at editor breakpoints | CI | CI | `TestDebugsForReal`, `TestShellSession` |
| Debug Current File for Go under Delve, at main or at editor breakpoints | CI | CI | `TestDebugsForReal` (Go, Go at a breakpoint) |
| Breakpoints: F9 and a click in the gutter, kept on their lines as text changes | CI | CI | `TestNativeEditorInTheShell`, `TestMouseAndFolding` |
| The line GDB stopped at marked in the editor | CI | CI | `TestStopLine`, `TestBreakAtALine` |
| Run and Debug in QEMU: serial output, QMP, registers, memory, disassembly | Local | CI | `TestDebugSessionInTheShell`, `TestDebugBootSectorInQEMU` |
| Editor breakpoints in a NASM boot sector, through its listing | Local | CI | `TestBreakpointInABootSector`, `TestBootSectorListing` |
| Editor breakpoints and the stopped line in an ELF kernel with debug information | Local | CI | `TestBreakpointInAnELFKernel` |
| Call stack of a QEMU debug session (Stack tab), with source lines from DWARF or a NASM listing | Local | CI | `TestBreakpointInAnELFKernel`, `TestBreakpointInABootSector` |
| A breakpoint set while the machine runs takes effect at once; Pause | Local | CI | `TestBreakpointWhileRunning` |
| Workspace trust: Build, Run and Debug in QEMU, Run and Debug Current File and a repository's own hooks folder ask first in an untrusted folder | CI | CI | `TestBuildAsksForTrust`, `TestMachineAndRunCurrentFileAskForTrust`, `TestCommitAsksToTrustTheRepositoryHooks`, `TestRepositoryHooksPathNeedsTrust`, `TestTrustStore` |
| Machine snapshots (qcow2 overlay) | Local | CI | `TestSnapshotsInTheShell`, `TestMachineSnapshots` |
| Set Up Machine: write or edit `[qemu]` and `[gdb]` | Local | CI | `TestSetUpMachine`, `TestMachineFile` |
| Git tab: status, diff, stage, commit, branches, stash | CI | CI | `TestGitPanel`, `TestGitBranchesAndStashes` |
| Side-by-side Git diff (unstaged against the index, staged-only against HEAD, untracked against nothing), change navigation, Show Changes in Current File | Local | Local (WSL) | `TestGitDiffView`, `internal/diff` tests; renders `git-diff-*` |
| Git: merge or rebase a diverged branch; resolve, Continue or Abort | CI | CI | `TestCombineByMerge`, `TestCombineByRebaseWithAConflict`, `TestGitConflictInProgress`, `TestGitRemoteWorkflow` |
| Git worktrees: create, list, open, remove | CI | CI | `TestWorktrees`, `TestGitWorktrees` |
| Git fetch, pull and push | CI (local remote, SSH and HTTP servers) | CI (local remote, SSH and HTTP servers) | `TestGitRemoteWorkflow`; `TestSSHRemote` and `TestHTTPRemote` against servers on loopback; never against a hosted service |
| Git asks for a password, key passphrase or a new server's trust in a dialog | CI | CI | `TestFetchAsksForCredentials` (an HTTP server wanting a password), `TestGitAsksThroughPyxForge`, `TestAskUserDialogs`; `TestSSHRemote` (a new server's fingerprint and a key's passphrase asked by real ssh: Git for Windows' and Windows' own on Windows, OpenSSH on Linux and macOS) |

## Installing and keeping up to date

| Feature | Windows | Linux | Tests |
|---|---|---|---|
| Doctor: finds tools and says how to install them | CI | CI | `TestDoctor` |
| Tool versions cached between launches | CI | CI | `TestVersionCache` |
| Managed toolchains (GCC, GDB, NASM, binutils, Git) | CI | No | `internal/managed` tests, against local test servers |
| QEMU through its own installer, elevated | Not tried | No | `internal/managed` tests stop short of running it |
| Windows installer and portable archive | Release CI | No | `release-windows.yml` builds them with `tools/winpkg`, installs, checks and uninstalls on a runner and in a clean Windows Server Core container (last on 2026-11-25, `7d29010`) |
| `.deb` package | No | CI | CI builds it, installs and removes it with apt |
| AppImage | No | CI | CI builds it, runs it and checks its SHA-256 |
| Update check against GitHub Releases, SHA-256 verified download | CI | CI | `internal/update` tests, against local test servers |
| `pyxforge clean`, including the Neovim configuration release candidates left | CI | CI | `TestClean` |
