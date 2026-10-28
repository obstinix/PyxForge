# Empty workspace startup and Welcome

## Root cause

`cmd/pyxforge/main.go` passes `cli.Run(...).Folder` straight to `shell.New`.
Previously, `internal/cli/cli.go:runOpen` called `folderArg` even without an
argument, and `folderArg` returned `env.Getwd()`. Thus a launcher-selected process
working directory became a user workspace. `openDefault` tried to redirect paths
under `%SystemRoot%` to the home directory, but still opened an unrelated folder;
if that detection or home lookup failed it kept the working directory.

`internal/ui/shell/shell.go:NewWithOptions` then used that root for the window
title, Explorer, recent folders, and session restoration. Explorer's `childUIDs`
listed the supplied root. There was no representation of an empty window.
Simply passing an empty string would also have been insufficient: joining an
empty Explorer root with a child path, and loading `pyxforge.toml` from an empty
root, can resolve paths relative to the process directory.

## Behavior

- No arguments, or `pyxforge open` without a path: `Folder` stays empty. Neither
  cwd nor home is consulted to select a workspace. The title is exactly `PyxForge`.
- An explicit folder still opens normally. Its stored files and layout can be
  restored, but a previous workspace is never selected implicitly on startup.
- An explicit file opens in the existing editor without selecting its parent.
- Empty Explorer operations do not enumerate, watch, expand, reveal or create
  filesystem entries. Empty configuration, branch and workspace lookups are guarded.
- Opening a folder validates it before disturbing the current workspace. Empty,
  loading, open and failed states are distinct. Failed opens retain the existing
  workspace and never enter recent history.
- Changing workspace stops the old shell's tools and watchers, then constructs
  the existing panels in the same native window. Panel dimensions and visibility
  are retained. The old shell rejects late callbacks, including already queued
  callbacks. Unsaved buffers use the existing Save / Don't Save / Cancel workflow.
- Close Workspace is available through the command palette and from Welcome when
  a workspace is active. It returns to Welcome regardless of the startup preference.
- No terminal or editor process starts on an empty launch. An explicit terminal
  or editor launch without a workspace uses the user's home directory (temporary
  directory only if home is unavailable) as its process directory. This never
  assigns a workspace or populates Explorer.

## Welcome and actions

Welcome is an ordinary closable non-file tab, supported by the existing editor
architecture, and can be reopened through **Help > Welcome**. It uses existing
kit typography, theme tokens, Fyne links and controls. Its two action columns
stack in narrow editor panes, and the content scrolls when space is limited.
The existing empty editor placeholder remains available when startup Welcome is
disabled or the Welcome tab is closed.

Open File uses the existing Neovim/file-opening workflow. Open Folder uses the
native Windows Shell folder picker; other platforms retain the Fyne folder picker.
New Project creates a new, named empty directory under an explicitly selected
parent and opens it. It does not overwrite an existing directory; the existing
build and machine setup tools still configure the project.

The Go application had no clone action. The new clone dialog calls
`internal/git.Clone`, reusing the existing Git executable lookup, network policy,
process containment, timeout and error handling. Existing destinations are
refused. Successful cloning offers Open Folder. No automatic submodule checkout
is introduced. Documentation and toolchain links use existing project resources.

## Persistence

The existing Fyne preferences store is reused. `recent.folders` remains the recent
project list: newest first, deduplicated, at most ten entries. Welcome displays
project names and paths, offers removal, and removes missing entries when clicked.
No sample projects are inserted. `welcome.showOnStartup` defaults to true and is
written by the checkbox. Disabling it never enables implicit workspace selection.
On Windows, normal app preferences live under
`%APPDATA%/fyne/io.github.obstinix.pyxforge/preferences.json`.

## UI preservation and images

No theme, icon, activity-bar layout, Inspector layout, bottom-panel layout,
workbench dimensions, Neovim rendering, or existing shortcut definitions changed.
The Help menu gains only Welcome; existing menu items retain their order.
The workspace name in the status bar reads No Folder Open in an empty window.

Images are Fyne software-rendered captures of the real shell, not native desktop
screenshots. The before image was rendered from unmodified Git HEAD in a temporary
checkout, explicitly supplying System32 to reproduce the reported bad workspace.
The after images use the modified shell with no workspace.

- [Before, 1440px](../design/welcome-screens/before/empty-smoked-kraft-crimson-1440.png)
- [After, 1440px](../design/welcome-screens/after/empty-smoked-kraft-crimson-1440.png)
- [After, 1024px](../design/welcome-screens/after/empty-smoked-kraft-crimson-1024.png)
- [After, 800px](../design/welcome-screens/after/empty-smoked-kraft-crimson-800.png)

Pixel comparison at 1440px found zero differing pixels in the menu bar, activity
bar, Explorer header, Inspector and bottom panel. Central editor and Explorer body
changes are intentional. Existing layout tests also exercise panel proportions.

## Windows build and run

Use Go 1.27.1 or newer and a working MinGW-w64 C compiler on PATH, as documented
in SETUP.md. Neovim is required for editing and terminal integration.

```powershell
go build -ldflags "-H=windowsgui" -o dist/PyxForge.exe ./cmd/pyxforge
.\dist\PyxForge.exe
.\dist\PyxForge.exe "D:\Projects\MyOS"
.\dist\PyxForge.exe "D:\Projects\MyOS\boot.asm"

# No workspace should be selected even from System32:
$app = (Resolve-Path .\dist\PyxForge.exe).Path
Start-Process -FilePath $app -WorkingDirectory "$env:WINDIR\System32"
```

For Start Menu or Desktop shortcuts, use the absolute executable path without a
folder argument. The shortcut's Start in field must not determine the workspace.
Automated native launches were checked from PowerShell, the repository, an unrelated
directory, System32, and temporary shortcuts placed on the Desktop and in the
Start Menu. All reported the title PyxForge and closed normally. Both shortcuts
used System32 as Start in and were removed after testing. These checks launched
the shortcut files through Windows Shell; they did not click the Start Menu UI.
The native folder-picker interaction still needs manual acceptance testing.

## Validation

Executed on Windows with Go 1.27.1, MinGW-w64 GCC, Git and the repository-pinned
Neovim 0.12.6 in a temporary tool directory:

| Check | Result |
| --- | --- |
| Native Windows GUI build | Passed; executable in `dist/PyxForge.exe` |
| `go test ./...` | 30 packages passed; one package failed on the pre-existing Java test below; three packages have no tests |
| Full shell suite (editor, terminal, menus, layouts and new Welcome tests) | Passed |
| Full Git suite, including cloning a real local repository | Passed |
| Targeted CLI, empty Explorer, workspace, clone and Neovim tests with `-race` | Passed |
| `go test -tags ci ./internal/ui/shell -run TestWelcomePreferencesAcrossProcesses -v` | Passed with separate writer and reader processes and actual file-backed Fyne preferences |
| `go vet ./...` | Passed |
| `staticcheck` 2026.2.1 | Passed |
| `go run ./tools/forbidcheck` | Passed |
| `gofmt -l internal cmd tools`, `git diff --check` | Clean |
| Native launches from different working directories and shortcut files | Passed, title PyxForge in all five cases |
| Software-rendered comparison at 1440px | Existing menu, activity bar, Explorer header, Inspector and bottom panel are pixel-identical |

The full run passed 294 tests/subtests and skipped 21 external-tool/platform cases.
`internal/runfile/TestRunsForReal/Java` failed because the installed Java cannot
launch the source file (`Could not find or load main class ...Hello.java`). The
same test fails identically in an untouched temporary checkout of Git HEAD.
That unrelated behavior was left unchanged.

QEMU/GDB live integration, NASM-dependent boot tests, language-server coverage,
and some compiler/platform cases were skipped when their prerequisites were
unavailable. Their unit tests ran, but this does not establish a live QEMU/GDB
session on this machine. Native folder-picker selection/cancellation remains a
manual acceptance check. Initial sandbox runs also blocked Git shell pipes;
Git and native build validation was rerun outside that sandbox.


## Changed files

- `internal/cli/cli.go`, `cli_test.go`: explicit-only startup selection and regressions.
- `internal/workspace/workspace.go`, `internal/config/config.go`: empty-root guards.
- `internal/ui/explorer/explorer.go`, `watch.go`, `explorer_test.go`: empty tree behavior.
- `internal/ui/shell/shell.go`, `openfolder.go`, `session.go`, `editorhost.go`: shell
  startup, workspace lifecycle, state persistence and process directory integration.
- `internal/ui/shell/welcome.go`, `project.go`, `folderdialog_windows.go`,
  `folderdialog_other.go`: new Welcome, project creation, clone and folder actions.
- `internal/ui/shell/recent.go`, `commands.go`, `menus.go`, `files.go`: history,
  commands, Help entry and safe file-dialog starting directory.
- `internal/ui/shell/gitpanel.go`, `gitremote.go`, `machine.go`, `machinesetup.go`,
  `snapshots.go`: prevent project-only operations without a selected workspace.
- `internal/ui/shell/welcome_test.go`, `preferences_integration_test.go`,
  `doctor_test.go`: new regressions and replacement of the old new-process-only test.
- `internal/git/clone.go`, `clone_test.go`: clone service and real local Git test.
- `tools/snapshot/main.go`: empty-workspace render option and smaller render sizes.
- `docs/INSTALL.md`, this document, and `docs/design/welcome-screens/`: usage,
  implementation notes, validation and review renders.
