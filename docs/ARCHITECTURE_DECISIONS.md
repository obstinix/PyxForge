# PyxForge architecture decisions

The authoritative record of what PyxForge has decided and why. Newest decisions first; each
says what would make it worth revisiting. Earlier decisions (D1–D15, answered by the owner,
`obstinix`, from 2026-10-08) are in the table further down. Longer records: [ADR 0005](architecture/0005-native-go-fyne.md)
(Go + Fyne), [ADR 0006](architecture/0006-embedded-neovim.md) (embedded Neovim, superseded by D16),
[ADR 0007](architecture/0007-agents.md) (agents); the 2.x records are in [`archive/2.x/adr/`](archive/2.x/adr/).
Where things stand is in [`PROJECT_STATE.md`](PROJECT_STATE.md).

## D29 — Opening a folder that holds everything asks first

- **Date:** 2026-11-28. **Status:** Done (Phase 7, owner review F1).
- **Problem.** Opening a drive's root or the home folder means listing, searching and watching
  every file the user has; the owner saw starts that froze.
- **Options.** (a) Refuse such folders. (b) Open them, with work on workers. (c) Ask first.
- **Selected.** (c), with (b) as well. A drive's root, the file system's root and the home folder
  are asked about from every way in (Open Folder, a recent folder, a drop, the command line);
  Cancel is the default and Enter cancels. Reopen Last Workspace is a setting, off by default.
- **Revisit when** a project legitimately lives at a drive's root.

## D28 — Work off the UI thread, and crashes contained

- **Date:** 2026-11-28. **Status:** Done (Phase 7, owner review F1, F5).
- **Problem.** File walks and drawing on the UI thread froze the window; a panic in any
  goroutine ended the app and left no trace.
- **Selected.** Walks (Go to File, Explorer folders, the graph) and drawing run on workers. An
  Explorer folder that lists within 30 ms shows at once, as before; a slower one shows
  "Loading…". Workers run through `crash.Go`: a panic is logged with its stack to
  `%APPDATA%\PyxForge\logs` (the newest 20 kept) and said in the window, and the app goes on.
  A panic on the UI thread is logged before the app exits. A watchdog logs each time the window
  did not answer for over 200 ms.
- **Trade-offs.** A UI-thread panic still ends the app: Fyne's event loop cannot be resumed.
- **Revisit when** Fyne offers a recoverable event loop.

## D27 — Large files: lines as slices of the file, limits, a paged hex view

- **Date:** 2026-11-28. **Status:** Done (Phase 7, owner review F4).
- **Problem.** A 100 MB log took 3 s on the UI thread and grew the heap by 575 MB; larger files
  ran out of memory.
- **Options.** (a) A rope or piece table over the file on disk. (b) Lines as slices of the bytes
  read, edited lines copied. (c) Refuse large files.
- **Selected.** (b): unedited lines share the file's bytes, so a file costs its size plus a
  slice header a line; columns still count runes. With limits in Settings > Files & Explorer:
  large-file mode above 2 MB (no colours, no language server), read-only above 50 MB, the hex
  view above 512 MB and for binaries. The hex view reads one screen at a time from disk.
- **Trade-offs.** Text between 50 and 512 MB is in memory whole; (a) would page it.
- **Revisit when** files over 512 MB need editing as text.

## D26 — reserved for Glass on by default (Phase 9)

The owner's prompt calls this decision D22; D22–D25 were taken, so it is D26 and is written in
Phase 9, when it is built.

## D25 — Forge Notes: research notes inside the project

- **Date:** 2026-11-27. **Status:** Done (identity prompt Phase 6).
- **Problem.** Systems work is mostly research: datasheets read, registers decoded, why a boot
  hang happened. Today it lives outside the project, in a separate notes app, and nothing links
  it to the code it explains.
- **Options.** (a) Leave notes to other tools. (b) A notes database of PyxForge's own. (c) Plain
  Markdown files in a project folder, with links PyxForge understands.
- **Selected.** (c). Notes are Markdown in `notes/` (set by `[notes] dir` in `pyxforge.toml`,
  created only with the first note), so they travel with the repository and open anywhere.
  Links: `[[path]]`, `[[file.c]]` by unique base name (an ambiguous name is listed, never
  guessed), `[[file.c:42]]`, `[[file.c#symbol]]` (language server first, else a text search
  marked approximate), `[[other-note]]` and `#tags`. A Backlinks panel shows the notes that link
  to the active file; notes are nodes in the graph (D23). Reading a note runs nothing (D14).
- **As built.** `internal/notes` parses and resolves the links and keeps the backlink index;
  every Markdown file in the project counts as a note for backlinks, and a note name resolves
  to the notes folder first. `[[file.c#symbol]]` asks the language server when the file has
  one (off the UI thread, when the link is opened); the index and Backlinks use the text
  search, and say so. A rename on disk is a delete and a create: links to the old name lead
  nowhere until the note is edited, and Backlinks lists them under "Links that lead nowhere".
  The preview is `internal/ui/mdview` (Fyne's RichText; code blocks in the editor's syntax
  colours); it opens in the editor group beside the note and follows its edits. Notes are
  rounded squares in the graph, and `[[file.c]]` links connect notes to code there.
- **Trade-offs.** Link syntax is Obsidian's, not CommonMark's; other viewers show `[[…]]`
  literally.
- **Revisit when** notes need attachments or embeds beyond images.

## D24 — Terminal command blocks through shell integration

- **Date:** 2026-11-27. **Status:** Done (identity prompt Phase 4).
- **Problem.** The Terminal panel is a plain terminal: nothing shows whether a command worked,
  output cannot be navigated by command, and errors in it never reach Problems.
- **Options.** (a) Guess prompts from the screen text. (b) Shell integration: the shell marks
  prompts and commands with OSC 133 (`A` prompt, `B` command, `C` output, `D;exit` end) and the
  folder with OSC 7, as other terminals do.
- **Selected.** (b). `internal/vt` records the marks as history positions; `internal/term` turns
  them into blocks (command, folder, start, end, exit, duration). The scripts are embedded in
  PyxForge and given only to sessions PyxForge starts, through wrapper files: bash `--rcfile`
  (a wrapper that sources `~/.bashrc`), zsh `ZDOTDIR` (a wrapper folder in the cache), fish
  `--init-command`, pwsh/PowerShell `-NoExit -Command`. **The user's own rc files are never
  changed.** Settings > Terminal > Command blocks (on by default) turns it off.
- **Shell choice (owner, 2026-11-27).** cmd.exe has no way to emit the marks, and it is
  `%COMSPEC%`, so on Windows blocks would never show by default. Settings > Terminal > Shell
  lists the shells found on the machine (PowerShell 7 `pwsh`, Windows PowerShell, cmd.exe, Git
  Bash; on Unix `$SHELL` and the others in `/etc/shells`), and new sessions default to `pwsh`,
  else Windows PowerShell, else `%COMSPEC%`; on Unix to `$SHELL` as before. With cmd.exe chosen
  the panel says once "Command blocks need PowerShell, bash, zsh or fish" with a link to the
  setting.
- **Keys.** K2 stands: the terminal keeps every plain-Ctrl key, so jumping between prompts and
  searching output use Ctrl+Shift chords where K2 has room, otherwise the palette. Built as
  Ctrl+Shift+Up/Down (previous and next command) and Ctrl+Shift+F (Find in Terminal Output).
- **As built.** Windows' ConPTY passes OSC 133 and OSC 7 through (checked before building).
  bash marks a command with PS0 on bash 4.4 and later (the DEBUG trap misses a command that
  starts with a subshell) and the DEBUG trap on older bash; zsh with preexec and precmd
  hooks, re-marking PS1 when a prompt theme rebuilds it; fish with its prompt and preexec
  events, wrapping fish_prompt lazily because fish loads it after --init-command; PowerShell
  through its prompt function and PSReadLine's PSConsoleHostReadLine. QEMU's serial port and
  GDB's console are Terminal sessions, read only until Send input.
- **Trade-offs.** Integration runs PyxForge's own script in the user's shell; a shell started
  by hand inside a session (ssh, a nested bash) has no marks until it loads one.
- **Revisit when** a shell people use cannot be integrated through a wrapper.

## D23 — Graph view

- **Date:** 2026-11-27. **Status:** Done (identity prompt Phase 5); symbol nodes and
  linker-script edges are PLANNED.
- **Problem.** The roadmap's "project graph from real imports" is open, and nothing shows how a
  project's files, notes and builds connect.
- **Options.** (a) Export to an external graph tool. (b) One Fyne object per node and edge.
  (c) A model and layout in pure Go, drawn into one raster.
- **Selected.** (c). `internal/graph` (no Fyne): node and edge model, import parsers per language
  (C/C++ `#include "…"` against the file's folder and `pyxforge.toml` include paths, NASM
  `%include`, Go imports within the module, Rust `mod`/`use crate::` best effort, Python
  relative and project imports, JS/TS relative imports, Markdown links and `[[wikilinks]]`),
  metrics (degree, PageRank, components, label-propagation clusters with a fixed seed) and a
  deterministic force layout with a Barnes–Hut tree above 500 nodes. No parser runs project
  code (D14). `internal/ui/graphview` draws it in one `canvas.Raster` with `srwiley/rasterx`
  (already a direct dependency): no canvas object per node.
- **Glow.** Allowed only inside the graph canvas, as data: a halo that grows with a node's
  importance, which the user can switch off. It is never decoration elsewhere (D18 stands for
  the workspace). Colour schemes derive from the theme and accent and are contrast-checked
  against the graph background; no hard-coded neon.
- **As built.** A Go import links to the file that represents its package (doc.go, else the
  first by name), one link per imported package, so a package's dozens of files do not each
  take a link. C and NASM includes also resolve against the -I and -i folders of the build
  profiles in `pyxforge.toml`. Clusters are label propagation in a fixed shuffled order (a
  sweep in index order let one label flood a component). Measured on Windows 11: a layout tick
  at 2,000 nodes and 6,000 links takes 1.2 ms; this repository's graph (about 630 files) builds
  in 70 ms. Groups are numbered by size, so the largest take the scheme's first colours.
- **Trade-offs.** Import parsing is static and best effort (macros, build-generated includes and
  dynamic imports are missed); the graph says what it could not resolve rather than guess.
- **Revisit when** projects of more than about 20,000 files need the graph.

## D22 — Settings as a modal with categories and search

- **Date:** 2026-11-27. **Status:** Accepted; PLANNED (Phase 2 of the identity prompt).
- **Problem.** Settings is one scrolling editor tab; it grows with every feature, and nothing
  can be searched.
- **Options.** (a) Keep the tab and add sections. (b) A separate OS window. (c) An in-window
  modal with a navigation list and search, as Obsidian has.
- **Selected.** (c). An in-window modal (90 % of the window, at most 1100×780) over a scrim: a
  search field and a navigation list on the left (Options: General, Editor, Appearance,
  Terminal, Hotkeys, Files & Explorer, Toolchains, Machine, Source Control, Trusted Folders,
  About; Core features), pages of grouped rows on the right. Esc, × and a click on the scrim
  close it. Search covers titles, descriptions and keywords of every row.
- **Owner's answers (2026-11-27).**
  - The shortcut stays **Ctrl+Shift+,** (K2 gives PyxForge no plain Ctrl+, chord).
  - A Core features page (Graph, Notes, Outline, Bookmarks, Command Blocks) is added in the
    phase that builds its feature, never before it: a switch for a feature that does not
    exist would be a control that does nothing.
  - Rows move only for settings that exist; the editor-engine setting went with D16. Terminal
    gains a real Shell setting (D24).
  - Help opens documentation bundled with the program (INSTALL, KEYMAP, FEATURES,
    ACCESSIBILITY, CHANGELOG), so it works offline; the packages gain those files.
  - Hotkeys is display only until rebinding is designed (PLANNED).
- **Trade-offs.** A modal hides the workspace while open, so a setting's effect on the editor is
  seen after closing it (Appearance keeps live previews of its own).
- **Revisit when** settings need per-project values.

## D21 — Liquid Glass as an opt-in material on floating surfaces (revises D18)

- **Date:** 2026-11-27, contrast rule revised the same day by the owner. **Status:** Done
  (identity prompt Phase 3); the floating-chrome variant is PLANNED.
- **Problem.** The owner wants a distinctive material. D18 removed glass because translucent
  panes under text, with accent glows behind them, looked generic.
- **Options.** (a) Stay flat. (b) Glass everywhere again. (c) Glass only where things float,
  opt-in.
- **Selected.** (c). Appearance > Material: **Flat (default)** or **Liquid Glass**, with a Blur
  strength (Subtle 12, Normal 24, Strong 40) and a live preview. Glass applies only to floating
  surfaces: the command palette, the Settings modal, notifications, tooltips, and the editor's
  completion and hover boxes (the graph controls when the graph exists, Phase 5). It never sits
  under the editor's or the terminal's body text, the sidebars or the status bar. Menus and the
  floating inspector stay flat; the find bar is docked above the editor, not floating, so it
  stays flat too. The recipe (`kit.Float` with `Glass`): Fyne's live `canvas.Blur` (measured at
  0.2–3.2 ms a frame, `docs/development/liquid-glass-spike.md`), a per-theme `GlassTint` (the
  Overlay colour at 62 % on dark palettes, 72 % on Ink & Paper), a refraction band 10 px wide at
  the rim (the backdrop captured when the surface opens, magnified 4 %, under the blur), a 1 px
  specular line (white 35 % top-left to 8 % bottom-right) inside a 1 px dark hairline, the
  existing shadow outside, and a 140 ms open (98 % to 100 %, glass fading in; none under Reduce
  Motion).
- **Unchanged from D18.** No backdrop gradients or glows in the workspace; Flat users see no
  change. **New allowance:** a gradient only in the glass rim.
- **Contrast (revised by the owner).** The first rule, 4.5:1 over an all-white and an all-black
  backdrop, cannot hold at a 62/72 % tint: dark palettes would need 0.64–0.68 for primary text
  and 0.79–0.87 for secondary (the spike's table). The owner chose to keep the tint, measure
  against the workspace actually behind the glass, and keep text readable with plates:
  - text-dense content (palette results, Settings pages, notification bodies, completion and
    hover lists) sits on a **plate**, the opaque Overlay surface (`kit.Plate`), so every text
    role meets 4.5:1 whatever is behind;
  - text on bare glass (the Settings page list and header, notification titles, tooltips) is
    primary only, and meets 4.5:1 against the glass as drawn over a busy editor with a
    selection and an accent button, at the weakest blur, in a narrow and a wide window, in
    every theme and accent (`TestGlassContrast`). Secondary text measured 3.10:1 over the
    accent button, so on glass the Settings page list is all primary.
- **Trade-offs.** On dark palettes, over a dark editor, a 62 % tint and a 24 px blur leave
  little of the backdrop visible: the glass reads as a dark frost with a lit edge, and most of a
  surface is plate. Text on bare glass is safe only as long as what floats over is the
  workspace, whose colours the theme controls (PyxForge shows no images under a floating
  surface). The refraction band shows the backdrop as it was when the surface opened. Glass
  costs GPU time and memory that Flat does not, so Flat stays the default.
- **Revisit when** a floating surface can sit over content the theme does not control (images,
  a web view), or the frame cost cannot be held.

## D20 — Split editor groups: one file in one group

- **Date:** 2026-11-27. **Status:** Done (`ce4429c`).
- **Problem.** The master transformation prompt asks for split editor groups. PyxForge's editor
  (D16) has one `codeedit` widget per document, and a Fyne widget can be in one place only, so
  showing one file in two groups needs several views over one buffer: cursors, selections,
  folds and undo grouping per view, and edits reaching every view.
- **Options.** (a) Several views per buffer, as other editors do. (b) A file in at most one
  group: groups only arrange the open documents side by side.
- **Selected.** (b), the owner's choice. Up to three groups; moving a tab takes it out of its
  group, and splitting a group's only tab is refused with the reason. `s.editors` is the active
  group; workspace state records each file's group.
- **Revisit when** someone needs the same file side by side (for example its top and bottom):
  that is option (a), a change to `codeedit` and the document model.

## D19 — Retire the 2.x stack (`legacy/`)

- **Date:** 2026-11-24. **Status:** Done. The file deletions landed in `acc9dca` (committed by
  mistake with a theme test fix), the CI, ignore-rule and comment changes in `140bf98`. The
  2.x code is in git history up to `fd193a2`.
- **Problem.** D8 kept the Tauri desktop app, the VS Code extension and the Rust core in
  `legacy/` until every parity row was Complete or Dropped. It carried 128 files of the
  technology 3.0 forbids, an npm lock file with a Dependabot alert (minimatch, #29), and a
  three-platform CI job, for code nothing in 3.0 runs.
- **Investigation.** No Go package imports or reads anything under `legacy/`; the Rust core's
  tests were ported one for one as golden cases (`internal/config`, `internal/build`,
  `internal/inspect`, `internal/qemu`), and the extension's diagnostic fixtures likewise. The
  parity rows still open (agents, QEMU status details, presets, the offline test job) are
  3.0 work that needs none of the 2.x code.
- **Selected.** Delete `legacy/`, its CI job and ignore rules; `tools/forbidcheck` no longer
  exempts the path. Parity rows 26 and 27 are Dropped.
- **Revisit when** never: the history keeps the code.

## D18 — Flat visual language: no glass, no glow, no gradients

- **Date:** 2026-11-24. **Status:** Accepted; revised by D21 (glass as an opt-in material on
  floating surfaces only; the workspace stays flat).
- **Problem.** The owner judged the interface generic and "AI-looking": a workspace backdrop
  with two accent radial gradients (`kit.Backdrop`), translucent panes over it
  (`kit.PaneFill`, glass on by default), blurred translucent overlays, and an Ink & Glass
  theme whose only identity was that material.
- **Alternatives.** Keep glass as an opt-in setting (G1); tone the glows down; remove the
  material entirely.
- **Selected.** Remove it. Surfaces are flat and opaque; depth comes from the four surface
  steps and 1 px hairline borders; overlays are the opaque Overlay surface with a strong
  border and no blur; no gradient anywhere in the workspace. The Glass setting and command
  go. Ink & Glass is retired (its distinguishing feature was the glass layer, and Ink & Paper
  already covers the light paper look). Kept: Smoked Kraft, Ink & Paper, Verdigris Forge,
  Monochrome, each with the Crimson or Amber accent, plus System.
- **Trade-offs.** One light theme instead of two; saved settings that name Ink & Glass fall
  back to Ink & Paper.
- **Revisit when** a theme needs a material of its own and can show it without translucency
  over text.

## D17 — Terminal without Neovim

- **Date:** 2026-11-24. **Status:** Accepted and implemented (`d337658`, `fac8593`, and the
  shell's move onto them).
- **Problem.** The Terminal panel was Neovim's `:terminal` (D13). Issue #80: on Windows the
  panel's Neovim crashes (0xC0000005) when a program exits, a Neovim bug (`terminal_close`
  refreshes, a pending resize reaches `pty_proc_resize` after the ConPTY was freed); present
  on Neovim master too. Keeping Neovim for the terminal alone would also keep it as a runtime
  dependency after D16.
- **Prototype of `github.com/fyne-io/terminal`** (BSD-3-Clause, Fyne's own), read and tried
  against the acceptance checks (shells, colours, resize, scrollback, process exit):
  - no scrollback: a line scrolled off the top is dropped (`scrollUp` overwrites row 0);
  - its ANSI colours are fixed package variables, so they cannot follow PyxForge's themes;
  - its Windows start runs `powershell.exe` from a hard-coded path, ignoring the user's
    shell and Run Current File's program, and calls `log.Fatalf` if waiting on the process
    fails, which would end PyxForge;
  - no per-program exit status for a session started with an argument list.
  It fails the scrollback check, so per the decision's fallback the VT layer is PyxForge's.
- **Decision.** Three packages, none of which knows about Fyne except the last:
  - `internal/pty`: ConPTY through `golang.org/x/sys/windows` (no new dependency on
    Windows), `github.com/creack/pty` (MIT) elsewhere. It reports the exit status, and closes
    its side only once output has been quiet for 60 ms (at most 2 s) after the exit, so the
    last line always arrives. `Resize` and `Write` after the end return `ErrClosed`, under the
    same lock that closes the console: #80's window cannot happen here.
  - `internal/vt`: an xterm-compatible screen with 10,000 lines of history (Neovim's default):
    cursor movement, erase, scroll regions, insert/delete, 16/256/24-bit colour, the alternate
    screen, DEC line drawing, bracketed paste, mouse modes, status and cursor reports.
  - `internal/term` joins them (typing at a prompt, Clear); `internal/ui/termview` draws a
    screen in the theme's colours and encodes keys, pastes and the mouse.
- **Consequences.** The Terminal panel no longer starts a Neovim; Esc Esc (leaving terminal
  mode) is gone, the wheel scrolls the history instead. #80 is fixed by construction. The
  user's shell is the same as before: `%COMSPEC%` on Windows, `$SHELL` elsewhere. Not done:
  re-wrapping lines on resize (lines are cut or padded, like xterm without reflow), combining
  characters (dropped), the terminal's own font choice.
- **Revisit when** a program people use needs a sequence `internal/vt` lacks, or reflow on
  resize is asked for.

## D16 — PyxForge's own editor replaces embedded Neovim

- **Date:** 2026-11-24. **Status:** Accepted; steps 1–4 done (2026-11-24), step 5 next.
- **Problem.** The editor is an embedded Neovim (D4, ADR 0006). It depends on a separate
  program and its Lua configuration and plugins, starts a second process per panel, behaves
  like Vim rather than a conventional IDE (key ownership needed K1, then K2, then a toggle),
  and its terminal crashes on Windows (#80). The owner asked for an editing experience
  PyxForge owns, that installs with nothing else.
- **Alternatives investigated (2026-11-24, from the projects' own repositories and licences):**

  | Option | What it would take | Verdict |
  |---|---|---|
  | Keep Neovim (A) | Nothing new; #80 stays until upstream fixes it | Rejected: external runtime, Vim model, upstream crash |
  | Helix (A) | Terminal-only program (MPL-2.0), no embedding API or UI protocol; would run in a terminal pane and lose structured events | Rejected: the same dependency problem, less integration than Neovim |
  | Zed (A) | GPL-3.0 editor; its GPUI framework (Apache-2.0) is Rust and owns the whole window | Rejected: not embeddable in a Fyne window; licence |
  | Lapce (A) | Rust, Floem UI, its core is not published as a library | Rejected: would mean a Rust sidecar and a second UI toolkit |
  | Scintilla (B) | C++ component (permissive licence) drawing into a native Win32 window or a GTK widget | Rejected: Fyne draws everything into one OpenGL canvas; there is no native child widget to host it, so it would need a different hack per platform |
  | Tree-sitter (C, parsing) | Official Go bindings (MIT), cgo, one C grammar per language | Deferred: highlighting comes from Chroma first; Tree-sitter is the upgrade path for folding and structure |
  | **Native Go editor (C)** | Document model, Fyne widget, Chroma highlighting (pure Go, MIT), a Go LSP client | **Selected** |

- **Selected approach.** `internal/textbuf` (text as lines of runes, grouped undo and redo,
  atomic save keeping line endings and BOM, change events), `internal/syntax` (Chroma lexers
  reduced to the theme's eleven syntax roles), `internal/ui/codeedit` (the widget: cursor and
  selection, conventional keys, auto-indent, auto-pairing, bracket matching, folding, find
  and replace, comment toggling, completion list, information box, diagnostics, gutter
  marks), `internal/lsp` (JSON-RPC client: diagnostics, completion, hover, definition,
  references, rename, formatting, code actions; UTF-32 positions when offered, else UTF-16).
  Language servers stay separate programs found on the machine or installed by the
  Dependency Manager; the editor works without them.
- **Evidence (prototype, Windows 11, 2026-11-24).** `internal/textbuf`, `internal/syntax`,
  `internal/ui/codeedit` and `internal/lsp` tests pass; `TestClangd` drives a real clangd
  23.1 through diagnostics, hover, definition, references, rename, completion and formatting
  (positions in UTF-32). `TestLargeFileFrameTime`: a 10,000-line C file opens and draws its
  first frame in about 0.2 s; a scrolling frame averages well under 1 ms (test driver).
  `TestHighlightSpeed`: 5,000 lines coloured in about 0.18 s. The real-window check is
  `go run ./tools/editorspike` (results in PROJECT_STATE.md).
- **Trade-offs.** No Vim emulation (Vim users lose modal editing; an optional Vim keymap is
  possible later inside the widget). Highlighting is lexical (Chroma), not syntactic, until
  Tree-sitter is added. Very large files (over about 50,000 lines) have not been measured.
- **Migration.** (1) the widget in the shell behind the editor setting, Neovim still the
  default; (2) language-server client wired per language, replacing `nvim/lua/pyxforge/lsp.lua`;
  (3) native editor the default; (4) the terminal off Neovim (D17); (5) remove Neovim, `nvim/`,
  `internal/neovim`, `internal/ui/editor` and the bundled Neovim from the installer.
  Step 3 came after a check by hand of a fresh build in the installed layout on Windows 11:
  opening a file, typing, clangd's diagnostic and completion, Problems, F12, saving, and the
  terminal (shell, a compile and run, Ctrl+C, history, exit status). Embedded Neovim stays
  selectable in Settings → Editor until step 5.
- **Revisit when** a measured editing feature cannot be done well in the widget (IME input,
  right-to-left text) and an embeddable component that fits one OpenGL canvas appears.

## Decisions D1–D15

Answered by the owner (`obstinix`) on 2026-10-08 and after. Where a later decision above
replaces one, its row says so.

| ID | Decision | Answer | Notes |
|---|---|---|---|
| D1 | Backend language | **Go for everything.** Port `pyxforge-core` into Go packages. | The 59 Rust unit tests (all passing on 2026-10-08, see `docs/archive/2.x/CURRENT_STATE-2026-10-08.md`) and the extension's 10 diagnostic/preset tests become golden fixtures. Fall back to a Rust sidecar only per module, with a recorded reason. |
| D2 | Visual direction | **Five first-class themes plus an independent accent system.** Smoked Kraft, Ink & Paper, Ink & Glass, Verdigris Forge, Monochrome. Accents: at least Crimson and Amber, selectable independently of theme. Plus a System mode. | Full owner text: `docs/design/THEME_SYSTEM_REQUIREMENTS.md`. Supersedes the recommended default and Section 7's "at most 3 themes". Open questions below. |
| D3 | Glass (replaced by D18) | **Tinted near-opaque overlay surfaces with a 1 px specular edge, no blur, as the baseline.** | Native blur (DWM Mica/acrylic, macOS vibrancy) is an optional later enhancement behind an interface. Glass on overlays only. See Q2 for how this meets Ink & Glass's "subtle blur where supported". |
| D4 | Editor engine (replaced by D16) | **Real Neovim over the UI protocol** (`nvim --embed`, `nvim_ui_attach`, `ext_linegrid`), rendered natively in Fyne, gated by a Phase 3 spike. | Fallback: Neovim in a PTY pane, with the reason recorded in `docs/decisions/`. CodeMirror is retired either way. |
| D5 | Platforms for 3.0 | **Windows and Linux released; macOS compile-checked only.** | WSL2 is the Linux development path. |
| D6 | Repository delivery | **Revised 2026-10-09: `main` is the delivery branch.** Verified increments are committed and pushed to `main` directly, without pull requests. | Originally: work on `v3`, merge at the parity gate. The owner merged Phase 0 into `main` (PR #15) and then chose direct delivery to `main`; `v3` is superseded. `legacy/` still stays until the parity gate (D8). |
| D7 | Commit attribution | **Commits authored only as `obstinix`; no AI co-author trailers.** | Resolved by the owner; not open for change. |
| D8 | Legacy code (closed by D19) | **Move the Tauri desktop and the VS Code extension into `legacy/` until parity.** | The forbidden-technology check excludes `legacy/` by path. `legacy/` is deleted in the commit that closes the parity gate. |
| D9 | JS plugin loader | **Drop the JS loader; Lua plugins hosted by Neovim, deferred until after Phase 5.** | The current loader runs arbitrary files through `new Function("ctx", code)` with no sandbox (`desktop/src/main.ts:504`). |
| D10 | Child processes | **Every process PyxForge starts ends with it**, also after a forced exit: a kill-on-close Job Object for PyxForge on Windows, `Pdeathsig` for QEMU, GDB and build tools on Linux. Processes started in the integrated terminal end with PyxForge too. | Decided 2026-10-10 after a QEMU outlived a killed PyxForge. `internal/proc`, `TestChildrenDieWithAKilledParent`. |
| D11 | Snapshots | **Two kinds, never mixed.** Machine states are QEMU's own (`savevm`), stored in a qcow2 overlay in the cache folder that is rebuilt with the image, at most 10. Diagnostic snapshots are JSON records of a paused machine for comparison, in the config folder, at most 50; they restore nothing. | Decided 2026-10-10. A raw boot image cannot hold QEMU snapshots, and writing into the user's project was ruled out. |
| D12 | Disassembly | **PyxForge decodes x86 itself** (`golang.org/x/arch`), from GDB's memory reads or the image, in the mode chosen (real, protected, long). GDB's own listing is used only for addresses. | GDB decodes 16-bit code as 32-bit against QEMU's x86-64 register layout; `objdump` on PATH is often LLVM's, which cannot read raw images. |
| D13 | Terminal sessions (to be replaced by D17) | **Each session is a `:terminal` buffer in one Neovim dedicated to the panel.** | Reuses Neovim's terminal emulator and ConPTY support and the editor view; sessions keep separate output and processes. |
| D14 | Opening a project | **Opening a project executes nothing from it.** Git runs with `core.fsmonitor=false`, with the content filters named in the repository's own configuration emptied, and without going into submodules; staging, committing, switching and stashing are refused, with the reason, when that configuration names a filter, merge driver or signing program they would run; fetch, pull and push likewise when it names a program Git runs to reach a remote (`core.sshCommand`, `core.askPass`, `core.gitProxy`, a credential helper, `remote.<name>.uploadpack` or `.receivepack`) or allows the `ext::` transport. ssh runs in batch mode unless the user chose an ssh command, so nothing waits on a prompt nobody sees. A repository setting that repeats one from the user's global or system configuration, key and value, is the user's own (`git lfs install --local` writes such copies) and is not refused. Settings from the user's global and the system configuration (Git LFS) apply as usual, and so do the hooks in the repository's `.git/hooks` on commit, switch, pull and push, as with `git`. A hooks folder named by the repository's own `core.hooksPath` (one the user's configuration doesn't repeat) runs only in a trusted folder: it can be a tracked folder named by a `.git/config` that came with an archive, while `.git/hooks` travels with neither a clone nor an archive (#30). **Running a project's own commands needs trust (#31).** Build, Run and Debug in QEMU, and Run and Debug Current File run only on request, with arguments as arrays and folders inside the project, and in a folder the user hasn't trusted they first show the commands they would run and ask: Cancel runs nothing; Trust and run remembers the folder. Trust is kept in the user's configuration folder (`trusted-folders.json`), never in a project, covers the folders inside, and is revoked in Settings > Trusted folders or with Revoke Trust for This Folder; projects PyxForge creates from its own templates are trusted. The command line (`pyxforge build`, `run`) runs what the user typed and doesn't ask. | A repository's own Git configuration could otherwise run a program on `git status`: `core.fsmonitor`, or a clean filter once file times differ from the index, as after unpacking an archive. `TestRepositoryConfigCannotRunCode`, `TestRepositoryFiltersDoNotRun`, `TestSubmoduleFiltersDoNotRun`, `TestRepeatedUserFiltersAreNotRefused`, `TestGitLFSRepositoryStagesAndCommits`, `TestRepositoryNetworkProgramsAreRefused`, `TestRepositoryHooksPathNeedsTrust`, `TestCommitAsksToTrustTheRepositoryHooks`, `TestBuildAsksForTrust`, `TestMachineAndRunCurrentFileAskForTrust`, `TestTrustStore`. |
| D15 | Windows distribution | **An Inno Setup installer and a portable zip, per user by default, with Neovim bundled.** `PyxForge.exe` is the windowed program (no console) and `pyxforge.com` the console one, from the same source. Neovim (Apache 2.0 and Vim license) ships unmodified, pinned by version and SHA-256; Git, QEMU, GDB and compilers are not bundled and are found or chosen in Tools: Doctor. Settings stay in the user profile; uninstalling removes them only on request. Releases are drafted by CI and published by a maintainer. | Decided 2026-10-20. Users should not need Go or a Git clone to run PyxForge; the editor is the one tool every user needs. The rest are large, separately licensed and updated, and many users already have them. `tools/winpkg`, `packaging/windows/pyxforge.iss`. |

## Superseded decisions from 2.x

| 2.x record | Status in 3.0 |
|---|---|
| ADR 0003, Tauri v2 as desktop UI stack | Superseded by Go + Fyne (master prompt Section 2, Section 8). A new ADR must say so before Phase 2. |
| ADR 0004, CodeMirror 6 editor | Superseded by D4 (Neovim). A new ADR must say so before Phase 3. |
| PRD §13 / Checkpoint 1, "Desktop is primary, extension is baseline" | Both frontends move to `legacy/` (D8); the 3.0 Fyne app becomes the only frontend. |
| `docs/archive/2.x/DESIGN.md`, cyan `#00D4FF` single accent | Superseded by D2 (Crimson and Amber accents, five themes). |

## Theme questions raised by the D2 answer

Answered by the owner on 2026-10-08:

| Q | Answer |
|---|---|
| Q1 | **Ink & Glass is a light theme, as in the references** (paper base, white-tinted glass on overlays). This overrides the phrase "dark ink foundation" in the D2 text. The five themes are therefore two light (Ink & Paper, Ink & Glass) and three dark (Smoked Kraft, Verdigris Forge, Monochrome). Ink & Paper is flat and opaque; Ink & Glass is distinguished by its tinted overlay layer. |
| Q2 | Proposed default accepted: tinted near-opaque glass, no blur, on every platform. **Correction (Phase 1, 2026-10-08):** the question said Fyne cannot blur content behind an in-window overlay. That was wrong: Fyne 2.8 has `canvas.Blur`, which its own modal pop-ups use. **Revised by G1 below.** |
| Q3 | Proposed default accepted: no Signal Yellow theme in 3.0; hazard yellow kept as a candidate accent. |
| Q4 | Proposed default accepted: Monochrome is dark. |
| Q5 | Proposed default accepted: OS light → Ink & Paper, OS dark → Smoked Kraft. |
| Q6 | Proposed default accepted: Amber from the Smoked Kraft amber; Crimson new, per-theme, contrast-checked. |
| Assets | The Gemini PyxForge logo concept (`pyforge_ui_kit/_unidentified/gemini_generated_image_…png`) is the owner's and is the brand-mark candidate. The five third-party images in `_unidentified/` are removed from the repository on `v3`. |

## Phase 1 review answers

The owner answered the Phase 1 design critique on 2026-10-08 (`docs/design/PHASE1_REVIEW.md`).

| ID | Question | Answer |
|---|---|---|
| R1 | Which findings to fix before the Phase 1 review | **P0 and the top P1s:** keyboard operation and focus visibility, key ownership versus Neovim, accent placement and theme distinctness. Placeholder copy, tool probes, the default dock tab and native-chrome polish are recorded as known gaps. |
| K2 | Who owns keys while the editor has focus (2026-10-20, replaces K1) | **PyxForge takes the conventional editor keys; Neovim gets the rest.** Ctrl+N, O, S, W, F, H and P (Cmd on macOS) and Ctrl(+Shift)+Tab run PyxForge's commands in the editor; Ctrl+Shift chords everywhere; the terminal keeps every plain-Ctrl key; Alt and Ctrl+Alt (AltGr) are never bound. "Toggle Neovim Keys in the Editor" gives the plain-Ctrl keys back to Neovim. Policy: `docs/architecture/KEYMAP.md`. | The owner's Phase 1 brief: PyxForge is a general-purpose editor first, and K1 left Ctrl+W, Ctrl+O, Ctrl+S to Neovim, so the conventional keys did something unexpected (PXF-DEF-010). `TestKeyboardPolicy`, `TestEditorShortcutRunsOnce`. |
| K1 | Who owns keys while the editor has focus (superseded by K2) | **Neovim.** Every key goes to Neovim while the editor is focused. Shell commands use Ctrl+Shift chords (Cmd+Shift on macOS), which the editor passes back to the shell, and `:Pyx` commands sent from Neovim over RPC (Phase 3). Ctrl+W, Ctrl+B, Ctrl+J and Ctrl+P are no longer shell shortcuts. Policy: `docs/architecture/KEYMAP.md`. |
| G1 | How Ink & Glass becomes distinct, given that text ghosted through 94 % glass with no blur | **Glass becomes an appearance setting any palette can use**, and Ink & Glass gets its own surface signature. With the setting on, overlays are translucent over a `canvas.Blur` backdrop; off (the default), they are opaque. Revises D3's no-blur baseline and Q2. |
| A1 | Where the accent goes | **On the active tab of the focused region only**, as a 2 px underline (Fyne also tints that tab's label). Rail markers for visible panels become neutral, so one accent mark shows at a time. Revises DESIGN_SYSTEM.md §1 and §12. |

The original questions and proposed defaults follow for reference.

**Q1. Is Ink & Glass light or dark?** Every reference source for Ink & Glass is a *light* system:
paper `#FAF9F6` (prose), surface `#f9f9f9` and ink `#000000`/`#1a1c1c` (frontmatter), white glass
`#FFFFFF80` (`ink_glass/DESIGN.md`). The D2 answer describes it as a "dark ink foundation".
Proposed default: build Ink & Glass as a **dark** theme (ink-black surfaces, dark-tinted glass
overlays with a cream/white specular edge), because that is what the owner wrote, and because
Ink & Paper already covers the light paper look. The light reference values still feed Ink & Paper.

**Q2. Blur.** D3 says no blur in the baseline; Ink & Glass asks for "subtle blur where the native
platform supports it". Fyne cannot blur content behind an in-window overlay, so blur is only
possible at the OS window level (Windows 11 Mica/acrylic, macOS vibrancy), which shows the
desktop behind the window, not the workspace behind a palette. Proposed default: Ink & Glass
overlays use the D3 tinted near-opaque recipe on every platform; an optional, off-by-default
"native window material" setting may be added later behind a platform interface. This keeps D3
and the D2 text consistent.

**Q3. Signal Yellow.** It is one of the six reference concepts but not one of the five required
themes. Proposed default: no Signal Yellow theme in 3.0. Its hazard yellow (`#f7c902`) is
recorded in `SOURCES.md` as a candidate third accent for later.

**Q4. Is Monochrome dark, light, or both?** "Near-black / white / gray foundation" fits either.
Proposed default: dark (near-black surfaces), because Ink & Paper is already the light theme.

**Q5. System mode mapping.** Proposed default: OS light maps to Ink & Paper, OS dark maps to
Smoked Kraft. The accent stays whatever the user chose.

**Q6. Native accents.** Smoked Kraft (amber `#f6bb84`) and Verdigris Forge (cream `#f4ede2` on
verdigris `#2d5a4c`) each have an accent of their own in the references. Proposed default: the
Amber accent set is derived from the Smoked Kraft amber; Verdigris's cream/verdigris pair becomes
its surface and text character, not an accent; Crimson is new (no reference contains it) and its
per-theme values are derived and contrast-checked in Phase 1.
