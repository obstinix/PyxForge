# Defect register

The record of every PyxForge defect and missing feature found from the Phase 0 audit on. It
supersedes the item list in [defect-map.md](defect-map.md), which stays as the Phase 0 snapshot:
the register file the remediation brief named did not exist, so this one is built from that map
and the owner's reports, and grows as Phase 1 finds more.

**Status:** Confirmed (reproduced, not yet fixed), Fixed, Partially fixed, Not reproduced,
Suspected (a cause or symptom not yet proven), Missing feature, Deferred, Blocked.
**Severity:** S1 loses data or crashes; S2 breaks a workflow; S3 degrades one; S4 cosmetic.
**Priority:** P0 first, then P1, then P2. "Root cause" is stated only when shown by a
reproduction or a test; otherwise it is labelled a hypothesis.

## Summary

| ID | Title | Component | Sev | Pri | Status | Commit |
|---|---|---|---|---|---|---|
| PXF-DEF-001 | Unexpected tabs; editor history crosses projects | Editor/session | S3 | P1 | Not reproduced (startup); related path confirmed | |
| PXF-DEF-002 | Memory grows with every file opened and region switched | UI shell | S2 | P0 | Fixed | 5e598d0 |
| PXF-DEF-003 | Responsiveness | UI shell, toolchain | S3 | P1 | Partially fixed | 5e598d0 |
| PXF-DEF-004 | Stability over long sessions | App | S2 | P1 | Partially fixed | 5e598d0 |
| PXF-DEF-005 | Keyboard focus stays in the explorer after opening a file | Explorer/editor | S2 | P0 | Fixed | bef5fe6 |
| PXF-DEF-006 | Clipped panel content and notifications | UI shell | S3 | P1 | Partially fixed | 17703d4 |
| PXF-DEF-007 | Panels cannot be resized | UI layout | S2 | P1 | Fixed | 3c563a8 |
| PXF-DEF-008 | Window placed partly off-screen | Window | S2 | P1 | Fixed | 3c563a8 |
| PXF-DEF-009 | No Open File, Save As, menu bar | File workflows | S2 | P1 | Fixed | a98ae51, f0ff90d, 3fff970, 7724b6c |
| PXF-DEF-010 | Conventional shortcuts go to Neovim | Shortcuts | S2 | P1 | Fixed | a98ae51, f0ff90d |
| PXF-DEF-011 | Saving a binary file changes its bytes | Editor | S1 | P0 | Fixed | de10543, 7ef2408 |
| PXF-DEF-012 | File changed on disk while edited: prompt in the grid | Editor | S1 | P0 | Fixed | bef5fe6 |
| PXF-DEF-013 | Encodings and line endings | Editor | — | — | Not reproduced (works) | de10543 (test) |
| PXF-DEF-014 | Terminal basics | Terminal | — | — | Not reproduced (works) | |
| PXF-DEF-015 | Shell exit replaces the terminal's output | Terminal | S2 | P1 | Fixed | 8da1538 |
| PXF-DEF-016 | Terminal panel too small | Terminal/layout | S3 | P1 | Fixed | 3c563a8 |
| PXF-DEF-017 | Terminal process lifecycle | Terminal | S3 | P1 | Partially fixed | f2ad857, cc20c81, 8da1538 |
| PXF-DEF-018 | No tooltips on icon buttons | UI kit | S3 | P1 | Fixed | 3fff970 |
| PXF-DEF-019 | Commands only discoverable through the palette | Commands | S3 | P1 | Fixed | 3fff970, 7724b6c |
| PXF-DEF-020 | No recent files or folders | File workflows | S3 | P1 | Fixed | 3fff970, 7724b6c |
| PXF-DEF-021 | Dialogs ignore Esc and Enter; messages cut off | Dialogs | S2 | P1 | Fixed | 25dc08d, 17703d4 |
| PXF-DEF-022 | Git refuses to stage in a normal Git LFS repository | Git | S2 | P0 | Fixed | 4a4fdca |
| PXF-DEF-023 | QEMU/GDB setup not discoverable without pyxforge.toml | Machine | S3 | P1 | Confirmed | |
| PXF-DEF-024 | Closing a tab could select one tab and show another's buffer | Tabs | S2 | P0 | Fixed | 5e598d0 |
| PXF-DEF-025 | Neovim's child processes outlived Session.Close (flaky tests) | Neovim/process | S3 | P1 | Fixed | f2ad857, cc20c81, 8da1538 |
| PXF-DEF-026 | Toast expiry raced the test goroutine in shell tests | Test harness | S3 | P1 | Fixed | d9b4449 |
| PXF-DEF-027 | Explorer cannot reopen the row that is already selected | Explorer | S3 | P1 | Fixed | bef5fe6 |
| PXF-DEF-028 | Windowed build opens a console window for every program it runs | Process/Windows | S2 | P0 | Fixed | 22d587a |
| PXF-FEAT-001 | General-purpose editor | Editor | — | P1 | Partially fixed | de10543 |
| PXF-FEAT-002 | Language-aware editing | Editor/LSP | — | P2 | Partially present | |
| PXF-FEAT-003 | Build and run per language | Build | — | P2 | Missing feature | |
| PXF-FEAT-004 | Unified run and debug | Debug | — | P2 | Missing feature | |

## Items

### PXF-DEF-001 — Unexpected tabs; editor history crosses projects

- **Type / component:** defect (reported) / session restore, Neovim history.
- **Reproduction:** the owner reported about 20–25 tabs opening at startup. A fresh profile opens
  no tabs; a restored session reopens exactly its saved files, three launches in a row; the
  owner's saved states hold 1–3 files each. Related, reproduced: in a restored session, Ctrl+O
  in the editor reopened `file05.txt`, opened only in an earlier session, as a new tab.
- **Expected:** startup opens the saved session and nothing else; editor history stays within
  the workspace.
- **Actual:** startup as expected in every test; history reaches other sessions and projects.
- **Root cause (related path):** every file buffer Neovim enters becomes a tab
  (`editorhost.go` `onEvent` BufEnter), and Neovim's ShaDa (`%LOCALAPPDATA%\pyxforge-data\shada\main.shada`)
  is shared by every workspace; the owner's copy lists files in five folders. The startup
  report itself is unexplained.
- **Planned:** a ShaDa file per workspace; then ask the owner for the exact startup steps.
- **Regression test:** none yet. **Limitations:** startup report not reproduced.

### PXF-DEF-002 — Memory grows with every file opened and region switched

- **Type / component:** defect / `internal/ui/shell` regions, Fyne tabs.
- **Reproduction:** open 20 files from the explorer and close them, three times.
- **Expected:** memory levels off after the first round.
- **Actual (before):** Windows working set 157 → 561 → 894 → 1,027 MB, kept after idle;
  headless live heap 44.8 → 317 → 589 → 861 MB, then 1,244 MB after 30 region switches.
- **Root cause (confirmed):** moving the active-region accent refreshed a Fyne
  `container.ThemeOverride`. In Fyne 2.8.1 each refresh mints a new theme scope, and the font
  cache, keyed by scope and never emptied, parses the whole font set again: 12.95 MB per
  refresh, measured per region. Fyne also drops a widget's scope when its renderer expires, so a
  single, never-refreshed override is no fix.
- **Fix:** `kit.Tabs` replaces DocTabs/AppTabs and draws the accent from its own state.
- **After:** working set 130 → 194 → 197 → 198 MB, 197 MB after 75 s idle; headless 18.7 →
  18.9 → 19.0 → 19.2 MB, 19.2 MB after 30 switches.
- **Regression test:** `TestWorkingDoesNotGrowMemory` (kept 544.5 MB on the old code),
  `TestTabsAccent`. **Limitations:** the first round still grows by about 64 MB (glyph textures,
  Neovim buffers, first load of each font), measured once; overflowing tab rows scroll instead
  of Fyne's "more" menu.

### PXF-DEF-003 — Responsiveness

- **Type / component:** defect / shell, toolchain.
- **Measured:** window in 416–637 ms; region switching and typing 0.34 s of CPU in a 10 s
  profile; the cost was the font re-parse on each file open (DEF-002), now gone.
- **Open:** toolchain detection starts a `--version` process for each of 19 tools at every
  launch (2.58 s of thread time, off the UI thread); a paste can wait up to 2 s for a busy UI
  thread (`editorhost.go` clipboard bridge).
- **Planned:** cache tool versions by path and modification time. **Regression test:** DEF-002's.

### PXF-DEF-004 — Stability over long sessions

- **Status:** partially fixed: the memory growth that would have ended long sessions is fixed
  (DEF-002). No crash or hang occurred in any audit or Phase 1 test. A panel once showed three
  shells where one was expected; not reproduced (DEF-017).

### PXF-DEF-005 — Keyboard focus stays in the explorer after opening a file

- **Reproduction:** click a file in the explorer and type: the keys go to the tree.
- **Expected:** opening a file from the tree (click, Enter) puts the cursor in the editor;
  previewing (Space) may keep the tree focused.
- **Root cause (confirmed):** Fyne's tree focuses itself after its tap handler has run
  (`treeNode.Tapped`: Select, then Focus), undoing any focus change made while opening; and the
  first file's tab was selected, and the editor focused, while the editor pane was still hidden.
  Reproduced headlessly on a2a1307: after opening from the tree, focus was on the tree.
- **Fix:** opening records an intent (focus or not) that is applied when Neovim reports the
  buffer, after the pane is shown; an already-open tab is focused after the current event. Click
  or Enter opens for editing; Space previews and keeps focus in the tree.
- **Regression tests:** `TestOpeningFromTheExplorerFocusesTheEditor` (replays Fyne's tap order),
  `TestOpenPreviewAndActiveRow`, `TestAccentFollowsTheActiveRegion`. Checked in the Windows
  build: click a file, type, and the text goes into the file.

### PXF-DEF-006 — Clipped panel content and notifications

- **Reproduction:** default window size: Git "Recent commits", the terminal's Restart button,
  the QEMU command line, and notification text are cut off.
- **Root cause:** panels lay content out at fixed sizes with no scroll or wrap fallback; a
  toast's body is one line of `kit.Text` in a 360 px toast that disappears after 6 s (10 s for
  errors).
- **Fixed so far (17703d4):** a toast's body wraps to its width; a very long one is shortened
  with a pointer to the Log panel, which gets every message whole; an error stays until
  dismissed. Tests: `TestLongBodiesWrap`, `TestShorten`, `TestLifetimes`.
- **Still open:** the Git "Recent commits" list, the terminal's Restart button and the QEMU
  command line at small sizes (the resizable panels, 3c563a8, give them room, but they do not
  wrap or scroll yet).

### PXF-DEF-007 — Panels cannot be resized

- **Actual:** explorer 256 px, inspector 296 px, dock min(220 px, 40 %) were constants
  (`theme/metrics.go`, `shell/layout.go`); DESIGN_SYSTEM §8 already called the explorer and the
  dock resizable.
- **Fix (3c563a8):** the rules beside the explorer, above the dock and beside the inspector are
  sashes (`kit.Sash`, a 7 px grip around the 1 px rule, resize cursor, double click for the
  default), painted above the panels. Limits: editor at least 360×160, explorer 160, dock 72,
  inspector 220–600. Saved with the workspace. **Regression test:** `TestPanelsResize`.
- **Limitations:** not yet dragged by hand in the Windows build (the harness could not bring the
  window to the front while the machine was in use).

### PXF-DEF-008 — Window placed partly off-screen

- **Reproduction:** launch, close and relaunch: Windows cascades each window 38 px; by the 7th
  launch the 1,406 px tall window sits at y = 266, its bottom 144 px below the 1,528 px work area.
- **Root cause:** a fixed 1440×900 size, Windows' cascading placement, and no clamp to the
  monitor's work area.
- **Fix (3c563a8):** the window is centred on its screen at every launch, and its size (default
  or saved) is kept inside the work area (`SystemParametersInfo`) with room for the frame.
  Measured at 160 %: three launches in a row all open at (189, 80)–(2371, 1486) inside the
  2560×1528 work area. **Regression test:** `TestFitSize`.
- **Limitations:** the work area is the primary screen's; on other platforms the size is not
  clamped (no API without cgo).

### PXF-DEF-009 — No Open File, Save As, menu bar

- **Actual:** only Open Folder (new window) and New File (inside the workspace) exist, both in
  the palette.
- **Fixed so far (a98ae51):** Open File… (Ctrl+O, any path, starting in the active file's
  folder), Save As… (Ctrl+Shift+S: a path form with Browse…, replacing a file asks first; Fyne's
  save dialog is not used because it opens, and so empties, the chosen file before anything is
  written), Close All Editor Tabs (Ctrl+Shift+W, asks about unsaved changes). The Windows build
  crashed on Ctrl+O until the dialog was sized after Show. Tests: `TestSaveAsAndCloseAll`,
  `TestFileDialogsOpen`.
- **Fixed later (f0ff90d):** New File (Ctrl+N) opens an untitled file; every way of saving it asks
  where; the tab then takes the file's name. Test: `TestUntitledFile`. Esc in the dialogs:
  DEF-021 (25dc08d).
- **Fixed later (3fff970, 7724b6c):** a menu bar (File, Edit, View, Go, Run, Help) under the title bar.
  Every item is a registered command, so menus, palette and keys agree; shortcuts are shown,
  not bound, because a Fyne main menu would run them before the focused widget sees the key
  and take Ctrl+W from a shell (KEYMAP.md). Menus are built when they open, and what cannot
  run now is greyed out. Edit acts on the focused editor (through Neovim: undo, redo, the
  system clipboard register) or text field. The first Windows check showed the workspace
  through Fyne's pop-up menu; the menus now sit on PyxForge's overlay material (7724b6c).
  Tests: `TestMenus`, `TestEditMenuInTheEditor`. Recent: DEF-020.
- **Checked in the Windows build:** the menu bar, an opaque File menu with its shortcuts, Esc
  on Save As.

### PXF-DEF-010 — Conventional shortcuts go to Neovim

- **Actual:** Ctrl+N/O/W/P go to Neovim (Ctrl+O walks the jumplist into old files, DEF-001);
  shell commands are Ctrl+Shift chords only (decision K1).
- **Decision:** the owner's Phase 1 brief changes K1: Ctrl+N, O, S, W, F, H, P and Ctrl+Tab
  become application shortcuts, routed before Neovim.
- **Fixed so far (a98ae51):** decision K2 (KEYMAP.md). In the editor Ctrl+O, P, S, W and
  Ctrl(+Shift)+Tab run PyxForge's commands, routed by one function so each runs once; Ctrl+Shift
  chords everywhere; the terminal keeps every plain-Ctrl key; Alt and AltGr never bound; "Toggle
  Neovim Keys in the Editor" gives the plain-Ctrl keys back. Tests: `TestKeyboardPolicy`,
  `TestEditorShortcutRunsOnce`; checked in the Windows build.
- **Fixed later (f0ff90d):** Ctrl+N (New File), Ctrl+F (Find) and Ctrl+H (Find and Replace): a
  find bar under the file (literal text, case, whole word, count, Replace, Replace All as one
  undo step; Neovim's search pattern, so n and N go on). Tests: `TestFindAndReplace` (also under
  Neovim 0.9.5), `TestFindBar`.
- **Limitations:** Ctrl keys while a Fyne text field has focus go to that field (Fyne gives a
  focused entry every shortcut).

### PXF-DEF-011 — Saving a binary file changes its bytes

- **Reproduction:** open a 1,024-byte binary file, save without editing: 1,025 bytes, 0x0A
  appended. A text file without a final newline was given one the same way.
- **Root cause (confirmed):** Neovim read every file as text, and its default 'fixendofline'
  added the missing final newline.
- **Fix:** `internal/textfile` (Git's rule: a NUL byte in the first 8000 bytes and no Unicode
  BOM) sends binary files to a tab of their own (facts, hex dump, "Open as Text (Read-Only)");
  `nvim/lua/pyxforge/files.lua` reads any binary buffer in binary mode, read-only;
  'fixendofline' is off.
- **Regression tests:** `TestSavingKeepsEveryByte` (fails on the old configuration for binary and
  no-final-newline files), `TestDetect`, `TestBinaryFilesAreNotEditedAsText`.
- **Follow-up (7ef2408):** files.lua used `vim.uv`, which Neovim 0.9 (CI's Ubuntu runner,
  PyxForge's minimum) lacks; CI failed on a2a1307 until it fell back to `vim.loop`. The Neovim
  tests pass under 0.9.5 (WSL) and 0.12.6.
- **Limitations:** UTF-16/32 without a BOM counts as binary (Neovim could not read it either);
  binary files are not editable in a hex editor.

### PXF-DEF-012 — File changed on disk while edited: prompt in the grid

- **Reproduction:** edit a file without saving, change it on disk, return to the editor:
  Neovim's W12 prompt appears inside the grid, the tab keeps its old title, focus stays in the
  explorer.
- **Expected:** a PyxForge dialog offering Reload, Overwrite or Decide Later; nothing lost
  silently. A clean buffer reloads silently (this works).
- **Root cause (confirmed):** no FileChangedShell handling: Neovim asked in its own grid.
- **Fix:** a FileChangedShell handler in the session bridge reloads clean buffers; with unsaved
  changes, or a deleted file, the buffer is left as it is and PyxForge asks: Reload from Disk,
  Overwrite Disk, Keep Editing. Save, Save All, close and quit with save, and Ctrl+S go through
  `pyxforge.save`, which asks again instead of writing. Files are checked when the window comes
  to the front and when the explorer's watcher sees them change; a stale report is dropped.
- **Regression tests:** `TestFileChangedOnDisk` (clean reload, conflict kept, Save/SaveAll/
  CloseBuffer refused, Overwrite, Reload, deleted), `TestChangedOnDiskAsksBeforeLosingChanges`.
  Checked in the Windows build: the dialog appears, Reload from Disk shows the new text.
- **Limitations:** a plain `:w` typed in Neovim during a conflict still gets Neovim's own
  question (its protection is kept, not bypassed); no Compare (diff) choice yet; files in
  folders the explorer has not listed are checked only on return to the window or the tab.

### PXF-DEF-013 — Encodings and line endings (works)

- Edited and saved CRLF and UTF-16 LE files kept their bytes in Phase 0; Phase 1's
  `TestSavingKeepsEveryByte` now saves LF, CRLF, UTF-8 BOM, UTF-16 LE/BE BOM and Latin-1 files
  unedited and compares every byte.

### PXF-DEF-014 — Terminal basics (works)

- cmd.exe in a real ConPTY through Neovim's `:terminal`, working folder = workspace (non-ASCII
  path included), stdout and stderr shown, resize reflows, Ctrl+C stops a command.

### PXF-DEF-015 — Shell exit replaces the terminal's output

- **Reproduction:** run a command, `exit`: the transcript is replaced by "The shell exited with
  status …"; one shell showed status -1073741510 (0xC000013A, STATUS_CONTROL_C_EXIT) unexplained.
- **Root cause (confirmed):** on TermClose the panel swapped the terminal view for a placeholder
  (`terminal.go`); an exited Neovim terminal cannot simply stay, as any key in terminal mode
  closes it.
- **Fix (8da1538):** the exited terminal's text, scrollback included, becomes a read-only buffer
  in the same place (`Session.KeepTranscript`), under a line saying how the shell ended, with
  Restart. Windows statuses are named (0xC000013A: ended by Ctrl+C, Ctrl+Break or its console
  closing; access violation and stack overflow are crashes; other negatives in hex).
- **Regression tests:** `TestKeepTranscript`, `TestTerminalSessions` (transcript and exit line).
- **Limitations:** the transcript keeps the text, not the colours.

### PXF-DEF-016 — Terminal panel too small

- About 8 rows at the default size; fixed dock height (DEF-007). **Fixed (3c563a8):** the dock
  is resized by dragging its sash, up to all but 160 px of the editor, and keeps that size.

### PXF-DEF-017 — Terminal process lifecycle

- Ctrl+C and quitting leave no orphans (job object). A panel once showed three shells; not
  reproduced. Closing a terminal Neovim did not wait for its child processes (DEF-025, fixed).

### PXF-DEF-018 — No tooltips on icon buttons

- `kit.IconButton.Label` was only the accessible name and the palette title.
- **Fix (3fff970):** Fyne 2.8 has no tooltips, so `kit.Tooltips` shows a hovered icon button's
  label, with its shortcut, after 600 ms, below the button or above it when there is no room.
  It is a layer of the window's content, not a canvas overlay, which would take the keyboard
  from the editor while the pointer rests on a button. Test: `TestTooltips`. Checked in the
  Windows build: "New Terminal (Ctrl+Shift+`)" under the dock's + button.

### PXF-DEF-019 — Commands only discoverable through the palette

- The palette (Ctrl+Shift+P) was the only list of commands and shortcuts.
- **Fix (3fff970, 7724b6c):** the menu bar (DEF-009) lists the commands with their shortcuts; Help >
  Keyboard Shortcuts opens KEYMAP.md.

### PXF-DEF-020 — No recent files or folders

- **Fix (3fff970, 7724b6c):** the last ten files and folders opened, per user, newest first. File >
  Open Recent… lists them in the palette, to pick or search, with Clear Recent; a file opens
  in this window, a folder in a new one. A path gone since is said so and taken off the list.
  Greyed out while the list is empty. Test: `TestMenus`. Checked in the Windows build.

### PXF-DEF-021 — Dialogs ignore Esc and Enter; messages cut off

- **Reproduction:** "Save changes?" ignores Esc; New File ignores Esc and Enter; error toasts end
  mid-sentence; terminal statuses are raw numbers.
- **Root cause (confirmed):** Fyne's dialogs answer neither key; the window's key hooks run only
  while no control has focus, and a focused entry swallows Esc.
- **Fix (25dc08d):** every dialog goes through `present`, a stack of the dialogs showing: Esc
  closes the one in front, Enter runs its default action; form fields (`dialogEntry`) pass Esc
  on and submit on Enter. Messages: see DEF-006 (17703d4).
- **Regression tests:** `TestDialogsAnswerTheKeyboard`, `TestDialogKeysDoNotReachTheEditor`.
- **Limitations:** Esc does nothing while a dialog's button has keyboard focus (after Tab).
  Checked by hand in the Windows build: Esc closes Save As.

### PXF-DEF-022 — Git refuses to stage in a normal Git LFS repository

- **Reproduction:** a repository whose `.git/config` repeats the system `filter.lfs.*`
  settings (what `git lfs install --local` writes): Stage fails, blaming the project.
- **Root cause (confirmed):** `d6325d0` treated every local-scope program setting as the
  project's own.
- **Fix:** a repository setting that repeats a global or system one, key and value, is the
  user's own; anything else is still refused, and the refusal names the user's own value.
- **Regression tests:** `TestRepeatedUserFiltersAreNotRefused` (6 cases; fails on `d6325d0`
  for the repeated settings), `TestGitLFSRepositoryStagesAndCommits` (real git-lfs 3.7.1,
  checks the committed LFS pointers), and the existing D14 tests.
- **Limitations:** a folder that is not a repository offers no "Initialize" action (step 10).

### PXF-DEF-023 — QEMU/GDB setup not discoverable without pyxforge.toml

- With `pyxforge.toml` the boot-sector example debugs (stops at 0x7c00, registers, Stop ends
  QEMU and GDB). Without it there is a short-lived, cut-off notification and no way to configure
  a machine; the resolved QEMU command line is clipped. **Planned:** step 11.

### PXF-DEF-024 — Closing a tab could select one tab and show another's buffer

- **Reproduction (Fyne 2.8.1, verified):** DocTabs a, b, c with c selected; remove a: b is
  selected while OnSelected reports c. The editor follows OnSelected, so Neovim showed c's
  buffer under b's tab.
- **Root cause (confirmed):** Fyne's `removeIndex` re-selects the last item through `setItems`
  and then shifts the index again.
- **Fix:** `kit.Tabs` keeps the selected tab when another is removed and reports a change only
  when the selected tab really changes. **Regression test:** `TestTabsSelection`.

### PXF-DEF-025 — Neovim's child processes outlived Session.Close (flaky tests)

- **Reproduction:** `TestTerminalRestart` and `TestTerminalSessions` failed now and then
  (baseline: 2 of 6 and 1 of 10 runs) with "TempDir RemoveAll cleanup … being used by another
  process".
- **Root cause (confirmed):** `Session.Close` waited for Neovim but not for the processes it had
  started; a terminal's console host and shell end a moment after Neovim, still in the session's
  folder. With the old Close a Neovim child was still running right after Close in 9 of 10 runs.
- **Fix:** Close stops the terminal jobs, records Neovim's process tree (`proc.Descendants`,
  Windows), closes Neovim, gives the tree a second, then terminates whatever still runs under
  Neovim's process ID (`proc.Terminate`), including what a shell started after the first look (a
  git-aware prompt's `git`, subshells): waiting alone still failed now and then, after the full
  five seconds Windows gives a console program to handle its console closing.
- **Regression test:** `TestCloseWaitsForChildProcesses`; with the final fix the terminal tests
  passed 20 of 20 repeated runs, and `go test ./...` twice in a row with no failure.
- **Follow-up (8da1538):** under Linux the test failed one run in three, as Descendants was
  Windows-only. It now reads /proc on Linux, and the terminal jobs' process IDs come from Neovim
  on every platform (macOS included). Under Linux (WSL) the terminal tests passed 10 of 10.
- **Limitations:** on macOS a shell's own children are not listed (no Descendants there); the
  shell itself is ended.

### PXF-DEF-026 — Toast expiry raced the test goroutine in shell tests

- **Reproduction:** a shell test running longer than 6 s panicked in
  `notifications.stackLayout.Layout` from a timer goroutine.
- **Root cause (confirmed):** toasts expire through `fyne.Do` from a timer goroutine; Fyne's test
  driver runs `fyne.Do` inline on the calling goroutine (`test/driver.go` `DoFromGoroutine`), so
  the expiry edited widgets the test goroutine was using. The app's GLFW driver queues the same
  call on the UI thread: not an application defect.
- **Fix:** shell tests whose dispatch runs work inline keep toasts up (`noToastExpiry`).

### PXF-DEF-027 — Explorer cannot reopen the row that is already selected

- **Root cause (confirmed by code and test):** Fyne's tree does not report a tap on the
  selected row (`Tree.Select` returns early), so after switching to another tab, clicking the
  first file in the explorer did nothing.
- **Fix:** no row stays selected; the file shown in the editor is drawn with the selection wash
  (`Explorer.SetActive`), following the selected tab. **Regression test:**
  `TestOpenPreviewAndActiveRow`.

### PXF-DEF-028 — Windowed build opens a console window for every program it runs

- **Found:** while preparing the Git workflow work (step 10), by running the build the release
  ships (`-H windowsgui`, as `tools/winpkg` makes `PyxForge.exe`) on a small repository and
  recording every top-level window that appeared. The test builds used until then are console
  programs, whose children share their console, so the earlier GUI checks could not show it.
- **Reproduction:** start the windowed build on a Git repository. In 12 s, Windows opened about
  70 windows: a Windows Terminal window for each toolchain probe (git, gcc, ld, nasm, make,
  objdump, gdb, three QEMUs, qemu-img, nvim, clangd, gopls, rust-analyzer, asm-lsp,
  tree-sitter) and another for each Git status refresh. Builds, QEMU, GDB, `taskkill` and the
  Neovim setup commands start the same way.
- **Root cause (confirmed):** a program without a console that starts a console program makes
  Windows give the child a new console, shown as a window (Windows Terminal by default on
  Windows 11), unless the child is created with `CREATE_NO_WINDOW`. No start site set it; the
  embedded Neovim was the exception (go-client starts it hidden).
- **Fix (22d587a):** `proc.NoWindow` adds `CREATE_NO_WINDOW` (a no-op elsewhere), called at every start
  site: git, the toolchain probes, builds and their `taskkill`, GDB, QEMU and qemu-img, the
  Neovim setup commands, and `StartDetached`. The child still has a console, which its own
  children share, and its output still reaches PyxForge's pipes. GDB is interrupted through
  MI, so no console signal is needed.
- **Regression tests:** `TestNoWindow` (the flag is added to those already set; output still
  arrives), `TestEveryChildHidesItsConsole` (every function in `internal/` and `cmd/` that
  builds on Windows and calls `exec.Command*` also calls `NoWindow`; removing one call fails it).
- **Checked in the windowed build:** after the fix, starting on the repository, opening a file
  (Neovim), opening a terminal and showing the Git tab opened no window but PyxForge's own.

### PXF-FEAT-001 — General-purpose editor

- Embedded Neovim edits any text file, encodings kept (DEF-013), binary files safe (DEF-011).
  New File, Open File, Save As, Open Recent, menus (DEF-009, DEF-020) and the external-change
  dialog (DEF-012) are in.

### PXF-FEAT-002 — Language-aware editing

- Tree-sitter highlighting and LSP where a server is installed (clangd, gopls, rust-analyzer
  here; no pyright, typescript-language-server or JDK). No in-app server setup.

### PXF-FEAT-003 — Build and run per language

- Builds run `pyxforge.toml` profiles only; no "Run current file" or per-language defaults.

### PXF-FEAT-004 — Unified run and debug

- Debugging is GDB attached to QEMU only; no host, Python, Node, Go or Rust debugger.
