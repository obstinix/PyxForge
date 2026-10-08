# Reference UI Analysis

Phase 0 analysis of the target UI concepts, 2026-10-08. Values and their sources are in
`SOURCES.md`; the owner's theme requirements are in `THEME_SYSTEM_REQUIREMENTS.md`. The
reference folders are specifications only: nothing from them is imported or shipped.

Status: complete for Phase 0. Static reading, the Impeccable detector (static and rendered),
captures of ten screens at 1440, 1280 and 1024 px (`reference-screens/`), and computed styles and
layout measurements (`SOURCES.md` → "Computed values", raw data in
`reference-computed-styles.json`). The captures used headless Chrome 154 directly because the
Chrome DevTools MCP loads only in a new session; the MCP remains the tool for the Phase 2
side-by-side comparisons.

## 1. Inventory, corrected

| Folder | Content | Finding |
|---|---|---|
| A, first batch | `ink_glass`, `smoked_kraft_charcoal_ide`, `verdigris_forge` (`DESIGN.md`); `main_workspace_{charcoal,verdigris,signal_yellow}` (`code.html`, `screen.png`) | The three `DESIGN.md` files are **byte-identical** to B's. Section 6.1 of the master prompt calls A's Ink & Glass "older"; it is the same file. |
| B, refined (canonical) | same six | Only the three `code.html` files changed. |
| K, `pyforge_ui_kit/` in the repo | `ink-and-paper/{main-workspace, build-diagnostics, debug-inspect, new-project-onboarding, qemu-control, theme-gallery, logo}`; `_legacy/` (seven screens, `blueprint_doodle/DESIGN.md`); `_unidentified/` | `_unidentified/ink_glass/DESIGN.md` is B's Ink & Glass with CRLF line endings (no other difference). `_unidentified/pyxforge_systems_architect_flow/code.html` is byte-identical to `_legacy/new_project_onboarding/code.html` (SHA-256 checked). |

What the "charcoal" workspace is: its `tailwind.config` is the Smoked Kraft palette (`#141312`,
amber `#f6bb84`). There is no separate "charcoal" design system; **charcoal = Smoked Kraft**.

What Signal Yellow is: a **light** neo-brutalist concept (body background hazard yellow
`#f7c902`, ink `#111111` text, 2 px ink borders with hard 3–4 px offset shadows, real
`backdrop-filter: blur(12px)`), not "near-black with yellow" as Section 6.3 describes.

### Refinement made two concepts worse

Counted from `code.html` class usage:

| Concept | Gradients | Blur classes | Arbitrary shadows | Accent glow shadows |
|---|---|---|---|---|
| Charcoal A | 0 | 11 | 1 | 0 |
| Charcoal B | **11** | 14 | 21 | **yes** (`shadow-[0_0_6px_#f6bb84]`, amber `drop-shadow`) |
| Verdigris A | 0 | 2 | 4 | 0 |
| Verdigris B | **7** | 7 | 6 | **yes** (`shadow-[0_0_8px_rgba(161,209,191,0.6)]`) |

Rule for 3.0: take B's **content and layout** (named terminal tabs, live disassembly, inline AI
suggestion, sector footprint) and A's **restraint** in effects.

### `_unidentified/` ownership

| Asset | What it is | Usable? |
|---|---|---|
| `gemini_generated_image_wrukvqwrukvqwruk_1.png` | PyxForge logo concept: constellation in a hand-drawn ring around a `{ ☼ }` bracket-sun mark, script "PyxForge" wordmark, tagline "CODE / NAVIGATE / EXECUTE" | **The owner's** (confirmed 2026-10-08); the brand-mark candidate. It is the origin of the concepts' "Sector Astrolabe" and the `{ ⊙ }` status glyph. |
| `1784228149516_image.png` | Third-party typographic poster ("La Pura", "Kuta Bali", signed by a type foundry) | No |
| `1784228164572_image.png` | Third-party cat illustration carrying another studio's logo | No |
| `1784228246648_image.png` | Third-party editorial layout with photographs of people | No |
| `4a313505….webp`, `90d9649a….webp` | Stock "programmer" doodle set and icon pattern | No |
| `ink_glass/DESIGN.md` | Copy of Ink & Glass | Duplicate |
| `pyxforge_systems_architect_flow/code.html` | Copy of `_legacy/new_project_onboarding` | Duplicate |

The five third-party images were committed to a public Apache-2.0 repository. On the owner's
decision (2026-10-08) they are removed from the repository on `v3`; mood-board material stays local.

## 2. Detector evidence

Impeccable 4.1.0, run with `--no-design-system --no-config` so each concept is judged on its own
pixels and markup rather than against its own `DESIGN.md`.

| Run | Targets | Exit | Findings | File |
|---|---|---|---|---|
| Static | A, B, K (all `code.html`) | 2 (findings) | 228 | `reference-slop-findings.json` |
| Rendered, attempt 1 | B, 3 screens as `file://` URLs | 1 (target not scanned: no browser found) | — | — |
| Rendered, attempt 2 (`IMPECCABLE_BROWSER` = Chrome 154, 1440x900) | B, 3 screens | 2 (findings) | 114 | `reference-slop-findings-rendered.json` |

The static scan cannot see Tailwind utilities (the CDN compiles them in the browser), so it
missed every gradient, glow and blur. The rendered scan is the authoritative one for B.

Rendered findings for B:

| Screen | Findings | Main rules |
|---|---|---|
| Charcoal | 39 | low-contrast 12 (binary watermark at 1.0:1, "Ctrl+S" at 1.1:1), thin border + 32 px shadow 9, nested cards 5, amber glow 5, 10 px "PID" label |
| Verdigris | 21 | nested cards 11, low contrast 3 (text over blurred backdrop), pulsing dots 3, sage glow 2 |
| Signal Yellow | 54 | undersized UI text 26 (9–10 px labels), nested cards 11, tiny text 7, low contrast 4 (incl. `📄` emoji file rows) |

Rules recorded as not applying: **tiny-text** asks for 14 px body text; PyxForge is a dense IDE
with a 12–16 px UI scale and 12–14 px code (Section 11.3). The **undersized-ui-text** 11 px floor
does apply and is adopted. **em-dash-overuse** is advisory copy style, not UI.

## 3. Per-concept assessment

### Smoked Kraft (charcoal workspace): basis for the primary dark theme

- **Does well:** the densest and most IDE-like layout; warm neutral ladder with no blue cast;
  register grid with RIP highlighted; RFLAGS as per-bit chips; hex buffer pinned to `0x7DF0–0x7DFF`
  with the `55 AA` bytes called out and "Magic Signature: VALID (0xAA55)"; "Sector Footprint
  482 / 512, 30 bytes left"; PTY tab named after the process with its PID; status bar with engine
  state, encoding, indentation and branch.
- **Needs improvement:** title-bar buttons ("Ping Core Backend", "Initialize Project", "Fetch
  Profiles") are developer plumbing; labels wrap ("Astrometry Sync: 104.2 kHz", "v0.82-asm"); a
  3%-opacity binary watermark fails contrast and carries no information; amber glows.
- **Remove:** "Sector Astrolabe" star chart; "Astrometry Sync … kHz"; "brewing" status;
  "Active Rig PyxCore-Rust 1.82" card; user avatar (PyxForge has no accounts); margin "scribble"
  quote in a script face over the editor.

### Verdigris Forge (crucible workspace): tinted dark theme

- **Does well:** **live disassembly with the PC row highlighted**; terminal tabs `bash#1`,
  `qemu-serial`, `gdb-server`; inline AI card that explains and offers an explicit "Apply";
  registers with decimal and meaning hints ("stack top", "cs:eip map", "drive: 0x80"); MMU state
  ("Real Mode (A20 gate enabled)"); status bar with QEMU PID and state.
- **Needs improvement:** "Apply Optimizatio/n" wraps; the AI card must show a diff before apply;
  macOS traffic-light dots on a Windows/Linux app.
- **Remove:** "temp: 840°C", "64°C", "Cycles: 1,482,900", "ARM64-v8 0.18 ms", "OXIDIZED FORGE
  CRC-32" badge, "Etched Memorandum" quote, "Node Constellations" nav item. None is backed by data.

### Ink & Paper (project kit main workspace): primary light theme

- **Does well:** a light theme that still reads as a serious tool; black ink on warm paper;
  monospace UI labels; AI action kept off the text; strong keyword weight in code.
- **Needs improvement:** editor and terminal as floating cards inside a padded canvas
  (card-in-card, wasted space); "v1.0.4 QEMU READY" in faded script over the tab bar; dot-grid
  background; coffee-cup and hexagon doodles.
- **Remove:** dot grid, doodles, wiggle hover, 20 px glass panels (all fail the repo's own
  `lint-slop.sh`).

### Ink & Glass: second light theme and overlay language (Q1: light)

- **Does well:** the rule that terminal and editor are the only opaque surfaces and glass is for
  floating layers; 1 px specular top edge; charcoal primary buttons.
- **Remove:** glass on every panel, hand-drawn doodle motifs in every card, Caveat tooltips
  rotated 2°, wiggle on every button hover, 32 px "paper" margins around every panel.

### Signal Yellow: not one of the five themes (Q3)

- **Does well:** bold, unmistakable identity; hard-edged borders read clearly; hex buffer with an
  "0xAA55 OK" chip.
- **Problems:** 9–10 px labels; yellow surfaces for hours of work; offset "etched" shadows on
  everything; marker and script faces; "Crew Advisory" quote card; `📄` emoji file icons.
- Kept only as a possible future accent (hazard yellow).

### Blueprint Doodle (`_legacy`): superseded

Cyan `#00D4FF` blueprint with pencil-jitter borders and torn edges. Superseded by D2. Nothing
carried over.

## 4. Classification (Section 6.5)

| Concept | Verdict | Reason (evidence) |
|---|---|---|
| Layout: left rail + explorer, center editor + bottom dock, right inspector, status bar | **KEEP** | All three dark concepts share it; matches Section 11.6. |
| Layout: floating cards in a padded canvas (Ink & Paper, Ink & Glass) | **REMOVE** | nested-cards 67 static + 27 rendered findings; wastes IDE space. |
| Typography: Syne / Geist / JetBrains Mono | **KEEP** | Smoked Kraft and Verdigris both use it; Section 11.3 picks it. |
| Typography: Unbounded, Space Grotesk, Yellowtail, Caveat, Kalam, Permanent Marker | **REMOVE** (one annotation face may survive for debugger notes only) | Six extra faces across concepts; script faces fail contrast in screenshots. |
| Type scale | **IMPROVE** | Adopt the 11 px floor; fix tight leading (1.2 on body) and wide tracking on body text (0.10 em). |
| Spacing: 4 px unit, 8–12 px panel padding, 1 px splitters with grab zone | **KEEP** | SK:203, VF:179; matches Section 11.4. |
| Spacing: 24 px gutters, 32 px outer margins (Ink & Glass) | **REMOVE** | Conflicts with density. |
| Colors: surface ladders per theme | **KEEP** | Real, consistent ladders in SK, VF, K, L. |
| Colors: accent glows, gradients | **REMOVE** | dark-glow 7, gradients 18 in B. |
| Panels: opaque, hairline-bordered, flush | **KEEP** | |
| Panels: glass on docked panels | **REMOVE** | Section 11.5; low-contrast findings over blur. |
| Sidebar / rail: 48 px icon rail with labeled explorer | **KEEP** | VF:181. |
| Tabs: active tab with top specular line, modified dot | **KEEP** | SK:243–244. |
| Tabs: chamfered corners (Verdigris) | **REMOVE** | Decorative; not worth custom shapes in Fyne. |
| Buttons: solid chalk/ink primary, hairline secondary, ghost | **KEEP** | SK:234–236. |
| Buttons: scale-on-press, wiggle, lift, glow on hover | **REMOVE** | Section 11.8. |
| Dialogs: L2 overlay recipe (near-opaque fill, rim, one shadow) | **REIMPLEMENT** | SK:217 without its 24 px blur, per D3. |
| Terminal: named tabs per process with PID | **KEEP** | Verdigris and charcoal both show it. |
| Terminal: as a floating card | **REMOVE** | |
| Editor: line numbers, current line, PC row marker in debug | **REIMPLEMENT** | Drawn from the Neovim grid plus a debug overlay (D4). |
| Status bar: engine, encoding, EOL, indent, branch, QEMU PID/state | **KEEP** | Must be fed by real services. |
| Status bar: cycles, temperature, "Constellation", "ORIGIN_ACTIVE" | **REMOVE** | Fake metrics (Section 22). |
| Icons: Material Symbols from Google Fonts at runtime | **REIMPLEMENT** | Must be bundled, open-licensed line icons (Section 11.4). |
| Icons: emoji (`📄`) | **REMOVE** | Signal Yellow and K. |
| Navigation: title-bar action buttons | **REIMPLEMENT** | As command-palette commands. |
| Navigation: theme pips in the title bar | **REIMPLEMENT** | Settings → Appearance plus "Change Theme" palette command (D2). |
| Command palette | **REIMPLEMENT** | Absent from every concept; SK:240 gives row height and selection style. |
| Inspector: registers, RFLAGS chips, hex buffer, live disassembly, MMU state, stack depth | **KEEP** | Only when backed by GDB/MI and the binary service. |
| Sector footprint (bytes of 512) | **KEEP** | Real build metric (parity N2). |
| Sector Astrolabe | **REMOVE**, replace with a 512-byte sector map | Decorative chart with no data. |
| Inline AI suggestion with Apply | **REIMPLEMENT** | Must show a diff and need an explicit apply (Section 13.8). |
| Animation: pulsing status dots | **REMOVE** | pulsing-dot 16 static findings. |
| Hierarchy: center editor dominant, side panels secondary | **IMPROVE** | Concepts give side panels equal weight with heavy titles; use type and spacing, not cards. |
| Responsive: docks collapse to overlays below 1280 px | **REIMPLEMENT** | Described in SK:205–208 and VF:181–183 but never implemented: measured, no concept collapses anything; the 256 px sidebar stays fixed and the dark concepts overflow vertically at 1280 and 1024 (`SOURCES.md`, "Layout proportions"). New design work under Section 11.6. |
| Sidebar: 256 px labeled nav (what every concept actually renders) | **IMPROVE** | Replace with Section 11.6's 48 px icon rail plus a collapsible explorer; the concepts' own `DESIGN.md` (VF:181) asks for the rail. |
| Type rendering in the dark concepts | **REIMPLEMENT** | Their declared fonts never load (serif fallback for code and titles), so their look has never been seen as designed; Phase 1 renders the Syne / Geist / JetBrains Mono pairing in Fyne before committing. |
| Interaction: hover washes 4–8 % on rows, solid 1 px focus | **KEEP** | SK:239, SK:251. |
| Logo: `{ ☼ }` bracket-sun mark | **KEEP** (one brand motif, Section 6.4) | Owner-confirmed concept; the mark is redrawn as a single-stroke line asset in Phase 1, without the script wordmark or constellation. |

## 5. Mapping to Fyne

| Need | Fyne approach | Risk / spike |
|---|---|---|
| Five themes × two accents + System, live switch | One `fyne.Theme` implementation reading a token set; `app.Settings().SetTheme` on change; System mode from the platform variant (light/dark) | Low |
| Theme tokens beyond Fyne's named colors (surface ladder, glass, syntax) | Custom token struct consumed by PyxForge widgets; Fyne's `ColorName`s mapped from it | Low |
| Three font families (Syne titles, Geist UI, JetBrains Mono code) | Fyne's theme supplies one font per text style (regular, bold, italic, monospace, symbol). A second sans for titles needs a custom text widget or per-text font support in the chosen Fyne version | **Phase 1 spike** |
| Glass overlay (D3) | `canvas.Rectangle` with alpha fill and stroke, a 1 px top line, a modal scrim rectangle | Low |
| Overlay shadow | Fyne draws shadows only for its own pop-ups and menus; a custom soft shadow needs layered rectangles or a pre-rendered nine-patch image | **Phase 1 spike** |
| Neovim grid | Custom widget painting a cell buffer (per grid) on `flush` | Phase 3 spike (D4) |
| Icons | Bundled SVG line icons as `fyne.Resource`, tinted per theme | Low; choose an icon set with a license in Phase 1 |
| 1024 x 640 minimum, docks collapsing | Custom layout with breakpoints | Medium |

## 6. Captures

Ten screens at 1440×900, 1280×800 and 1024×640, named `<folder>-<screen>-<width>.png` in
`reference-screens/`: B charcoal, B verdigris, B signal yellow, A charcoal, A verdigris,
A signal yellow, K main-workspace, K debug-inspect, K qemu-control, K build-diagnostics.
Method and measurements: `SOURCES.md` → "Computed values".

Still to do with the Chrome DevTools MCP in a later session, because they need interaction rather
than a static render: hover, focus and selected states of explorer rows, tabs and buttons. These
feed the component specs in `docs/design/components/` (Phase 1).
