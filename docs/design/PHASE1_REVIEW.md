# Phase 1 review

Phase 1 (design system and native shell) was reviewed on 2026-10-08. This file records what was
inspected, what the design critique found, what changed in response, and the gaps that remain.
Decisions referenced as R1, K1, G1 and A1 are in `docs/architecture/DECISIONS.md`.

## How it was reviewed

| Method | What |
|---|---|
| Review renders | `go run ./tools/snapshot -out <dir>`: the real shell drawn by Fyne's software renderer. 29 scenarios: every theme × accent at 1440, Smoked Kraft at 1280 and 1024, Ink & Paper at 1024, the floating inspector, the empty state, the palette and Settings for three themes, the glass setting on three themes, keyboard focus in Settings, the explorer and the Log list, notifications, About |
| Native window | `go build ./cmd/pyxforge` and a capture of the real GL window at 150 % scaling (the renders and the GL window agree; System resolves to Smoked Kraft from the OS dark setting) |
| Design critique | `/impeccable critique` with two independent assessments: a heuristic design review of the renders, and a deterministic pass (detector, tests, contrast, greps). Snapshot: `.impeccable/critique/` (git-ignored) |
| Platform guidance | Apple HIG MCP (accessibility, keyboard, menus, dialogs, toolbars) and shadcn/ui MCP anatomy (tabs, dialog, sonner, command, button) |
| Interaction states | Chrome DevTools MCP on the reference concepts: `SOURCES.md` § Interaction states |

The Impeccable detector returns nothing for this codebase, and that result is vacuous: it scans
the Go files but its rules match web syntax only (shown with control probes). The project's own
checks stand in for it: `TestContrast`, `TestPalettesAreDistinct`, `TestFocusIsNotSelection`,
`TestGlassSetting`, the shell's keyboard tests, staticcheck and the forbidden-technology check.

## Render review 1 (before the critique)

Fixed in the shell commits: tree separators removed; Fyne's `→` expanders replaced by Lucide
chevrons; placeholder text wrapped in a 360 px column instead of overflowing; a real reload icon;
palette rows no longer overlap or clip; the `⌘` glyph removed from command rows; the settings
gear drawn correctly (Lucide arcs rewritten explicitly for oksvg); tab labels no longer accented
everywhere; the editor empty state no longer hidden under the tab surface; the floating inspector
collapsed by default below 1280 px and restored when the window widens.

## Critique

Design health 22/40 (Acceptable). Nielsen scores: status 2, real-world match 3, control 2,
consistency 2, error prevention 3, recognition 2, efficiency 2, minimalism 3, recovery 2,
help 1. Verdict: a category-typical IDE skeleton carrying product-specific labels and voice; the
identity lives in the design system but did not yet reach the screen.

| Priority | Finding | Owner answer | Status |
|---|---|---|---|
| P0 | Keyboard operation and focus visibility fail outside the rail and palette | R1: fix | **Fixed** (`cba7ce4`): cards and the status-bar action focusable with Space/Enter; focus ring distinct from selection; ring around the explorer and Log list; region, panel-tab, inspector-tab and next/previous-tab commands; Fyne focus colour is a neutral wash |
| P1 | VS Code keymap collides with Neovim; Ctrl+Alt is AltGr | K1: Neovim owns keys | **Fixed** (`136e56c`): Ctrl+Shift chords only, enforced by a test; `docs/architecture/KEYMAP.md` |
| P1 | Accent spent on rail ticks; five themes read as three; glass ghosts text | A1, G1 | **Fixed** (`e6e7a3c`, `cba7ce4`): accent on the focused region's active tab only, neutral rail markers; glass is a setting (opaque by default, blurred when on); Ink & Glass, Smoked Kraft and Monochrome given distinct surfaces, enforced by ΔE |
| P1 | Placeholders read as idle states; Log hidden as the sixth tab; no tool probes | R1: record | Open, below |
| P2 | Native chrome and Settings look like Fyne defaults | R1: record | Open, below |

Also fixed during the pass: Settings' appearance hooks are rebuilt with the view (`08f1bb7`).

## Known gaps

Each item says where it belongs. None of them is hidden by the UI: the placeholders say what is
missing and when it arrives.

### Copy and content (critique P1, deferred by R1)

- Placeholder titles describe an idle state ("No terminal sessions") instead of leading with
  availability ("Terminal arrives in Phase 4"). The second line could carry detected facts.
- The dock opens on Terminal, a placeholder; Log, the only live tab, is sixth.
- No tool probes beyond Neovim: `qemu-system-*`, `gdb`/`gdb-multiarch`, `nasm` and whether
  `pyxforge.toml` parses are not reported. Phase 4 owns the probes; the status bar could show
  them earlier.
- Error toasts show the raw OS string, have no action and expire (the Log keeps them).
- "Phase 3/4/5" is roadmap language in user-facing copy.

### Native chrome and Settings (critique P2, deferred by R1)

- The title bar takes the OS accent colour, the window icon is Fyne's default, and Fyne adds a
  4 px window padding. The brand mark (owner's Gemini concept) is not yet an icon.
- The About dialog shows two titles ("About" and "PyxForge") and no copyable diagnostics.
- Tree rows are 21 px, not the 24 px row token.
- No tooltips: Fyne 2.8 has no tooltip widget; rail items are named only in the palette.
- Settings: the type specimen is not a setting, the card grid leaves a dead column at some widths,
  the accent dots are unlabelled (dark set, light set), and the page is squeezed above the open dock.
- Panels are not resizable, although `DESIGN_SYSTEM.md` §8 lists the explorer and dock as
  resizable. The split handle (6 px, `SplitDragWidth`) is specified but not built.

### Keyboard and accessibility

- Fyne exposes no accessibility tree, so screen readers (NVDA, Narrator) cannot read the UI.
  PyxForge must not claim assistive-technology support until this is verified on each platform.
- No in-app text or UI scale; only the `FYNE_SCALE` environment variable.
- Fyne tabs are not focusable: tabs are reached by commands (`components/tabs.md`).
- In the command palette, typing a query pre-selects the first match, so "theme" then Enter
  applies "Theme: System". Matched characters are not highlighted; there are no groups.
- Quick open skips dot-directories (`.github`) that the explorer shows.
- Clicking a tree row focuses the tree (Fyne behaviour), so the explorer's focus ring also shows
  after a mouse click. This is kept: the tree is then the keyboard target.
- Toasts do not pause on hover (Sonner does) and are not focusable.

### Open design questions for the owner

- Hover strength: the charcoal reference hovers rows to `#2b2a29` (about 8 %), Smoked Kraft's
  spec says a 4 % wash (`SOURCES.md` § Interaction states). PyxForge uses 4 %.
- Geist is on Impeccable's list of overused UI faces. It is the owner's choice; the identity
  question is whether Syne should carry more than 12 px panel titles.
- The machine-state strip (arch, CPU mode, CS:IP, QEMU and GDB state) proposed by the critique as
  PyxForge's first authored element belongs to Phase 5 but shapes the status bar now.

### Tooling

- Impeccable resolves the project's design authority to the obsolete `docs/DESIGN.md` and finds
  no root `PRODUCT.md`. The critique was run against `docs/design/DESIGN_SYSTEM.md` and
  `docs/design/PRODUCT_CONTEXT.md` by passing them explicitly. `/impeccable doctor` can repoint it
  if the owner wants that.
- QEMU and `gdb-multiarch` are not installed on the development machine (QEMU's installer needs
  an administrator approval the owner must give).
