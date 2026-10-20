# Defect map: Phase 0 audit

Audit of commit `f3e0ee0` (branch `integration`, equal to `origin/main`), 2026-10-20, on
Windows 11 at 160% display scale (work area 2560×1528 px). Every item was tested
against the built application in a throwaway profile (`APPDATA`, `LOCALAPPDATA` and `TEMP`
redirected), with fixture workspaces created for the audit and Git tests run only in temporary
repositories. No application code was changed.

## Read this first: the register was not available

The defect register, `docs/audit/defect-register.md`, is not in the repository, its history or
anywhere on this machine. Each row's topic comes from the grouping in the remediation brief
(001 startup tabs; 002–004 performance, responsiveness, stability; 005, 018, 019, 021
navigation, discoverability, tooltips/palette, dialogs, error messages; 006–008 alignment,
clipping, resizing, appearance; 009–013 and 020 file operations; 014–017 terminal; 022 Git; 023
QEMU/GDB; FEAT-001–004). Where the brief gives a group more topics than IDs, or the reverse, the
split between IDs is mine and is marked *(assigned)*. Once the register is supplied, the rows
must be re-keyed to its wording, and anything it describes that is not covered here must be tested.

## Legend

- **Reproduced?** Yes; Partly (some of the described behaviour); No (tried and not seen); N/A
  (a feature, assessed instead of reproduced).
- **State.** Broken: a reproduced defect. Missing: an expected function is absent. Degraded:
  it works, with the flaws listed. Works: tested and no defect found. Unconfirmed: not
  reproduced, so the register's exact steps are needed.
- Code locations are relative to the repository root, and line numbers are at `f3e0ee0`. Fyne
  locations are in the module `fyne.io/fyne/v2@v2.8.1`.

## Map

| ID | Reproduced? | Repro steps | Code location | State | Root-cause hypothesis | Shared root cause with |
|---|---|---|---|---|---|---|
| PXF-DEF-001 — tabs open on their own at startup | No (at startup). Partly: a related path does open unrequested tabs | Fresh profile, open a workspace: 0 tabs, welcome view. Open 3 files, quit, relaunch 3 times: the same 3 tabs each time, no duplicates, and saved state stays at 3 entries. The owner's saved window states hold 1–3 files each, with no duplicates. Related: relaunch a workspace whose state holds 2 files, click into the editor, press Ctrl+O 10 times: `file05.txt`, opened only in an *earlier* session, comes back as a new tab. | `internal/ui/shell/session.go:98` `restoreState`, `:133` `openFiles`; `internal/ui/shell/editorhost.go:204` `onEvent` (`BufEnter` makes a tab for any file buffer); `nvim/lua/pyxforge/options.lua` (no `shada` setting) | Unconfirmed | Every file buffer Neovim enters becomes a tab, whoever asked for it: jumplist (Ctrl+O), file marks (`'0`), `:b`, `:e`, quickfix, LSP go-to-definition. The ShaDa file (`%LOCALAPPDATA%\pyxforge-data\shada\main.shada`) is shared by every workspace through `NVIM_APPNAME`, and the owner's copy lists files in 5 different folders, so these jumps can reach files from other projects. Startup itself restored only saved files in every run. | DEF-012, DEF-010, FEAT-001 (cluster B) |
| PXF-DEF-002 — memory use grows while working *(assigned: performance)* | Yes | Fresh profile, workspace with 20 small files, working set (WS) 157 MB. Open `many/file01…20` from the explorer and close all 20 tabs: 561 MB, 574 MB after 75 s idle. Two more cycles: 894 MB, then 1,027 MB, still 1,027 MB after 75 s idle. Profiled build with forced GC: live Go heap 51 MB → 317 MB after the first cycle, then 317–318 MB through cycles 2 and 3. Heap profile: 302 MB (93.8%) under `painter.CachedFontFace` → `lookupFaces` → `go-text font.ParseTTF`. Goroutines 18 → 21; child processes return to baseline. | `internal/ui/shell/regions.go:25` `activate` (calls `ThemeOverride.Refresh`), `:39` `regionTabs`; `internal/ui/shell/shell.go:268-272` (an explorer open activates the editor, then the explorer again); Fyne `internal/cache/theme.go` `OverrideTheme` (new scope ID on every call); Fyne `internal/painter/font.go` `CachedFontFace` (cache keyed by style and scope; cleared only on a settings change) | Broken | Each `ThemeOverride.Refresh` gives everything under a tab bar a new cache scope. The next paint of text in that scope parses the whole font set again: theme font, Fyne default, `EmojiOneColor.otf` (4.2 MB), symbols, plus a system font for Devanagari names. The result is a new cache entry that is never evicted. In the profiled run the live heap levelled off after the first cycle, so the growth is bounded, but it is bounded at about 266 MB above baseline. The WS keeps rising to about 1 GB because Go's GC target is twice the live heap and the re-parsing churns allocations. To confirm in Phase 1: count `fontCache` entries per scope, and repeat with ASCII-only file names. | DEF-003, DEF-004 (cluster A) |
| PXF-DEF-003 — sluggish UI *(assigned: responsiveness)* | Partly | Window visible after 433, 472, 488 and 500 ms (release build, 4 launches) and 637 ms (profiling build), with 0.8–1.0 s of process CPU by then. A 10 s CPU profile of region switching and typing shows 0.34 s of non-idle CPU, so there is no measurable latency there. The cost lands on file opens: each first paint under a new scope re-parses fonts (cluster A). Startup spawns `--version` for every tool (2.58 s of thread time, off the UI goroutine). No UI freeze was observed. | Fyne `internal/painter/font.go`; `internal/toolchain` `(*Detector).detect` / `runVersion`; `internal/ui/shell/editorhost.go:149-160` (clipboard read waits up to 2 s on the UI thread) | Degraded | Font re-parsing on file open (cluster A) is the only measured hot path. The clipboard bridge's 2 s timeout can stall a paste when the UI thread is busy. | DEF-002 (cluster A) |
| PXF-DEF-004 — instability *(assigned: stability)* | Partly | No crash or hang occurred in any test in this audit. The working set reaches 1 GB in three open/close cycles (DEF-002), a likely path to failure in long sessions on machines with less memory. Earlier in the audit, one terminal panel showed 3 shells when 1 was expected; I could not reproduce this or explain it. | as DEF-002; `internal/ui/shell/terminal.go:154` `selectSession`, `:369` `closed` | Unconfirmed | If long sessions fail, the most likely cause is the retained font cache and the 1 GB working set (cluster A). The 3-shell observation needs the register's steps. | DEF-002 (cluster A), DEF-017 |
| PXF-DEF-005 — keyboard navigation between regions *(assigned: navigation)* | Yes | Open a file from the explorer (click or Enter): the tab opens, but keyboard focus stays in the tree, so typing goes to the tree and is lost. Moving between regions needs a Ctrl+Shift chord or the mouse. | `internal/ui/shell/shell.go:268-272` (`OnOpen`: "opening from the tree keeps the user in the tree"); `internal/ui/shell/regions.go:86-98` | Broken | Keeping focus in the tree is a deliberate rule, but it covers every way of opening, including Enter and a double-click, where users expect the editor to take focus. | DEF-009, DEF-010 (cluster D) |
| PXF-DEF-006 — clipped and misaligned content *(assigned: alignment, clipping)* | Yes (clipping); misalignment not seen on its own | Clipped at the default size: the Git panel's "Recent commits" list; the terminal's Restart button; the QEMU panel's resolved command line; notification text (the Git LFS refusal and the missing-machine-config message end mid-sentence). | `internal/ui/shell/panels.go:40-50`; `internal/ui/shell/gitpanel.go`; `internal/ui/shell/terminal.go:308-315`; `internal/ui/shell/machine.go`; `internal/ui/notifications/notifications.go:56-70` (`Post`) | Broken | Panels lay content out at fixed sizes with no scrolling or wrapping fallback. A notification's body is one line of `kit.Text` in a 360 px toast, so longer text is cut off, and the toast disappears after 6 s (10 s for errors) with no way to read the rest. | DEF-007, DEF-008, DEF-016 (cluster C); DEF-021 (cluster E) |
| PXF-DEF-007 — panels cannot be resized *(assigned: resizing)* | Yes | There is no splitter between explorer, editor, dock and inspector, so no border can be dragged. The explorer is 256 px, the inspector 296 px, and the dock min(220 px, 40% of the height), about 8 terminal rows. | `internal/ui/theme/metrics.go:18-20`; `internal/ui/shell/layout.go:43-101` | Missing | The shell layout is a fixed-metric custom layout with no splitter state and no persistence of user sizes. | DEF-006, DEF-016 (cluster C) |
| PXF-DEF-008 — window placed partly off-screen *(assigned: appearance)* | Yes | Launch, close and relaunch repeatedly: Windows cascades each new window 38 px down and right. By the 7th launch the 2182×1406 px window is at (266, 266), so its bottom edge is at 1,672 against a 1,528 px work area, and the status bar and the bottom of the dock are off-screen. | `internal/ui/shell/shell.go:181` (`Resize(1440, 900)`); `internal/ui/shell/session.go:122-124` (restores the saved size, never the position, with no clamp) | Broken | The default 1440×900 window is 1,406 px tall on this display, 92% of the work-area height. Windows cascades each new window by 38 px, and nothing clamps the size or position to the monitor's work area. | DEF-006, DEF-007 (cluster C) |
| PXF-DEF-009 — no New/Open/Save As *(assigned: file dialogs)* | Yes | No menu bar. The palette has "Open Folder in New Window…" (native folder dialog) and "New File…" (a name form, files inside the workspace only), but there is no Open File, Save As or Save a Copy anywhere. A file outside the workspace can be opened only from the command line (`pyxforge <file>` opens its project) or with `:e` inside Neovim. | `internal/ui/shell/commands.go:102-104`; `internal/ui/shell/openfolder.go:17`; `internal/cli/cli.go:186-199` (file argument) | Missing | The shell was built around a workspace folder and leaves file management to the explorer and Neovim commands. There is no desktop file layer beyond Open Folder. | DEF-010, DEF-020, DEF-018 (cluster D), FEAT-001 |
| PXF-DEF-010 — standard shortcuts do nothing expected *(assigned: shortcuts)* | Yes | In the editor, Ctrl+S saves (a Neovim mapping). Ctrl+N, Ctrl+O, Ctrl+W and Ctrl+P go to Neovim: Ctrl+O jumps back through the jumplist and can open old files as tabs (DEF-001), Ctrl+W starts a window command, and Ctrl+N/Ctrl+P move the cursor. The shell's own bindings are all Ctrl+Shift chords. | `internal/ui/shell/commands.go:16-17` ("Keybindings are Ctrl+Shift chords only (K1)"); `internal/ui/shell/keymap.go:23`; `nvim/lua/pyxforge/keymaps.lua:7-8` | Broken | Decision K1 deliberately leaves Ctrl+letter to Neovim. Fixing this means revisiting K1, which needs approval. | DEF-001 (cluster B), DEF-005, DEF-019 (cluster D) |
| PXF-DEF-011 — saving corrupts binary files *(assigned: save fidelity)* | Yes | Open `binary.bin` (bytes 0–255 ×4, 1,024 bytes, no trailing newline), make no change, then `:w`: the file is now 1,025 bytes, with 0x0A appended. | `nvim/lua/pyxforge/options.lua` (no `binary` / `fixendofline` handling); `internal/ui/shell/editorhost.go:204` (opens every file in Neovim) | Broken | Neovim's default `fixendofline` adds a final newline to any file that lacks one, and PyxForge neither detects binary files (NUL bytes) to set `binary` nor refuses or warns before editing them. This loses data. | DEF-012 (cluster B), FEAT-001 |
| PXF-DEF-012 — a file changed on disk while edited *(assigned: external changes)* | Yes | Clean buffer, file changed on disk: it reloads silently (correct). Modified buffer, file changed on disk: Neovim's W12 "file has changed" prompt appears *inside the editor grid*, the tab still shows the old title and dirty marker while the grid shows the prompt, and keyboard focus is in the explorer, so the prompt cannot be answered without clicking the editor. | `nvim/lua/pyxforge/options.lua:19-45` (`autoread` + checktime autocmd); `internal/ui/shell/editorhost.go:204-230` | Broken | Neovim's own conflict prompt is used instead of a PyxForge dialog. No `FileChangedShell` handling reaches the Fyne UI, so the shell's tab state and focus are not in step with Neovim's prompt. | DEF-011, DEF-001 (cluster B); DEF-021 (cluster E) |
| PXF-DEF-013 — encodings and line endings *(assigned: encoding)* | No | In a workspace with a non-ASCII name, edited and saved a CRLF file and a UTF-16 file: line endings, BOM and encoding kept. Fixtures for UTF-8 with BOM, UTF-16 BE and a Devanagari folder opened without error. | `nvim/lua/pyxforge/options.lua` | Works | No defect found; it may be a different symptom in the register. | n/a |
| PXF-DEF-014 — terminal basics *(assigned: start, cwd, I/O)* | No | The terminal tab starts `cmd.exe` in a real ConPTY through Neovim's `:terminal`, with cwd = the workspace root (including a non-ASCII path). stdout and stderr are shown, and resize reflows. | `internal/ui/shell/terminal.go:97-150`; `internal/neovim/session.go:468` `NewTerminal` | Works | No defect found. | n/a |
| PXF-DEF-015 — shell exit hides its output *(assigned: exit handling)* | Yes | Run a command, then `exit` (or end the shell): all scrollback is replaced by a placeholder ("The shell exited with status …"). One shell in the audit reported status `-1073741510`, shown as a raw signed number with no explanation. It is 0xC000013A, STATUS_CONTROL_C_EXIT. | `internal/ui/shell/terminal.go:154-160` `selectSession`, `:270-315`, `:338-375` | Broken | On `TermClose` the panel swaps the terminal view for a placeholder instead of keeping the buffer read-only with a status line, and statuses are printed raw with no Windows NTSTATUS mapping. | DEF-021 (cluster E), cluster B |
| PXF-DEF-016 — terminal too small *(assigned: terminal size)* | Yes | At the default size the dock is about 8 terminal rows and cannot be made taller (DEF-007). Maximising the window only gets it to min(220 px, 40%). | `internal/ui/shell/layout.go:87`; `internal/ui/theme/metrics.go:20` | Broken | Fixed dock height (cluster C). | DEF-007 (cluster C) |
| PXF-DEF-017 — terminal process lifecycle *(assigned: processes)* | Partly | Ctrl+C stops a running command, and no orphaned process remains afterwards. After quitting, the child processes are back to baseline (job-object containment). Seen once: 3 shells where 1 was expected; not reproduced. Restart button clipped (DEF-006). | `internal/ui/shell/terminal.go:97-150`, `:369-380` | Unconfirmed | The 3-shell observation may be a `start` that raced the first-show hook. It needs the register's steps. | DEF-004, DEF-006 |
| PXF-DEF-018 — controls are not discoverable *(assigned: tooltips)* | Yes | Hover any icon button (activity bar, panel actions such as New Terminal, Restart, Close): nothing shows its name. Without a menu bar, the 66 commands are reachable only through the palette (Ctrl+Shift+P) or their chords. | `internal/ui/kit/button.go:25-60` (`IconButton.Label` is only the accessible name and the palette title; `MouseIn` only sets hover styling) | Missing | Tooltips were never implemented for `IconButton`, and there is no menu bar to show commands and their shortcuts. | DEF-009, DEF-019 (cluster D) |
| PXF-DEF-019 — palette and shortcut discoverability *(assigned: palette)* | Partly | The palette opens with Ctrl+Shift+P, lists 66 commands with their chords and filters as you type. It is the only place shortcuts are listed. Commands that exist as Neovim mappings (Save via Ctrl+S) are listed under their Ctrl+Shift chord only. | `internal/ui/commandpalette/palette.go`; `internal/ui/shell/commands.go` | Degraded | It works. Discoverability depends on knowing the palette exists: there is no menu bar, and the palette is mentioned only by its chord on the welcome view. | DEF-018, DEF-010 (cluster D) |
| PXF-DEF-020 — no recent files or folders *(assigned: recent)* | Yes | There is no "Open Recent" anywhere. Window state is saved per workspace, but there is no list of workspaces or files to reopen. | `internal/ui/shell/session.go` (per-workspace state only) | Missing | Not implemented. | DEF-009 (cluster D) |
| PXF-DEF-021 — dialogs ignore the keyboard; messages cut off *(assigned: dialogs, error messages)* | Yes | Close a modified tab: the "Save changes?" dialog appears, but Esc does not cancel it. The explorer's New File dialog ignores both Esc and Enter. Error notifications end mid-sentence (Git LFS refusal, missing QEMU config), and terminal exit statuses are raw numbers (DEF-015). | `internal/ui/shell/editorhost.go:358` `confirmUnsaved` (`dialog.NewCustomWithoutButtons`), `:373` `confirmQuit`; `internal/ui/shell/session.go:174` (New File form); `internal/ui/notifications/notifications.go:56-70` | Broken | Dialogs come from Fyne's `dialog.NewForm` / `NewCustomWithoutButtons` and nothing binds Esc to Cancel or Enter to the default button. The form entries have no `OnSubmitted`. Notifications cut text off (DEF-006) and offer no "show more" or log link. | DEF-006, DEF-015, DEF-022, DEF-023 (cluster E) |
| PXF-DEF-022 — Git panel refuses a normal LFS repository | Yes | (a) Temporary repo, modify a file, stage, commit: works; history updates. (b) Temporary repo with `filter.lfs.*` in `.git/config`, set to the same values as the system config (what `git lfs install --local` writes): Stage fails with "this repository's own Git configuration sets filter.lfs.… = git-lfs …", and the notification cuts the text off. At least one of the owner's working repositories has this config, so staging fails there. (c) A folder that is not a repository: guidance text, but no "Initialize repository" action. | `internal/git/git.go:212` `projectPrograms`, `:235` `safety` (introduced by `d6325d0`); `internal/ui/shell/gitpanel.go` | Broken | **A regression from my own commit `d6325d0`** (D14 Git safety). It refuses every write while any program key exists in the local or worktree scope, even when the value equals the one the user's global or system config already runs. Proposed: treat a local value that is identical to a global or system value as user-trusted, keep refusing anything else, and test both with temporary repos. | cluster G; DEF-021 (cluster E) |
| PXF-DEF-023 — QEMU/GDB setup is not discoverable | Partly | With the `boot-sector` example (has `pyxforge.toml`): Debug starts QEMU paused with a GDB stub and attaches GDB, stops at `0x7c00` and shows registers; Stop ends both QEMU and GDB. Without `pyxforge.toml`: QEMU and GDB actions show a short-lived notification that cuts its text off, and there is no GUI to configure a machine. The resolved QEMU command line is clipped in the panel. | `internal/ui/shell/machine.go`, `machineviews.go`; `internal/qemu`, `internal/gdb`; `internal/config` (machine settings from `pyxforge.toml`) | Degraded | The machine workflow is configured only through `pyxforge.toml`, and the empty state does not explain that or offer to create one. | DEF-006, DEF-021 (clusters C, E); FEAT-004 (cluster F) |
| PXF-FEAT-001 — general-purpose editor | N/A | Embedded Neovim (`nvim --embed`, ext_linegrid) edits any text file; tabs follow buffers; save via Ctrl+S or Ctrl+Shift+S; encodings preserved (DEF-013). Gaps: binary files corrupted on save (DEF-011), no New/Open/Save As/Recent (DEF-009, DEF-020), Neovim-modal editing with no indication for non-Vim users beyond the status line `NORMAL`. | `internal/ui/editor`; `internal/ui/shell/editorhost.go`; `internal/neovim`; `nvim/` | Degraded | The engine is complete; the desktop file layer around it is missing. Neovim stays the engine (fixed architecture). | DEF-009, DEF-011, DEF-012, DEF-020 |
| PXF-FEAT-002 — language-aware editing | N/A | Treesitter highlighting; LSP attaches when a server is installed (`clangd` attached to `test file.c`, shown in the status bar); a missing server raises a "not installed" notification. Diagnostics feed the Problems panel. Installed here: clangd 23.1.1, gopls 0.23.0, rust-analyzer 1.97.0; not installed: pyright/basedpyright, typescript-language-server. | `nvim/lua/pyxforge/lsp.lua`, `treesitter.lua`; `internal/ui/shell/editorhost.go:232-260`; `internal/ui/shell/problems.go` | Degraded | It works where servers exist. There is no in-app way to install or point to a server, and no per-language settings. | FEAT-003 |
| PXF-FEAT-003 — build and run | N/A | Build runs the `pyxforge.toml` profile (Ctrl+Shift+B) and streams output to the Build panel. There is no "Run current file" and no per-language default (Python, Node, Go, Rust/Cargo, C/C++, Java). Without `pyxforge.toml` the Build panel has nothing to run. | `internal/ui/shell/commands.go` (`build.run`); `internal/build`; `nvim/lua/pyxforge/build.lua` | Missing | Run configurations exist only as project-file profiles. | DEF-023, FEAT-004 (cluster F) |
| PXF-FEAT-004 — unified run and debug | N/A | Debugging is GDB attached to QEMU only (DEF-023). There is no debugger for host programs (GDB/LLDB native), Python (debugpy), Node (inspector), Go (Delve) or Rust. Available here: GDB 17.2, LLDB 23.1.1, debugpy; Delve not installed. | `internal/ui/shell/machine.go`; `internal/gdb`, `internal/qemu` | Missing | The debug layer was built for the boot-sector machine; no adapter abstraction exists. | DEF-023, FEAT-003 (cluster F) |

## Root-cause clusters

- **A. Fyne font cache multiplied by theme-override scopes** (DEF-002, 003, 004). `activate()`
  refreshes the per-region `ThemeOverride`s on every change of region. Each refresh mints a
  cache scope, and each scope parses the full font set into a cache that is never evicted.
  One fix (draw the active-region accent without re-scoping) should remove about 260 MB of live
  heap and the 1 GB working set.
- **B. Neovim drives the UI one way, and its defaults and prompts reach the user unfiltered**
  (DEF-001, 010, 011, 012, 015). Any buffer Neovim enters becomes a tab. The ShaDa
  jumplist is shared across projects. `fixendofline` rewrites binary files. W12 prompts appear
  inside the grid. Terminal exit throws the buffer away. The shell needs a reconciliation layer:
  decide which buffers become tabs, handle `FileChangedShell` and binary detection in PyxForge,
  give each workspace its own ShaDa, and keep the buffers of exited terminals.
- **C. Fixed-metric layout with no constraints** (DEF-006, 007, 008, 016; clipping in 022, 023).
  The layout has constant panel sizes, no splitters, no scroll fallbacks, and no clamp of the
  window to the work area.
- **D. Missing desktop affordances** (DEF-005, 009, 010, 018, 019, 020). There is no menu bar,
  native file dialogs, recent list or tooltips. Shell shortcuts are Ctrl+Shift-only under
  decision K1, and the explorer keeps focus after every kind of open.
- **E. Dialog and message plumbing** (DEF-015, 021; message parts of 022, 023). There are no
  Esc/Enter handlers, notifications cut text off with nothing to expand them, and status codes
  are shown raw.
- **F. Configuration only through `pyxforge.toml`** (DEF-023, FEAT-003, FEAT-004). There is no
  run or debug model beyond project profiles and the QEMU machine.
- **G. Git safety rule too broad** (DEF-022). This is my own regression from `d6325d0`.

## Startup profile summary

Launch to visible window: **433–500 ms** (release build, 4 launches) and 637 ms (profiling build).
Release build at first paint: 155–184 MB working set, 44–46 threads, 0.8–1.0 s process CPU.
Below are the top costs from an 8 s CPU profile taken from launch. On Windows, Go's profiler also samples threads
blocked in system calls, so idle watchers appear as "time" and are listed separately.

| # | Cost | Numbers | Notes |
|---|---|---|---|
| 1 | Toolchain detection (`internal/toolchain` `Detector.detect`) | 2.58 s thread time: 1.90 s in `runVersion` (process start 0.80 s, waiting 1.16 s), 0.69 s in `find` (`os.Stat` across install folders) | Off the UI goroutine, but it checks 19 tools and starts a `--version` process for each one found, at every launch. Caching by executable path and modification time would remove almost all of it. |
| 2 | Window and GL setup (`glfw window.create`, `ShowAndRun`) | 0.46 s, of which 0.19 s is shader compilation (`createProgram`) | Fixed cost inside Fyne's GLFW driver. |
| 3 | Font loading (`painter.CachedFontFace` / `lookupFaces` / `loadSystemFonts`) | 0.09 s CPU at startup; 30.2 MB of idle heap in `opentype.findTableBuffer` alone, out of a 59 MB idle heap | A small time cost at launch; the memory problem appears later (cluster A). |
| 4 | Shell construction (`shell.New` / `build`) | 0.11 s | Widgets, explorer tree, command registration. |
| 5 | Idle watchers (not CPU): `fsnotify` directory watch and Fyne `WatchTheme` | 22.7 s and 7.2 s of blocked thread time over 8 s | Blocking waits that are always running. They are listed only so the profile is not misread. |

Neovim (`nvim --embed`) starts as a separate child process, so its startup is not in the Go
profile.

## Toolchain inventory

Looked up on `PATH` on the audit machine, 2026-10-20. A test that needs a missing tool is
marked SKIPPED.

| Tool | Version | Source |
|---|---|---|
| nvim | NVIM v0.12.6 | Program Files install |
| git | 2.55.0.windows.1 | Git for Windows |
| qemu-system-x86_64 | 11.1.0 | Program Files install |
| gdb | GNU gdb 17.2 (MinGW-w64, WinLibs) | WinGet: WinLibs MinGW-w64 UCRT |
| python | 3.11.9 (also `py` launcher 3.11.9; `debugpy` installed) | python.org per-user install |
| node | v22.23.1 | Program Files install |
| javac | **not installed**: not on `PATH`, `JAVA_HOME` unset, no JDK in the usual install folders | Java tests SKIPPED |
| gcc | 16.2.0 (MinGW-w64 x86_64 UCRT, POSIX threads) | WinGet: WinLibs |
| clang | 23.1.1 (also clangd 23.1.1, lldb 23.1.1) | WinGet: llvm-mingw |
| go | go1.27.1 windows/amd64 (gopls 0.23.0) | per-user Go install |
| rustc / cargo | 1.97.0 / 1.97.0 (rust-analyzer 1.97.0) | rustup |
| nasm | 3.01 | WinGet: WinLibs |
| Also relevant | cmake 4.4.2, ninja 1.13.2; **not installed:** make, dlv (Delve), pyright, typescript-language-server | |

## Proposed fix order

Data loss and regressions come first, then the single fix with the widest effect, then usability
clusters, then features. Each step is one or more single-purpose commits, with tests.

1. **DEF-022**, my regression: allow local `filter.*` / program values that are identical to
   global or system ones, keep refusing everything else, and show the full refusal text. Tests in
   temporary repositories only.
2. **DEF-011**, binary files corrupted on save: detect NUL bytes on read and set `binary`
   (no `fixendofline`). Proposed acceptance: saving an unedited file changes no byte, including
   text files with no final newline.
3. **Cluster A** (DEF-002, 003, 004): stop re-scoping the tab bars on region change. Acceptance:
   the live heap stays within a few MB of baseline over three 20-file cycles, measured with the
   same procedure as here.
4. **DEF-012**, external changes: handle `FileChangedShell` in PyxForge with a Fyne dialog
   (Reload, Keep mine, Compare), keep tab state in step, and move focus to the dialog.
5. **Cluster E** (DEF-021, DEF-015 message parts): Esc/Enter in every dialog, notifications
   that wrap, can be expanded and link to the log, plain-language exit statuses, and keeping a dead
   terminal's output visible.
6. **Cluster C** (DEF-008, 007, 006, 016): clamp the window to the work area, add draggable and
   persisted splitters for explorer, dock and inspector, and make clipped panels scroll or wrap.
7. **Cluster D** (DEF-005, 009, 010, 018, 019, 020): explorer Enter or double-click moves focus to
   the editor; a menu bar with native Open, Save As, New File and Recent; tooltips on every icon
   button. **Revisiting K1 (Ctrl+letter shortcuts) needs your decision first.**
8. **DEF-001**: once the register's steps are known, decide which Neovim-entered buffers may
   create tabs, and give each workspace its own ShaDa.
9. **DEF-023 + FEAT-003**: a run/build configuration UI that writes `pyxforge.toml`, plus
   per-language "Run current file" defaults that use the detected toolchains.
10. **FEAT-004**: a debug-adapter layer (GDB/LLDB for native, debugpy, Node inspector; Delve if
    installed) next to the existing GDB+QEMU path.
11. **FEAT-001/002 polish**: language-server setup guidance in the app, and per-language
    settings.
12. **DEF-004, DEF-017** (unconfirmed parts): repeat with the register's steps after steps 3
    and 5 land.
