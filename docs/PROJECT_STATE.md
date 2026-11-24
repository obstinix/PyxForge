# PyxForge project state

The authoritative record of where PyxForge 3.0 stands: what is verified, what is built but
not yet proven, what is being worked on and what comes next. Update it at the end of every
working session. The code is the final authority: when this file and the code disagree, fix
this file. Decisions and their reasons are in [`ARCHITECTURE_DECISIONS.md`](ARCHITECTURE_DECISIONS.md);
per-feature evidence by platform is in [`FEATURES.md`](FEATURES.md).

Status labels: **VERIFIED** (a test or a recorded check on the named platform proves it),
**IMPLEMENTED BUT UNVERIFIED**, **IN PROGRESS**, **BLOCKED**, **PLANNED**.

## Snapshot (2026-11-24)

| | |
|---|---|
| Version | 3.0.0 release candidate. Published pre-release: `v3.0.0-rc.2` (2026-10-20). `CHANGELOG.md` has 3.0.0-rc.3, not yet tagged. |
| Branch | `main`, pushed directly (no pull requests). |
| Latest pushed commit | See `git log origin/main`; the session log below names each milestone's commit. |
| Last verified CI | `886a6a3`: every job passes except the #80 terminal test on Windows. Release workflow on `55bcc77` passed, its clean Windows container check included. |
| Phase | 3.0 completion: native editor migration (D16), flat redesign (D18), terminal off Neovim (D17), fresh-machine and packaging checks, repository clean-up. |
| Platforms | Windows 11 x64 and Ubuntu 24.04 x64 released; macOS compile-checked only (21 test failures on its first run, #89). |

## Architecture today

One Go program, `pyxforge` (`cmd/pyxforge`): a Fyne desktop app and a command-line tool.

| Layer | Packages |
|---|---|
| Desktop shell | `internal/ui/shell` (workbench, panels, Settings, dialogs), `internal/ui/{explorer,commandpalette,notifications,kit,theme,icons,brand,splash}` |
| Editor (production) | Embedded Neovim: `internal/neovim` (process, msgpack-RPC, grid), `internal/ui/editor` (grid view), `nvim/` (Lua configuration, LSP and Tree-sitter set-up) |
| Editor (replacement, D16) | `internal/textbuf` (document), `internal/syntax` (Chroma highlighting), `internal/ui/codeedit` (widget), `internal/lsp` (language-server client) |
| Terminal | A second embedded Neovim's `:terminal` (D13; to be replaced, D17) |
| Project and build | `internal/config` (`pyxforge.toml`), `internal/workspace`, `internal/build`, `internal/runfile`, `internal/templates` |
| Run and debug | `internal/qemu`, `internal/gdb`, `internal/inspect`, `internal/snapshot`, `internal/listing` |
| Source control | `internal/git`, `internal/askpass` |
| Platform and setup | `internal/toolchain` (detection, Doctor), `internal/managed` (Dependency Manager), `internal/proc`, `internal/update`, `internal/trash`, `internal/textfile` |
| Packaging | `tools/winpkg` (Inno Setup installer, portable zip, bundled Neovim and VC++ runtime), `tools/linuxpkg` (`.deb`, AppImage), `tools/release` |

## Status by area

### Editor

| Item | Status | Evidence |
|---|---|---|
| Embedded Neovim editor: tabs, save, find and replace, diagnostics, breakpoints, LSP through Neovim | VERIFIED (Windows, Linux) | `FEATURES.md`, `internal/neovim` tests |
| Native editor: document model (undo groups, atomic save keeping CRLF and BOM, refuses binary) | VERIFIED (Windows; CI runs it on Linux) | `internal/textbuf` tests |
| Native editor: Chroma highlighting, 16 file types PyxForge opens | VERIFIED | `TestHighlightC`, `TestLanguagesPyxForgeOpens`, `TestHighlightSpeed` (5,000 lines in about 0.18 s) |
| Native editor widget: typing, auto-indent, auto-pairing, bracket matching, folding, find and replace, comment toggle, selection, clipboard, mouse, completion list, information box, diagnostics | VERIFIED in tests and a real window (Windows) | `internal/ui/codeedit` tests; `go run ./tools/editorspike` |
| LSP client: diagnostics, completion, hover, definition, references, rename, formatting, start-up timeout | VERIFIED against clangd 23.1 (Windows) | `TestClangd`, `TestServerThatNeverAnswers` |
| Native editor wired into the shell behind the editor setting (Settings > Editor, or the Use PyxForge's Editor command; takes effect at the next start): tabs, open, Save, Save As, Save All, Close and Close All with the unsaved-changes question, quit, New File, Find and Replace bar, Go to Line, breakpoints (gutter click or Toggle Breakpoint; they follow inserted lines), the debugger's stopped line, build and language-server diagnostics in the editor and in Problems, Run and Debug Current File, reload when the file changed on disk, text size, status bar | VERIFIED in tests and review renders (Windows); not yet tried in the packaged app | `TestNativeEditorInTheShell`, `TestNativeEditorLanguageServer`, `go run ./tools/snapshot -only editor-native` |
| Language features in the native editor: completion while typing or Ctrl+Space, Go to Definition, Show Hover Information, Rename Symbol, Format Document, Toggle Line Comment (Ctrl+/), Fold (Ctrl+Shift+[) | IMPLEMENTED BUT UNVERIFIED in the shell (the LSP requests are verified against clangd in `TestClangd`); the commands have no keys yet | |
| Language servers per language through `internal/lsp` (`lsp.Servers`, the Go port of `nvim/lua/pyxforge/lsp.lua`: clangd, gopls, rust-analyzer, asm-lsp, lua-language-server, taplo, marksman, bash-language-server, pyright, typescript-language-server) | VERIFIED for clangd; the others are chosen by `TestServerTable` but not run | `TestServerTable` |
| Not yet in the native editor: Neovim's Git diff view, Java's jdtls, the per-project jdtls data folder, multi-cursor, split views, project-wide search results inside the editor | PLANNED | |
| Native editor as default, Neovim removed | PLANNED (D16 steps 3–5) | |

`tools/editorspike` on Windows 11, 2026-11-24: a 10,000-line file opens and draws in 0.23–0.59 s;
paging through it, 271 pages, takes 0.53–0.64 ms per page, key and frame; a key typed 0.14 ms;
undo, save, a second document in its own tab; clangd's diagnostics 0.14–0.16 s after start,
completion, hover, go to definition; clangd ends with the window.

### Terminal

| Item | Status | Evidence |
|---|---|---|
| Sessions, restart, transcripts, Run Current File in the panel | VERIFIED | `TestKeepTranscript`, `TestRunInTerminal`, `TestTerminalLifecycleStress` |
| Neovim crash (0xC0000005) on Windows when a program exits | BLOCKED upstream (#80); narrowed by `settleResize`, fixed by D17 | Crash dumps mapped to `os_conpty_set_size(NULL)` |
| Native terminal (D17) | PLANNED | |

### Interface

| Item | Status | Evidence |
|---|---|---|
| Workbench, command palette, Settings, notifications, keyboard policy, type scale, editor and terminal text sizes | VERIFIED | `TestKeyboardPolicy`, `TestTextSizes`, snapshot renders |
| Flat redesign: no glass, no backdrop gradients, Ink & Glass retired (D18) | VERIFIED (renders on Windows, `TestFlatOverlays`) | `4a68e97` |
| Low-poly / line illustrations for Welcome and empty states | PLANNED | |

### Setup, packaging, release

| Item | Status | Evidence |
|---|---|---|
| Doctor, Dependency Manager (GCC, GDB, NASM, binutils, Git on Windows), first-run prompt | VERIFIED (Windows CI with local test servers) | `TestDoctor`, `TestDependencyManager`, `TestFirstRunSetup` |
| Windows installer and portable zip with Neovim and the VC++ runtime bundled | VERIFIED in a clean Windows container (release workflow, 55bcc77) | `release-windows.yml` |
| `.deb` on a clean Ubuntu 24.04 container: installs, Doctor, the window opens, removes | VERIFIED (CI) | `.github/scripts/clean-linux.sh` |
| AppImage | VERIFIED (CI runs it) | `ci.yml` |
| Code signing | BLOCKED: needs a certificate (owner) (#33) | |
| v3.0.0-rc.3 published | PLANNED: owner's call | |

### Repository

| Item | Status |
|---|---|
| Decision and state records consolidated into this file and `ARCHITECTURE_DECISIONS.md`; 2.x ADRs moved to `docs/archive/2.x/adr/` | VERIFIED (2026-11-24) |
| `legacy/` (2.x Tauri, VS Code extension, Rust core) removed with its CI job (D19) | VERIFIED (2026-11-24) |
| `pyforge_ui_kit/`, stale audit documents | PLANNED: remove with the safe-deletion procedure |
| README rewritten from the implementation, with an architecture diagram | PLANNED |

## Known bugs and failing tests

- #80: Terminal panel Neovim crash on Windows; its CI test fails on Windows.
- #89: 21 tests fail on macOS (compile-checked platform).
- #52: a click right after the window is activated is sometimes ignored on Windows.
- #64: Rust with the MSVC target cannot be debugged with GDB.

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

## Next action

D16 step 3: give the language commands keys (F12, Ctrl+K Ctrl+I or similar, recorded in
`KEYMAP.md`), try the native editor in the packaged app on Windows and Linux, then make it the
default engine. After that D17 (the terminal off Neovim), then step 5 (remove Neovim).
