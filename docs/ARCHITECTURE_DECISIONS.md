# PyxForge architecture decisions

The authoritative record of what PyxForge has decided and why. Newest decisions first; each
says what would make it worth revisiting. Earlier decisions (D1–D15, answered by the owner,
`obstinix`, from 2026-10-08) are in the table further down. Longer records: [ADR 0005](architecture/0005-native-go-fyne.md)
(Go + Fyne), [ADR 0006](architecture/0006-embedded-neovim.md) (embedded Neovim, superseded by D16),
[ADR 0007](architecture/0007-agents.md) (agents); the 2.x records are in [`archive/2.x/adr/`](archive/2.x/adr/).
Where things stand is in [`PROJECT_STATE.md`](PROJECT_STATE.md).

## D18 — Flat visual language: no glass, no glow, no gradients

- **Date:** 2026-11-24. **Status:** Accepted; implementation in progress (see PROJECT_STATE.md).
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

- **Date:** 2026-11-24. **Status:** Planned (decided in direction, not yet prototyped).
- **Problem.** The Terminal panel is Neovim's `:terminal` (D13). Issue #80: on Windows the
  panel's Neovim crashes (0xC0000005) when a program exits, a Neovim bug (`terminal_close`
  refreshes, a pending resize reaches `pty_proc_resize` after the ConPTY was freed); present
  on Neovim master too. Keeping Neovim for the terminal alone would also keep it as a runtime
  dependency after D16.
- **Direction.** A terminal PyxForge owns: a pseudo-terminal (ConPTY on Windows, a Unix pty
  on Linux) and a VT emulator drawn with the same cell renderer as the editor. Candidate to
  prototype first: `github.com/fyne-io/terminal` (BSD-3-Clause, Fyne's own; ConPTY through
  `ActiveState/termtest/conpty`, `creack/pty` elsewhere). Accept it only if a prototype shows
  shells, colours, resize, scrollback and process exit on both platforms; otherwise write the
  VT layer in `internal/` over the same pty libraries.
- **Revisit when** the prototype fails any of those checks.

## D16 — PyxForge's own editor replaces embedded Neovim

- **Date:** 2026-11-24. **Status:** Accepted; prototype verified, migration in progress.
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
| D8 | Legacy code | **Move the Tauri desktop and the VS Code extension into `legacy/` until parity.** | The forbidden-technology check excludes `legacy/` by path. `legacy/` is deleted in the commit that closes the parity gate. |
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
