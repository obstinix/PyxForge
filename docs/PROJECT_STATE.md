# PyxForge project state

The authoritative record of where PyxForge 3.0 stands: what is verified, what is built but
not yet proven, what is being worked on and what comes next. Update it at the end of every
working session. The code is the final authority: when this file and the code disagree, fix
this file. Decisions and their reasons are in [`ARCHITECTURE_DECISIONS.md`](ARCHITECTURE_DECISIONS.md);
per-feature evidence by platform is in [`FEATURES.md`](FEATURES.md).

Status labels: **VERIFIED** (a test or a recorded check on the named platform proves it),
**IMPLEMENTED BUT UNVERIFIED**, **IN PROGRESS**, **BLOCKED**, **PLANNED**.

## Snapshot (2026-11-27)

| | |
|---|---|
| Version | 3.0.0 release candidate. Published pre-release: `v3.0.0-rc.2` (2026-10-20). `CHANGELOG.md` has 3.0.0-rc.3, not yet tagged. |
| Branch | `main`, pushed directly (no pull requests). |
| Latest pushed commit | See `git log origin/main`; the session log below names each milestone's commit. |
| Last verified CI | `ce4429c` (2026-11-27): every job passes on Windows, Ubuntu and macOS. CI had been red from `de4b45b`: two racy tests (`b1b1c1a`), the clean-Ubuntu step still expecting Doctor to name Neovim (`474fb1b`), and paths spelled with 8.3 short names on the Windows runner (`4afaf33`); see session 7. Release workflow on `7d29010` (run 45698159264, 2026-11-25) passed: the native build's installer and portable zip built, installed, checked and uninstalled on a runner and in a clean Windows Server Core container. |
| Phase | 3.1 identity prompt (D21–D25): Phase 1 (component kit) done; Phase 2 (Settings modal) next. Before it, 3.0 completion after D16–D19 (Neovim gone, own terminal, flat design): workspace and editor features (setup wizard, tab management, status-bar encoding, Git side-by-side diff, references and symbols, split editor groups done; outline sidebar, bookmarks, Markdown preview, project graph next). |
| Platforms | Windows 11 x64 and Ubuntu 24.04 x64 released; macOS compile-checked only (21 test failures on its first run, #89). |

## Architecture today

One Go program, `pyxforge` (`cmd/pyxforge`): a Fyne desktop app and a command-line tool.

| Layer | Packages |
|---|---|
| Desktop shell | `internal/ui/shell` (workbench, panels, Settings, dialogs), `internal/ui/{explorer,commandpalette,notifications,kit,theme,icons,brand,splash}` |
| Editor (D16; Neovim removed 2026-11-24) | `internal/textbuf` (document), `internal/syntax` (Chroma highlighting), `internal/ui/codeedit` (widget), `internal/lsp` (language-server client) |
| Terminal | `internal/pty` (ConPTY, Unix pty), `internal/vt` (emulator), `internal/term` (session), `internal/ui/termview` (widget): D17 |
| Project and build | `internal/config` (`pyxforge.toml`), `internal/workspace`, `internal/build`, `internal/runfile`, `internal/templates` |
| Run and debug | `internal/qemu`, `internal/gdb`, `internal/inspect`, `internal/snapshot`, `internal/listing` |
| Source control | `internal/git`, `internal/askpass` |
| Platform and setup | `internal/toolchain` (detection, Doctor), `internal/managed` (Dependency Manager), `internal/proc`, `internal/update`, `internal/trash`, `internal/textfile` |
| Packaging | `tools/winpkg` (Inno Setup installer, portable zip, bundled MinGit), `tools/linuxpkg` (`.deb`, AppImage), `tools/release` |

## Status by area

### Editor

| Item | Status | Evidence |
|---|---|---|
| Native editor: document model (undo groups, atomic save keeping CRLF and BOM, refuses binary) | VERIFIED (Windows; CI runs it on Linux) | `internal/textbuf` tests |
| Native editor: Chroma highlighting, 16 file types PyxForge opens | VERIFIED | `TestHighlightC`, `TestLanguagesPyxForgeOpens`, `TestHighlightSpeed` (5,000 lines in about 0.18 s) |
| Native editor widget: typing, auto-indent, auto-pairing, bracket matching, folding, find and replace, comment toggle, selection, clipboard, mouse, completion list, information box, diagnostics | VERIFIED in tests and a real window (Windows) | `internal/ui/codeedit` tests; `go run ./tools/editorspike` |
| LSP client: diagnostics, completion, hover, definition, references, rename, formatting, start-up timeout | VERIFIED against clangd 23.1 (Windows) | `TestClangd`, `TestServerThatNeverAnswers` |
| Native editor wired into the shell behind the editor setting (Settings > Editor, or the Use PyxForge's Editor command; takes effect at the next start): tabs, open, Save, Save As, Save All, Close and Close All with the unsaved-changes question, quit, New File, Find and Replace bar, Go to Line, breakpoints (gutter click or Toggle Breakpoint; they follow inserted lines), the debugger's stopped line, build and language-server diagnostics in the editor and in Problems, Run and Debug Current File, reload when the file changed on disk, text size, status bar | VERIFIED in tests and review renders (Windows); not yet tried in the packaged app | `TestNativeEditorInTheShell`, `TestNativeEditorLanguageServer`, `go run ./tools/snapshot -only editor-native` |
| Language features in the native editor: completion while typing or Ctrl+Space, Go to Definition (F12), Show Hover Information (Ctrl+K), Rename Symbol (F2), Format Document (palette), Toggle Line Comment (Ctrl+/), F9 breakpoints, folding from the gutter or the palette | F9 and Ctrl+/ VERIFIED (`TestNativeEditorInTheShell`); F12, Ctrl+K and rename VERIFIED against clangd (`TestNativeEditorLanguageServer`); Format Document IMPLEMENTED BUT UNVERIFIED in the shell | `KEYMAP.md` |
| Language servers per language through `internal/lsp` (`lsp.Servers`, the Go port of `nvim/lua/pyxforge/lsp.lua`: clangd, gopls, rust-analyzer, asm-lsp, lua-language-server, taplo, marksman, bash-language-server, pyright, typescript-language-server) | VERIFIED for clangd; the others are chosen by `TestServerTable` but not run | `TestServerTable` |
| Editor tabs: right-click menu (Close, Close Others, to the Right, Saved, All; Pin; Copy Path; Reveal), pinned tabs that bulk closes leave open, moving by drag or the palette, pins and order restored with the workspace | VERIFIED in tests (Windows, Ubuntu under WSL); drag only with synthetic events, not yet by hand | `TestTabsPinAndMove`, `TestEditorTabManagement`, `TestPinnedTabsRestore` |
| Side-by-side Git diff (`internal/diff`, Myers' algorithm): unstaged against the index, staged-only against HEAD, untracked against nothing; change navigation, Refresh, Open File; Show Changes in Current File in the palette | VERIFIED in tests and renders (Windows; Ubuntu under WSL with `-race`); not yet by hand | `TestGitDiffView`, `internal/diff` tests, `go run ./tools/snapshot -only git-diff` |
| Find References (Shift+F12) and Go to Symbol in File (Ctrl+Shift+O) in the palette; `lsp.Client.DocumentSymbols` | VERIFIED against clangd 23.1 (Windows); other servers not run | `TestNativeEditorLanguageServer`, `TestClangd`, `TestSymbolForms` |
| Split editor groups (D20): up to three, a file in one group at a time; restored with the workspace | VERIFIED in tests and a render (Windows; Ubuntu under WSL with `-race`); not yet by hand | `TestEditorGroups`, `TestEditorGroupsRestore` |
| Encoding and line endings in the status bar (display only) | VERIFIED in tests (Windows; Ubuntu under WSL) | `TestEncodingAndLineEndingNames` |
| Not yet in the native editor: converting encodings or line endings, Java's jdtls, multi-cursor, the same file in two groups (D20), an outline sidebar, project-wide search results inside the editor | PLANNED | |
| Native editor as default, Neovim removed | VERIFIED (D16 steps 3–5; `928c1b4`, `5841f0d`, `a4b8b81`) | |

`tools/editorspike` on Windows 11, 2026-11-24: a 10,000-line file opens and draws in 0.23–0.59 s;
paging through it, 271 pages, takes 0.53–0.64 ms per page, key and frame; a key typed 0.14 ms;
undo, save, a second document in its own tab; clangd's diagnostics 0.14–0.16 s after start,
completion, hover, go to definition; clangd ends with the window.

### Terminal

| Item | Status | Evidence |
|---|---|---|
| Native terminal (D17): ConPTY / Unix pty, emulator with 10,000 lines of history, theme colours | VERIFIED (Windows 11 and Ubuntu under WSL, `-race`) | `TestProgramOutputAndStatus`, `TestTypingIntoAShell`, `TestResizeRightAfterExit`, `internal/vt` tests, `TestOutputColourAndPosition`, `TestShellSession`, `internal/ui/termview` tests |
| Sessions, restart, output kept after exit, Run Current File and Debug Current File in the panel | VERIFIED (Windows 11 and Ubuntu under WSL) | `TestTerminalSessions`, `TestRunCurrentFile` (Python input, C, GDB at main and at an F9 breakpoint) |
| Neovim crash (0xC0000005) on Windows when a program exits (#80) | Fixed by D17: the panel runs no Neovim; its stress test went with Neovim's terminal code | CI run of the D17 commits |
| The terminal by hand in the packaged app | IMPLEMENTED BUT UNVERIFIED (screen access declined) | |

### Interface

| Item | Status | Evidence |
|---|---|---|
| Workbench, command palette, Settings, notifications, keyboard policy, type scale, editor and terminal text sizes | VERIFIED | `TestKeyboardPolicy`, `TestTextSizes`, snapshot renders |
| Flat redesign: no glass, no backdrop gradients, Ink & Glass retired (D18) | VERIFIED (renders on Windows, `TestFlatOverlays`) | `4a68e97` |
| Low-poly / line illustrations for Welcome and empty states | PLANNED | |
| Kit controls (identity prompt Phase 1): Button (Default, CTA, Ghost, Danger, Icon), Switch, SettingRow, SettingGroup, SearchField, NavList, Dropdown, Slider; every shell button a `kit.Button` | VERIFIED in tests and renders (Windows; Ubuntu under WSL with `-race`); not yet used by hand in the packaged app | `TestComponentContrast`, `TestButton`, `TestSwitch`, `TestSettingRowsAndGroups`, `TestSearchField`, `TestNavList`, `TestDropdown`, `TestSlider`, `TestShellUsesKitButtons`; `kit-gallery-*` in all eight theme and accent combinations |

### Setup, packaging, release

| Item | Status | Evidence |
|---|---|---|
| Doctor, Dependency Manager (GCC, GDB, NASM, binutils, Git on Windows), first-run prompt | VERIFIED (Windows CI with local test servers) | `TestDoctor`, `TestDependencyManager`, `TestFirstRunSetup` |
| First-start setup wizard (Help > Setup Wizard…): profile, tools found or missing, managed installs with progress, first project | VERIFIED in tests (a managed install from a local test server); not yet tried by hand | `TestSetupWizard` (`d802872`) |
| Windows installer and portable zip with MinGit bundled (native build) | VERIFIED in a clean Windows Server Core container (release workflow run 45698159264 on `7d29010`) | `release-windows.yml`, `.github/scripts/clean-machine.ps1` |
| `.deb` on a clean Ubuntu 24.04 container: installs, Doctor, the window opens, removes | VERIFIED (CI) | `.github/scripts/clean-linux.sh` |
| AppImage | VERIFIED (CI runs it) | `ci.yml` |
| Code signing | BLOCKED: needs a certificate (owner) (#33) | |
| v3.0.0-rc.3 published | PLANNED: owner's call | |

### Repository

| Item | Status |
|---|---|
| Decision and state records consolidated into this file and `ARCHITECTURE_DECISIONS.md`; 2.x ADRs moved to `docs/archive/2.x/adr/` | VERIFIED (2026-11-24) |
| `legacy/` (2.x Tauri, VS Code extension, Rust core) removed with its CI job (D19) | VERIFIED (2026-11-24) |
| `pyforge_ui_kit/` | VERIFIED removed (`3aafa99`, by the owner) |
| Stale audit documents; `docs/antislop.md`, which still names 2.x files (`desktop/src`, `scripts/lint-slop.sh`) that no longer exist | PLANNED |
| README rewritten from the implementation, with an architecture diagram | VERIFIED (`181ccee`) |

## Known bugs and failing tests

- #80: fixed by D17 (the Terminal panel no longer runs Neovim); close the issue once CI on the
  D17 commits is green on Windows.
- #89: 21 tests fail on macOS (compile-checked platform).
- #52: a click right after the window is activated is sometimes ignored on Windows.
- #64: Rust with the MSVC target cannot be debugged with GDB.
- CI flake (2026-11-27, `4afaf33`): in the clean-Ubuntu step, `xvfb-run` failed to clean up its
  temporary folder and exited 5 in place of the window's 124 (still running when stopped); the
  same step passed on the next commit. If it repeats, start `Xvfb` directly instead of `xvfb-run`.

## Session log

- **2026-11-24, session 1.** Editor research and prototype (D16), decisions D16–D18, these two
  records. Three commits made locally; the push waited on the full race-enabled suite.
- **2026-11-24, crash recovery.** The Claude Code session crashed while that suite ran, so
  nothing reached GitHub. Recovered: the three commits were intact in the local `main`;
  GitHub had four newer web-UI commits from the owner (deleting `code.html` mock-ups and the
  Ink & Glass design notes under `pyforge_ui_kit/`). Rebased onto them without conflicts,
  re-ran vet, staticcheck, the forbidden-technology check and the race tests of the new
  packages (all pass), and pushed: `7b3ab63`, `e1000f7`, `ad250d8`. The crash also left the
  Go build cache (`%LOCALAPPDATA%\go-build`) with broken entries ("too many levels of
  symbolic links", "can't find export data"); it was deleted and rebuilt. The full
  `go test -race ./...` from before the crash never completed: re-run it.

- **2026-11-24, session 2 (after the recovery).** D18 flat redesign (`4a68e97`), D19 removal of
  `legacy/` (`acc9dca`, `140bf98`, `daae7df`), the native editor in the shell (`9665b8b`,
  `ea3fa36`, `415a0d6`, `eca0755`), one path spelling for language servers after a Windows CI
  failure (`7f97328`), and `PYXFORGE_EDITOR` (`3b34650`). The full race suite passed locally on
  Windows after the D18 test fix (48 packages). CI: Ubuntu passes from `daae7df`; Windows on
  `daae7df` failed only `TestClangd` (8.3 temporary path), fixed by `7f97328`, whose run was still
  going when the session ended. Not done: driving the packaged app by hand with the native
  editor (screen access was declined), so the packaged-app check is still open.

- **2026-11-24, session 3.** CI on session 2's commits failed only on Windows, in
  `TestCloseAfterResizeWithATerminal` (#80, Neovim's own crash). The hands-on check of the
  native editor needs screen access, declined, so D17 went first. `fyne-io/terminal` was
  prototyped and rejected (no scrollback; see D17). Built `internal/pty`, `internal/vt`,
  `internal/term` (`d337658`) and `internal/ui/termview` (`fac8593`), then moved the Terminal
  panel onto them and removed Neovim's terminal code with its tests, the #80 stress test
  included. Verified with `-race` on Windows 11 and on Ubuntu under WSL: the three packages,
  the widget, `TestTerminalSessions` and `TestRunCurrentFile` (Python reading input, C, GDB at
  main and at an F9 breakpoint, a compile error); the full suite on Windows. The widget was
  rendered offscreen in all four themes and looked at; box-drawing borders are drawn as lines
  so they join. Not done: the terminal by hand in the packaged app (no screen access).
  CI on `7dc405c`: Ubuntu, macOS, Lua and govulncheck pass; Windows failed only
  `TestShellSession`, a timing fault in the new test (Clear while the prompt was still
  arriving), fixed in `ff654da`, whose CI run passes on every platform.

- **2026-11-24, session 4.** The owner granted screen access. A fresh build was put in the
  installed PyxForge folder (the original exe backed up and restored afterwards) and run with
  `PYXFORGE_EDITOR=native` on a C project. Worked by hand: Go to File opens `main.c`; typing
  marks the tab modified; clangd underlines an undeclared name, offers completions, and
  Problems counts it; F12 goes to the definition; Ctrl+S saves and clears the mark; the
  Terminal starts cmd.exe in the project, compiles and runs the program ("y is 42"), Ctrl+C
  stops `ping -t`, the wheel scrolls back, `exit 3` shows "The shell exited with status 3."
  with Restart; File > Exit ends PyxForge, clangd and the shell. Found and fixed: the status
  bar said "Neovim: not attached" with the native editor; an ended terminal still drew its
  cursor. Not settled: clicks on an Explorer row and on the title bar's Close did nothing
  under the automation, in the installed release (Neovim) too, while the same tap opens the
  file in a test; treat as #52 until the owner sees otherwise by hand. Then the native editor
  became the default (D16 step 3); the Neovim tests now ask for Neovim explicitly. Commits `c2dda14`,
  `928c1b4`; CI green on the rerun (see Snapshot).

- **2026-11-24, session 5.** D16 step 5 (most of it): Neovim removed. Gone: `internal/neovim`,
  `internal/ui/editor`, `nvim/`, `tools/nvimspike`, the shell's Neovim host, the editor-engine
  setting, `pyxforge setup editor`, Neovim and tree-sitter in Doctor, the bundled Neovim and
  Visual C++ runtime in the Windows package, `neovim` in the .deb's Recommends, the Lua CI job.
  Moved to the native editor: the changed-on-disk question (Reload, Overwrite, Keep Editing; Save
  asks again), Rename and Trash of open files, UTF-16 files (with a byte-order mark), binary files
  opened as text read-only, language servers that end with the editor even mid-start. The
  Neovim-only tests were ported or removed. Gaps: the Git tab's diff opens the file only (no
  side-by-side view yet); a new file gets LF on every platform and no final newline.
  `pyxforge clean` removes the old Neovim configuration. Verified on Windows: gates and the
  full race suite. Not done yet: Linux run of the shell tests, CI on the commit, the CI
  workflow's Neovim install and crash-dump steps, README/INSTALL/KEYMAP text about Neovim.

- **2026-11-26, session 6 (master transformation prompt, installer first).** README rewritten
  for the native architecture (`181ccee`); install guide, keymap, contributing and roadmap brought
  up to date and the Linux CI test fixed (`de4b45b`). The last Neovim leftovers removed (`a4b8b81`):
  the "Toggle Neovim Keys" command (left on, it blocked Ctrl+S and the other conventional keys in
  the native editor), CI's Neovim installs and crash catcher, `tools/crashdump`. The release
  workflow's install check and `clean-machine.ps1` now expect the native build (Git bundled, no
  `nvim` folder). Verified locally: `go run ./tools/winpkg -version 3.0.0-dev.1` builds the
  installer (50 MB) and the portable zip (76 MB), and its checks pass (byte-identical builds,
  bundled Git runs, no missing DLLs). Not done: the clean Windows container run (the release
  workflow, run by hand with a version only), which needs the owner's go-ahead to trigger; the
  installer was not installed on the development PC, where it would upgrade the owner's copy.
  The release workflow then ran with the owner's go-ahead and passed, clean container included.
  Also: the first-start setup wizard (`d802872`) and editor tab management (`2ba369d`).

- **2026-11-27, session 7 (fresh clone).** CI had been red on `main` since `de4b45b`: on Ubuntu
  `TestTerminalSessions` compared a drawn frame with a later screen when the prompt arrived after
  the output, and on both platforms `TestPinnedTabsRestore` raced with a language server it
  started without the test dispatch queue. Both tests fixed (`b1b1c1a`), verified with `-race`
  on Windows 11 and Ubuntu under WSL. CI on `b1b1c1a` then passed on Windows and macOS but failed
  the clean-Ubuntu step, which still required Doctor to report Neovim missing; it now expects Git,
  which the `.deb` only recommends (`474fb1b`, checked under WSL). This record brought up to date with the code. Then:
  encoding and line endings in the status bar (`3ecc95a`); the line-diff package (`3b401b7`) and
  the Git tab's side-by-side diff with Show Changes in Current File (`4879db1`), looked at in
  renders in Smoked Kraft and Ink & Paper. Document symbols in the LSP client (`7fdbff7`) and Find
  References and Go to Symbol in File (`752035c`), checked against clangd. CI on `4879db1` showed
  that a path spelled with a Windows 8.3 short name (the runner's TEMP) did not match Git's long
  spelling; fixed in `sameFile` and a new `relWithin` (`4afaf33`), verified by running the shell
  suite with TEMP set to a short name. Split editor groups (`ce4429c`) after the owner chose one
  file per group (D20). Not done by hand: the diff and tab drag in a real window.

- **2026-11-27, session 8 (identity prompt).** The owner's identity prompt (3.1: Obsidian-style
  workbench, graph, command-block terminal, Liquid Glass) was read against the code first; six
  disagreements were reported (Ctrl+, against K2, settings that no longer exist, cmd.exe hiding
  command blocks, feature switches before their features, a docs folder the packages lack, a 2.x
  `antislop.md`) and the owner took the recommendations. D21–D25 recorded with those answers,
  `antislop.md` rewritten, the five reference screens saved (`953bcdc`). Phase 1, the component
  kit: Button, Switch, SettingRow, SettingGroup, SearchField, NavList, Dropdown and Slider in
  `internal/ui/kit`, every shell button moved onto `kit.Button`. Found on the way: two contrast
  failures (a pressed CTA with Accent.Active and a hovered Danger fill, both 4.07:1, fixed in the
  recipes), wrapped setting descriptions clipped because Fyne's RichText caches its size until
  refreshed (fixed, with one deferred relayout), and the About dialog still claiming an embedded
  Neovim (`1217f87`). Rust tests now skip, with the reason, where rustup has no toolchain
  (`1271360`). Gate 1: vet, staticcheck, forbidcheck, `go test -race ./...` on Windows 11 and
  Ubuntu under WSL, and the `kit-gallery` renders in all four themes and both accents, looked at.

## Next action

1. Close #80 on GitHub (fixed by D17; CI green on `ff654da`): left to the owner.
2. The owner: click a file in the Explorer and the title bar's Close by hand, with the native
   editor; and a check by hand on Linux.
3. Identity prompt Phase 2: Settings as a modal (D22) with Terminal > Shell (D24).
4. Editor: an outline sidebar of the file's symbols (they are in the palette today).
5. Project: bookmarks, Markdown preview, a project graph from real imports.
