# Design Value Sources

Every value PyxForge 3.0 may use, with the file and line it came from. Abbreviations:

- **B** = `target ui reference 2/new refined reference ui/` (canonical concepts)
- **A** = `reference or similar target ui/stitch_pyxforge_doodle_ui_kit/` (first batch)
- **K** = `pyforge_ui_kit/` in the repo (project's earlier kit)
- **L** = 2.x shipped code under `desktop/src/`

The three `DESIGN.md` files in A and B are byte-identical (SHA-256 checked 2026-10-08); only
the `code.html` files differ. Citations use B.

**Status: every value below is *declared*,** read from `DESIGN.md` YAML frontmatter, `DESIGN.md`
prose, or a page's `tailwind.config`/`<style>`. The Chrome DevTools MCP pass (next session)
records *computed* values and layout measurements at 1440, 1280 and 1024 px in the "Computed"
sections, and settles conflicts. Do not use the reference `screen.png` files for typography:
`main_workspace_charcoal/code.html` never loads JetBrains Mono and `main_workspace_verdigris`
loads no text fonts at all, so both screenshots render code and titles in a serif fallback.

## Conflicts inside the references

Each concept's frontmatter (which its `code.html` uses) disagrees with its own prose.

| Concept | Frontmatter / HTML | Prose | Proposed rule |
|---|---|---|---|
| Smoked Kraft | base `#141312`, accent `#f6bb84`, text `#e6e2df` | base `#1A1918`, sunken `#141312`, amber `#D19A66`, chalk `#E8E6E1` | Prose for surfaces (it defines the full ladder); amber from both, chosen by contrast in Phase 1 |
| Verdigris Forge | background `#091612`, primary-container `#142822` | canvas `#142822`, deep `#0d1a16`, raised `#1a332c` | Prose ladder; `#091612` as the sunken well |
| Ink & Glass | surface `#f9f9f9`, ink `#1a1c1c` | paper `#FAF9F6`, ink `#000000` | Depends on Q1 in `DECISIONS.md` |
| Radii, Smoked Kraft | HTML config: DEFAULT 2 px, lg 4 px, xl 8 px | frontmatter: sm 2, DEFAULT 4, md 6, lg 8, xl 12 px | Frontmatter; it matches Section 11.4 (4 controls, 6 panels, 8 overlays) |

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

## Ink & Glass (see Q1: light in references, dark in owner spec)

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

## Computed values (pending)

To be filled from Chrome DevTools MCP: colors, font families and metrics, padding, gaps,
borders, radii, shadows, opacity, and layout proportions (rail, explorer, inspector, bottom
dock, tab bar, status bar) for B charcoal, B verdigris, B signal yellow, A charcoal and
K ink-and-paper main-workspace, at 1440, 1280 and 1024 px.
