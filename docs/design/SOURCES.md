# Design Value Sources

Every value PyxForge 3.0 may use, with the file and line it came from. Abbreviations:

- **B** = `target ui reference 2/new refined reference ui/` (canonical concepts)
- **A** = `reference or similar target ui/stitch_pyxforge_doodle_ui_kit/` (first batch)
- **K** = `pyforge_ui_kit/` in the repo (project's earlier kit)
- **L** = 2.x shipped code under `desktop/src/`

The three `DESIGN.md` files in A and B are byte-identical (SHA-256 checked 2026-10-08); only
the `code.html` files differ. Citations use B.

**Status.** The per-theme tables below are *declared* values, read from `DESIGN.md` YAML
frontmatter, `DESIGN.md` prose, or a page's `tailwind.config`/`<style>`. The final section,
"Computed values", records what Chrome actually rendered and measured at 1440, 1280 and 1024 px
(2026-10-08), with raw data in `reference-computed-styles.json` and screenshots in
`reference-screens/`. Where the two disagree, the computed section says so. Do not use the
reference `screen.png` files or the dark concepts' renders for typography: their declared fonts
never load (see "Fonts that actually rendered").

## Conflicts inside the references

Each concept's frontmatter (which its `code.html` uses) disagrees with its own prose.

| Concept | Frontmatter / HTML | Prose | What the page renders (measured) | Proposed rule |
|---|---|---|---|---|
| Smoked Kraft | base `#141312`, accent `#f6bb84`, text `#e6e2df` | base `#1A1918`, sunken `#141312`, amber `#D19A66`, chalk `#E8E6E1` | page `#141312`, panels `#1c1b1a` at 75 %, amber `#f6bb84`; `#1A1918` covers 196 px² | Decide in Phase 1 from side-by-side renders of both ladders; the rendered page has only been seen through 24 px blur |
| Verdigris Forge | background `#091612`, primary-container `#142822` | canvas `#142822`, deep `#0d1a16`, raised `#1a332c` | page `#091612`, panels `#111e1a` at 90–95 %, wells `#05110d`; `#142822` not used as a surface | Same: Phase 1 renders both |
| Ink & Glass | surface `#f9f9f9`, ink `#1a1c1c` | paper `#FAF9F6`, ink `#000000` | no Ink & Glass page exists | Light (Q1). Phase 1 picks between the two papers and inks from renders |
| Radii, Smoked Kraft | HTML config: DEFAULT 2 px, lg 4 px, xl 8 px, full 12 px | frontmatter: sm 2, DEFAULT 4, md 6, lg 8, xl 12 px | 2 px on 55 elements, 12 px on 12 panels, 4 px on 9 | Section 11.4 values (4 controls, 6 panels, 8 overlays); the page's 2 px is below a usable focus-ring radius |

## Smoked Kraft (dark, primary)

Source file: `B/smoked_kraft_charcoal_ide/DESIGN.md` (SK) and `B/main_workspace_charcoal/code.html`.

| Role (3.0) | Value | Source |
|---|---|---|
| surface.base (canvas) | `#1A1918` | SK:173 |
| surface.sunken (wells, status bar, terminal) | `#141312` | SK:174, SK:4 |
| surface.lowest | `#0f0e0d` | SK:7 |
| surface ladder | `#1c1b1a`, `#201f1e`, `#2b2a29`, `#363433` | SK:8–11 |
| surface.bright | `#3a3938` | SK:6 |
| overlay (L2: palette, modal) fill | `rgba(28,27,26,0.94)` | SK:217 |
| overlay rim | `rgba(232,230,225,0.18)` 1 px | SK:217 |
| overlay shadow | `0 16px 36px -8px rgba(0,0,0,0.65)` | SK:217 |
| tooltip / context menu (L3) | `#1E1D1B`, rim `rgba(232,230,225,0.24)`, shadow `0 4px 12px rgba(0,0,0,0.5)` | SK:218 |
| specular top edge | `rgba(232,230,225,0.14)`–`0.16` 1 px | SK:176, SK:216 |
| text.primary (chalk) | `#E8E6E1` (prose) / `#e6e2df` (frontmatter) | SK:180, SK:12 |
| text.secondary (graphite) | `#B5B0A8` / `#ccc5bd` | SK:181, SK:13 |
| text.tertiary / outline | `#969088` | SK:16 |
| placeholder | `#54514C` | SK:239 |
| border.hairline | `rgba(232,230,225,0.08)` | SK:216, SK:254 |
| border.strong / splitter wash | `rgba(232,230,225,0.12)` | SK:182 |
| outline-variant | `#4a4640` | SK:17 |
| amber (native accent) | `#f6bb84` (fm) / `#D19A66` (prose); container `#2a1400` | SK:28, SK:185, SK:30 |
| error | `#C75A4A` (prose) / `#ffb4ab` (fm) | SK:186, SK:32 |
| success | `#8A9A7B` | SK:187 |
| primary button | fill `#E8E6E1`, text `#1A1918`, hover `#F5F3EF` | SK:234 |
| secondary button | fill `rgba(232,230,225,0.06)`, border `0.12`, hover fill `0.10` border `0.22` | SK:235 |
| ghost button hover | `rgba(232,230,225,0.05)` | SK:236 |
| focus | solid 1 px `#E8E6E1`, no glow | SK:239 |
| selected row | `rgba(232,230,225,0.08)` + 2 px left marker | SK:240, SK:251 |
| hover row | `rgba(232,230,225,0.04)` | SK:251 |
| modified dot | 5 px `#D19A66` | SK:244 |
| switch transition | 120 ms linear | SK:248 |

## Verdigris Forge (dark, tinted)

Source file: `B/verdigris_forge/DESIGN.md` (VF) and `B/main_workspace_verdigris/code.html`.

| Role (3.0) | Value | Source |
|---|---|---|
| surface.base | `#142822` | VF:157 |
| surface.sunken | `#0d1a16` (prose) / `#091612` (fm, HTML background) | VF:157, VF:4 |
| surface.raised | `#1a332c` | VF:157 |
| surface ladder (fm) | `#05110d`, `#111e1a`, `#15221e`, `#1f2d28`, `#2a3833` | VF:7–11 |
| verdigris midtone (focus, inactive chrome, dividers) | `#2d5a4c` | VF:158 |
| cream (text, primary button) | `#f4ede2` | VF:159 |
| text (fm) | `#d7e6df`, variant `#c2c8c4`, outline `#8c928f` | VF:12–16 |
| glass fill | `rgba(20,40,34,0.65)` + 16 px blur (D3: replace blur with near-opaque tint) | VF:161 |
| specular edge | `rgba(244,237,226,0.18)` | VF:162 |
| alert | `#d48243` | VF:163 |
| success | `#72b896` | VF:164 |
| ghost text | `#9cb8ad` | VF:200 |
| placeholder | `#4e786b` | VF:203 |
| status chip | 22 px high, fill `rgba(45,90,76,0.35)` | VF:207 |
| tool rail | 48 px | VF:181 |

## Ink & Glass (light; Q1 answered 2026-10-08)

Source file: `B/ink_glass/DESIGN.md` (IG).

| Role | Value | Source |
|---|---|---|
| paper | `#FAF9F6` | IG:107 |
| ink | `#000000` | IG:108 |
| glass | `#FFFFFF80` + 20–32 px blur, 1 px white top edge | IG:109 |
| active | `#1A1A1A` | IG:110 |
| surface ladder (fm) | `#f9f9f9`, `#ffffff`, `#f3f3f4`, `#eeeeee`, `#e8e8e8`, `#e2e2e2`, dim `#dadada` | IG:4–11 |
| text (fm) | `#1a1c1c`, variant `#4c4546`, outline `#7e7576`, outline-variant `#cfc4c5` | IG:12–17 |
| glass shadow | 10 % black, large radius | IG:138 |
| radii | panels 16 px, controls 8 px | IG:146–147 |
| spacing | unit 4, gutter 24, margin 32, panel padding 16 | IG:86–89 |

## Ink & Paper (light, primary)

Sources: `K/ink-and-paper/main-workspace/code.html` (`tailwind.config`, `<style>`), and the 2.x
shipped theme `L/themes/ink-and-paper.css`.

| Role | Value | Source |
|---|---|---|
| paper background | `#FAF9F6` | K tailwind `background`; L `--bg-void` |
| inset | `#F3F3F4` | L `--bg-inset`; K `surface-container-low` |
| surface ladder | `#ffffff`, `#f9f9f9`, `#eeeeee`, `#e8e8e8`, `#e2e2e2` | K tailwind |
| text | `#1A1C1C`, `#4C4546`, `#7E7576` | L `--text-*`; K tailwind |
| border | `rgba(0,0,0,0.12)`, strong `rgba(0,0,0,0.25)` | L `--border-*` |
| ink accent (2.x) | `#1A1A1A`, dim `rgba(0,0,0,0.06)`, border `rgba(0,0,0,0.40)` | L `--accent*` |
| error | `#ba1a1a` | K tailwind |
| removed in 3.0 | dot-grid `radial-gradient` background, `rgba(255,255,255,0.4)` + 20 px blur panels, wiggle hover | K `<style>`; fails `lint-slop.sh` |

## Monochrome (dark proposed, Q4)

No concept file. Two real sources from 2.x:

| Role | `L/themes/mono.css` | 2.x `docs/DESIGN.md` §3 |
|---|---|---|
| surface.base | `#121212` | `#0A0A0A` |
| surface.raised | `#1a1a1a`, `#242424` | `#111111`, `#161616` |
| surface.sunken | `#0a0a0a` | `#050505` |
| border | `#2e2e2e`, strong `#444444` | `rgba(255,255,255,0.08)`, `0.16` |
| text | `#e0e0e0`, `#a0a0a0`, `#707070` | `#EDEDED`, `#8A8F98`, `#5A5F68` |
| status | — | success `#10B981`, error `#EF4444`, warning `#F59E0B` |

## Accents (D2)

| Accent | Source | Status |
|---|---|---|
| Crimson | Owner brief; no reference file contains a crimson | To derive per theme in Phase 1 with contrast calculations (4.5:1 text, 3:1 UI) |
| Amber | Smoked Kraft `#f6bb84` / `#D19A66`; container `#2a1400` | Base values exist; per-theme variants derived in Phase 1 |
| Hazard yellow (candidate, Q3) | Signal Yellow `#f7c902`, dark `#d4ac00`, light `#ffdf43` (`B/main_workspace_signal_yellow/code.html` tailwind) | Not in 3.0 unless the owner says so |

## Typography

| Concept | Display | Body | Code | Script | Source |
|---|---|---|---|---|---|
| Smoked Kraft | Syne | Geist | JetBrains Mono | — (HTML also loads Kalam) | SK:52–135; charcoal `<link>` |
| Verdigris Forge | Syne | Geist | JetBrains Mono | "warm-cream script" for notes | VF:52–123 |
| Ink & Glass / Ink & Paper | Unbounded | JetBrains Mono for UI labels | JetBrains Mono | Yellowtail (wordmark), Caveat (notes) | IG:52–77; K `<link>` |
| Signal Yellow | Space Grotesk | — | JetBrains Mono | Yellowtail, Permanent Marker | signal yellow `tailwind.config` |

Master prompt Section 11.3 already picks Syne / Geist / JetBrains Mono, which matches the two
dark concepts. Declared scale in Smoked Kraft (closest to Section 11.3's 12/13/14/16 UI and
12/13/14 code):

| Style | Family | Size / line height | Weight | Tracking | Source |
|---|---|---|---|---|---|
| label-sm | Geist | 11 px / 1.2 | 500 | +0.06 em | SK:130–135 |
| label-md | Geist | 12 px / 1.2 | 500 | +0.04 em | SK:124–129 |
| body-sm | Geist | 12 px / 1.4 | 400 | +0.01 em | SK:106–111 |
| body-md | Geist | 14 px / 1.5 | 400 | 0 | SK:100–105 |
| title-md | Geist | 16 px / 1.4 | 600 | −0.01 em | SK:88–93 |
| code-sm | JetBrains Mono | 11 px / 1.5 | 400 | 0 | SK:118–123 |
| code-md | JetBrains Mono | 13 px / 1.6 | 400 | 0 | SK:112–117 |
| headline-sm | Syne | 18 px / 1.35 | 500 | 0 | SK:82–87 |

Font licenses (Syne, Geist, JetBrains Mono, Unbounded, Caveat, Kalam, Space Grotesk: SIL OFL;
Yellowtail, Permanent Marker: Apache-2.0) are to be verified from each font's own repository and
recorded in `FONT_LICENSES.md` in Phase 1. Every concept loads its icons from Google's Material
Symbols font at runtime; 3.0 needs a bundled, open-licensed line-icon set (Section 11.4).

## Sizes declared in the references

| Item | Value | Source |
|---|---|---|
| Tool rail | 48 px | VF:181 (matches Section 11.6) |
| Tree row | 24 px | SK:251 |
| Command palette item | 36 px, 4 px radius | SK:240 |
| Status chip | 22 px | VF:207 |
| Checkbox | 14 px, 2 px radius | SK:247 |
| Modified dot | 5 px | SK:244 |
| Splitter | 1 px line, `space-xs` (4 px) grab zone each side | SK:203 |

## Computed values (measured 2026-10-08)

**Method.** The Chrome DevTools MCP is installed but loads only in a new session, so the same
engine was driven directly: Chrome 154.0.8037.98, headless, on this laptop. Screenshots use
Chrome's `--screenshot` on the untouched `code.html`. Computed values come from a scratch copy of
each page carrying a measurement script that waits for `document.fonts.ready` plus 3.5 s for the
Tailwind CDN, then records `getComputedStyle` and bounding boxes; each run's viewport was checked
equal to the target. Network was on only for the concepts' own Tailwind CDN and Google Fonts
(allowed by Section 3.4). Ten screens × three sizes: B charcoal, verdigris, signal yellow;
A charcoal, verdigris, signal yellow; K main-workspace, debug-inspect, qemu-control,
build-diagnostics; at 1440×900, 1280×800, 1024×640. Files: `reference-screens/<id>-<width>.png`,
`reference-computed-styles.json` (key `<id>-<width>`).

### Fonts that actually rendered

| Screen | Families requested (element count) | Web fonts that loaded | Result |
|---|---|---|---|
| B charcoal | JetBrains Mono 113, ui-monospace 76, Geist 18, Syne 7, Kalam 1 | Geist, Kalam, Material Symbols | JetBrains Mono and Syne never load and have no generic fallback: **code and panel titles render in the browser's default serif** |
| B verdigris | JetBrains Mono 153, Geist 22, Syne 4 | Material Symbols only | **Everything but the icons renders in default serif** |
| A charcoal | as B | Geist, Material Symbols | same as B |
| B signal yellow | JetBrains Mono 123, Space Grotesk 23, Yellowtail 1, Permanent Marker 1 | all | renders as designed |
| K main-workspace | JetBrains Mono 62, Unbounded 3, Caveat 2, Yellowtail 1 | all | renders as designed |

Consequence: the Syne / Geist / JetBrains Mono pairing chosen in Section 11.3 has never actually
been seen in any concept. Phase 1 must render it (in Fyne) before it is committed to.

### Layout proportions

| Screen | Width | Header | Left sidebar | Status bar | Context bar | Explorer / editor / inspector | Content padding, gap | Content height vs viewport |
|---|---|---|---|---|---|---|---|---|
| B charcoal | 1440 | 56 | 256 | 28 | 41 | 279 / 570 / 279 | 16, 12 | 927 vs 900 |
| | 1280 | 56 | 256 | 28 | — | 239 / 490 / 239 | 16, 12 | 967 vs 800 |
| | 1024 | 56 | 256 | 28 | 90 (wraps) | 175 / 362 / 175 | 16, 12 | 1045 vs 640 |
| B verdigris | 1440 | 56 | 256 | 28 | 49 | 193 / 684 / 291 | 4, 4 | 900 vs 900 |
| | 1280 | 56 | 256 | 28 | — | 166 / 591 / 251 | 4, 4 | 870 vs 800 |
| | 1024 | 56 | 256 | 28 | 87 (wraps) | 123 / 442 / 187 | 4, 4 | 942 vs 640 |
| A charcoal | 1440 / 1280 / 1024 | 56 | 256 | 28 | — | same grid as B | 16, 12 | 902 / 906 / 998 |
| K main-workspace | 1440 | 64 | 256 | 32 | — | editor card 1120 wide, no inspector | 32, 24 | fits |
| | 1024 | 64 | 256 | 32 | — | editor card 704 wide | 32, 24 | fits |
| K debug-inspect | 1024 | 64 | 256 | 32 | — | 427 / 395 | 32 | 888 vs 640 |

Findings:

- No concept uses the 48 px icon rail its own `DESIGN.md` (VF:181) and Section 11.6 call for.
  All use a 256 px labeled sidebar that never collapses.
- None adapts below 1440: columns shrink proportionally, labels and buttons wrap (screenshot
  `B-charcoal-1024.png`: nav clipped at "Extensio…", "Ping Core / Backend" on two lines, register
  names touching their values), and the dark concepts overflow vertically from 1280 down.
  Section 11.6's collapse rules have no reference to copy; they are new design work.
- Useful proportions at 1440: charcoal's explorer : editor : inspector is about 1 : 2 : 1;
  verdigris gives the editor 58 % of the content width with a 291 px inspector, which reads better.

### Colors as rendered (dominant painted areas, text and edges)

| Screen | Page | Panels / wells | Text (most used first) | Edges |
|---|---|---|---|---|
| B charcoal | `#141312` | panels `#1c1b1a` at 75 % over 24 px blur; header `#0f0e0d` at 90 %; sidebar `#0f0e0d` at 80 %; status `#0f0e0d`; editor well `#141312` at 95 % | `#ccc5bd` 67, `#c8c6c2` 58, amber `#f6bb84` 41, `#e6e2df` 37, `#83827e` 31 | 1 px `#4a4640` at 30–60 % on 130 edges; amber at 40–50 % on 16 |
| A charcoal | `#141312` | panels solid `#1c1b1a`, `#201f1e`; chips `#363433` | same as B | **none** (no borders at all) |
| B verdigris | `#091612` | panels `#111e1a` at 90–95 %; wells `#05110d` | `#8c928f` 85, `#e8e2d7` 29, `#ccc6bb` 22, `#c2c8c4` 19, `#b5ccc2` 17, `#a1d1bf` 15 | almost no borders; 1 px rings drawn as box-shadow: `#424845` at 10–30 %, `#a1d1bf` at 20–30 % |
| K main-workspace | `#FAF9F6` | white at 30–50 % over 16–24 px blur | `#1a1c1c` 24, `#4c4546` 22, `#000000` 21, `#4c4546` at 40 % 9 | 1 px black at 10 %, `#7e7576` at 10–30 % |
| B signal yellow | `#f7c902` | `#fff8e0` at 82 % over 12 px blur | `#111111` 71, hazard `#f7c902` 28, `#111111` at 40–70 % 37 | 2 px `#111111` on 67 edges, 1 px on 40 |

### Type in use

| Screen | Dominant UI text | Code | Labels | Smallest functional text |
|---|---|---|---|---|
| B charcoal | mono 11 / 16.5, weight 400 | mono 11 / 16.5, keywords 700 | Geist 12 / 14.4, 500, +0.48 px | 10 px ("4 nodes", "PID") |
| B verdigris | mono 11 / 16 | mono 13 / 21.1, keywords 600 | Geist 11 / 16, 600, uppercase, +0.55 px | 10 px (register names) |
| K main-workspace | mono 16 / 24 | mono 16 / 26, keywords 700 | mono 10 / 15, 700, uppercase, +1 px | 10 px |
| B signal yellow | mono 11–12 | mono 12 / 19.5, keywords 800 | Space Grotesk | 9–10 px |

The concepts run denser than Section 11.3 (UI 12–16, code 12–14). Adopt Section 11.3, with the
11 px floor from the detector for any functional text.

### Radii and effects

| Screen | Radii (count) | Shadows | Backdrop blur | Gradients | Animation | Opacity < 1 |
|---|---|---|---|---|---|---|
| B charcoal | 2 px (55), 12 px (12), 4 px (9) | `0 8px 32px rgba(0,0,0,.4)` ×9; amber glow `0 0 6–8px #f6bb84` ×4 | 24 px ×12 | 9 hairline "rules" (transparent → chalk 25–35 % → transparent) | pulse 2 s ×2 | 26 |
| A charcoal | 2 px (51), 12 px (11), 4 px (8) | Tailwind elevation only | 12–24 px ×10 | none | pulse ×2 | 25 |
| B verdigris | 4 px (55), pill (17), 8 px (5) | 1 px rings; sage glow `0 0 8px` ×2 | 24 px ×6, 12 px ×1 | 7 (rules, one panel wash) | pulse ×3, ping ×1 | 6 |
| K main-workspace | pill (7), 12 px (2), 8 px (2) | Tailwind elevation | 16–24 px ×4 | none | pulse ×1 | 16 |
| B signal yellow | 4 px (49), pill (9) | hard offset `2–4px 2–4px 0 #111` ×13 | 12 px ×6 | none | pulse ×4, ping ×1 | 6 |

## Interaction states (Chrome DevTools MCP, 2026-10-08)

Read in this session with the Chrome DevTools MCP: every `:hover`, `:focus`, `:focus-visible`
and `:active` rule in the stylesheet each page's Tailwind CDN compiles, with resolved values and
how many elements use each class, then one live hover to confirm (B charcoal, sidebar "Search"
link: background `rgba(0,0,0,0)` → `#2b2a29`, text `#ccc5bd` → `#e6e2df`, `transition-colors`
150 ms; matches the extracted rule exactly).

| Screen | Hover | Press | Focus | Decorative motion |
|---|---|---|---|---|
| B charcoal (Smoked Kraft) | rows and nav: background steps to `#2b2a29` (12 elements), text to `#e6e2df` (18); primary button `#e4e2dd` (3); panels `#201f1e` at 60–70 % (7) with border `#4a4640` at 40 % (8); one destructive item turns `#ffb4ab` | `scale(0.95)` (6) | **no `:focus` or `:focus-visible` rule at all** | `group-hover:scale-110` (2) |
| B verdigris | background steps to `#15221e` (16) or `#15221e` at 30 % (18) or `#1f2d28` (7); text to `#d7e6df` (10) or `#e8e2d7` (9); 1 px sage ring at 40 % (1) | — | **none** | `hover:scale-110` (4), `group-hover:rotate-12` (1) |
| K ink-and-paper main workspace | `#e2e2e2` at 20 % (5) or `#e8e8e8` at 50 % (4); text to `#000000` (10); `opacity` to 1 (8) | `scale(0.95)` (2) | only the Tailwind forms plugin default: 1 px ring `#2563eb` on inputs (a framework default, not a design decision) | `wiggle` keyframes on hover; `translate-y-0` (1) |

Consequences for 3.0:

- **Hover strength disagrees.** SK:251 specifies a 4 % chalk wash (what `State.Hover` implements:
  about `#232220` on Smoked Kraft Base). The rendered page steps to an opaque `#2b2a29`, which is
  clearly more visible. To decide in the Phase 1 review from the native app, not from either
  source alone.
- **Keyboard focus is new design work.** None of the concepts designs a focus state. PyxForge's
  2 px `Accent.Focus` ring (DESIGN_SYSTEM.md §12) has no reference to copy.
- **Removed:** press-scale, hover scale, rotate and wiggle (Section 11.8; DESIGN_SYSTEM.md §13).
