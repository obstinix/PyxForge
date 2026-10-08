# PyxForge Design System

Version for Phase 1, 2026-10-08. The code is the source of truth for values:
`internal/ui/theme/`. This document explains the model, cites every source, and records the
rules that produced derived values. Owner decisions referenced as D1–D9 and Q1–Q6 are in
`docs/architecture/DECISIONS.md`; theme requirements are in `THEME_SYSTEM_REQUIREMENTS.md`.

## 1. Principles

1. **One component system, five token sets.** Widgets read semantic tokens and never branch on
   a theme's name (D2).
2. **Readability first where the work happens.** Editor, terminal, debugger, hex, disassembly and
   logs are opaque and quiet. Glass is for things that float (Section 11.5).
3. **The accent marks what matters.** One or two accented things per screen: keyboard focus, the
   active tab of the focused region, the current instruction. Never decoration, and never a
   status that is merely "on" (decision A1).
4. **Depth from tone, not effects.** Flat surfaces separated by 1 px rules; one soft shadow only
   under overlays; no gradients, glow or lift (Section 11.8).
5. **Every value has a source** in a reference file, the owner's brief, a contrast calculation,
   or a written rule below. `TestContrast` enforces the accessibility rules in code.

## 2. Token model

```text
Selection {palette ID or "system", accent ID}   ← Settings → Appearance, persisted
   │  resolved per OS appearance (System mode, Q5)
   ▼
Tokens = Palette (surfaces, text, borders, states, glass, status, syntax)
       + AccentSet for the palette's polarity (primary, hover, active, muted,
         focus, selection, border, onPrimary)
   │
   ├── Fyne theme adapter (internal/ui/theme/fyne.go) → every Fyne widget
   ├── PyxForge widgets: theme.Current(), re-read in Refresh on every theme change
   ├── Editor theme (Phase 3): Syntax + Surface → Neovim highlight groups
   ├── Terminal theme (Phase 4): Surface.Sunken, Text, Syntax/Status → ANSI 16
   └── Diagnostics: Status colours
```

| Group | Tokens | Used for |
|---|---|---|
| Surface | Base, Raised, Sunken, Overlay | panels and editor; rail, tab bars, headers; terminal, status bar, inputs; overlay body |
| Text | Primary, Secondary, Tertiary, Disabled | reading hierarchy |
| Border | Hairline, Strong | panel dividers; inputs and emphasized edges |
| State | Hover, Pressed, Selected | translucent washes over any surface |
| Glass | Fill, Edge, Rim, Scrim, Shadow (+ blur, offset), Blur | overlay material |
| Status | Success, Warning, Error, Info, OnStatus | diagnostics, notifications, process state |
| Syntax | Comment, Keyword, Type, Function, Variable, Constant, String, Number, Operator, Attribute, Error | editor and terminal |
| Accent | Primary, Hover, Active, Muted, Focus, Selection, Border, OnPrimary | focus, active markers, primary actions |

Shared metrics (spacing, sizes, radii, type scale) live in `metrics.go` and are the same for
every theme.

## 3. Source key

| Key | File |
|---|---|
| SK:n | `target ui reference 2/…/smoked_kraft_charcoal_ide/DESIGN.md`, line n |
| VF:n | `…/verdigris_forge/DESIGN.md`, line n |
| IG:n | `…/ink_glass/DESIGN.md`, line n |
| K | `pyforge_ui_kit/ink-and-paper/main-workspace/code.html` (`tailwind.config`, measured) |
| L | 2.x `legacy/desktop/src/themes/ink-and-paper.css` |
| D | 2.x `docs/DESIGN.md` §3 tokens |
| M | 2.x `legacy/desktop/src/themes/mono.css` |
| computed | measured values in `SOURCES.md` → "Computed values" |
| Section n | `refactor-v2.md` |

## 4. Rules for derived values

A value marked *rule* or *derived* in the code comes from one of these:

| Rule | Statement | Why |
|---|---|---|
| Alpha ladder | Dark themes: Hover = Text.Primary at 4 %, Selected 8 %, Pressed 12 %, Hairline 8 %. Light themes: black at 4 / 6 / 10 % and hairline 10–12 %. | SK:251 and SK:182 define the ladder for Smoked Kraft; L defines 6 % and 12 % for Ink & Paper. Applying the same ladder keeps states identical in feel across themes. |
| Info = Text.Secondary | Info is neutral except in Verdigris, which has its own secondary sage. | Info is not alarming; colouring it would add a hue with no meaning. |
| Darken to 4.5:1 | Ink & Paper and Ink & Glass tertiary `#7e7576` measures 4.25:1 on paper; it is darkened to `#6e6566` (5.37:1). | Tertiary text carries metadata a user must read (Section 16.3). |
| Disabled weaker | Disabled text is about 2.7:1 on light themes and the reference placeholder colour on dark ones; `TestContrast` requires it to be weaker than Tertiary. | WCAG exempts disabled text; it must still read as unavailable, not as metadata. |
| Lighten to 4.5:1 | Smoked Kraft clay `#c75a4a` (SK:186) measures 4.17:1; lifted to `#d4685a` (4.96:1). | Errors are read as text in the Problems list. |
| Neutralize | Monochrome takes D's `#8a8f98`/`#5a5f68`, which lean blue, and removes the hue: `#8c8c8c`, `#5a5a5a`. | "Near-black / white / gray foundation" (D2 text). |
| Dark scrim | Black at 45–50 % behind modals on dark themes; ink at 18–20 % on light themes. | Section 11.5 asks for a dimmed scrim; this dims enough to read as inactive while overlay text keeps its own contrast. |
| Dark shadow | Overlays on dark themes without their own value use black 60 %, blur 32, offset 12 (Smoked Kraft's SK:217 is 65 %, 36, 16). | One soft shadow, overlays only (Section 11.8). |
| Light literals | Ink & Paper colours constants, strings and numbers one sepia `#6b4e2e`; Ink & Glass uses its own greys (IG:24, IG:43). | Owner: restrained, no rainbow, no shared palette across themes. |
| Monochrome syntax | A neutral ramp from Text.Tertiary to white; keywords are white and bold. | Monochrome has no hue to spend; weight separates keywords. |

## 5. Themes

Five palettes (D2, Q1, Q4). Polarity decides which accent set applies.

| Theme | Polarity | Base / Raised / Sunken / Overlay | Text primary / secondary / tertiary | Character |
|---|---|---|---|---|
| Smoked Kraft | dark (primary) | `#1c1916` / `#26211b` / `#14120f` / `#221e1a` | `#e8e6e1` / `#b5b0a8` / `#969088` | warm kraft-brown charcoal, chalk text, densest |
| Verdigris Forge | dark | `#111e1a` / `#1f2d28` / `#091612` / `#142822` | `#f4ede2` / `#c2c8c4` / `#8c928f` | oxidized green, cream text, copper operators |
| Monochrome | dark | `#0c0c0c` / `#151515` / `#000000` / `#171717` | `#ededed` / `#a0a0a0` / `#8c8c8c` | true neutral greys over a black well, crisp hairlines |
| Ink & Paper | light (primary) | `#faf9f6` / `#ffffff` / `#f3f3f4` / `#ffffff` | `#1a1c1c` / `#4c4546` / `#6e6566` | warm paper, flat, white chrome, sepia literals |
| Ink & Glass | light | `#f3f6f9` / `#dde5ec` / `#ebeff3` / `#ffffff` | `#0b0d10` / `#3e4651` / `#56606b` | cool page, blue-grey chrome, blue-black literals, clearest glass |

Signatures (Phase 1 review). At shell scale the surfaces fill the window, so each theme must be
recognisable from them alone. The reference greys for Smoked Kraft and Monochrome, and for
Ink & Paper and Ink & Glass, were nearly identical, so three palettes moved off their references:
Smoked Kraft warmed toward kraft, Monochrome deepened to true neutrals over black, and Ink & Glass
cooled to a blue-grey chrome. `TestPalettesAreDistinct` keeps every same-polarity pair at least
ΔE 15 apart, summed over base, raised and sunken (measured: Smoked Kraft–Monochrome 20.3,
Ink & Paper–Ink & Glass 16.7, Smoked Kraft–Verdigris 23.7, Verdigris–Monochrome 30.5).

Each value's source is the comment beside it in `internal/ui/theme/<theme>.go`. Status colours:

| Theme | Success | Warning | Error | Info |
|---|---|---|---|---|
| Smoked Kraft | `#8a9a7b` SK:187 | `#d19a66` SK:185 | `#d4685a` SK:186 lifted | = secondary |
| Verdigris Forge | `#72b896` VF:164 | `#d48243` VF:163 | `#ffb4ab` VF:32 | `#a1d1bf` VF:24 |
| Monochrome | `#10b981` D | `#f59e0b` D | `#ef4444` D | = secondary |
| Ink & Paper, Ink & Glass | `#2f6b3a` derived | `#8a5300` derived | `#ba1a1a` K, IG:32 | = secondary |

Known overlap: Smoked Kraft's warning amber and the Amber accent share a hue, as they do in the
reference (SK:185 uses amber for both). Warnings always carry an icon, so hue is never the only
signal.

## 6. Accents

Independent of theme (D2), one value set per polarity (Q6).

| Accent | Polarity | Primary | OnPrimary | Selection | Source |
|---|---|---|---|---|---|
| Crimson | dark | `#e0566a` | `#1a0a0d` | primary at 30 % | derived: lifted until it reads as text (≥ 4.5:1) on every dark Base |
| Crimson | light | `#b0243a` | `#ffffff` | primary at 18 % | derived: deepened until white text reads on it (≥ 4.5:1) |
| Amber | dark | `#f6bb84` | `#2a1400` | primary at 24 % | SK:28, SK:30; borders at 50 % as rendered (computed) |
| Amber | light | `#9a5b13` | `#ffffff` | primary at 18 % | derived from SK amber, deepened for white text |

Measured by `TestContrastReport` (2026-10-08), tightest cases:

| Theme + accent | Tertiary / base | Error / base | Accent text / base | On-accent | Focus / raised |
|---|---|---|---|---|---|
| Smoked Kraft + Crimson | 5.53 | 4.94 | 4.76 | 5.22 | 4.34 |
| Smoked Kraft + Amber | 5.53 | 4.94 | 10.30 | 10.31 | 9.39 |
| Verdigris + Crimson | 5.41 | 10.10 | 4.66 | 5.22 | 3.90 |
| Verdigris + Amber | 5.41 | 10.10 | 10.09 | 10.31 | 8.44 |
| Monochrome + Crimson | 5.82 | 5.20 | 5.32 | 5.22 | 4.96 |
| Monochrome + Amber | 5.82 | 5.20 | 11.51 | 10.31 | 10.75 |
| Ink & Paper + Crimson | 5.37 (5.10 on sunken) | 6.14 | 6.31 | 6.65 | 6.65 |
| Ink & Paper + Amber | 5.37 (5.10 on sunken) | 6.14 | 5.14 | 5.41 | 5.41 |
| Ink & Glass + Crimson | 5.90 (5.54 on sunken) | 5.96 | 6.13 | 6.65 | 5.22 |
| Ink & Glass + Amber | 5.90 (5.54 on sunken) | 5.96 | 4.99 | 5.41 | 4.25 |

## 7. Typography

| Role | Family | Size / weight | Where | Source |
|---|---|---|---|---|
| UI body | Geist Regular | 13 px | labels, tree rows, menus | Section 11.3; SK:100 body-md |
| UI strong | Geist SemiBold | 13 px | Fyne bold, emphasis | Section 11.3 |
| Label | Geist Medium | 11 px, +0.04 em, uppercase for section labels only | dock tab captions, section labels | SK:124–135 |
| Heading | Geist SemiBold | 16 px | settings headings | SK:88 title-md |
| Display | Syne SemiBold | 13–16 px, uppercase, +0.06 em for panel titles; 16 px sentence case for dialog titles | panel and dialog titles only | Section 11.3; SK:192 |
| Code and data | JetBrains Mono | 13 px; bold for keywords; italic for comments | editor, terminal, registers, hex | Section 11.3; SK:194 |

Rules: no functional text below 11 px (Impeccable `undersized-ui-text`); wide tracking only on
short uppercase labels (`wide-tracking`); body line height at least 1.3 (`tight-leading`).
Fyne's `canvas.Text` has no letter-spacing control, so the tracking values above are not rendered
in 3.0: panel titles are uppercase Syne without added tracking. Geist
has no italic: UI text never relies on italics. Syne is applied per text object through
`canvas.Text.FontSource` (Fyne 2.5+), so the theme's single regular font stays Geist.

The reference concepts never rendered Syne, Geist or JetBrains Mono (`SOURCES.md`). The Phase 1
review renders are the first time this pairing is actually seen; see `PHASE1_REVIEW.md`.

## 8. Spacing, sizing, radii

| Token | Value | Source |
|---|---|---|
| Spacing | 4, 8, 12, 16, 24 | Section 11.4, 4 px grid |
| Rail | 48 | Section 11.6, VF:181 |
| Explorer | 256 (resizable) | rendered width of every concept's sidebar (computed) |
| Inspector | 296 (collapsible) | Verdigris inspector at 1440 px, 291, rounded to the grid (computed) |
| Bottom dock | 220 (resizable) | — chosen so a 640 px window keeps ≥ 300 px for the editor |
| Tab bar | 32 | — fits 13 px text with 8 px padding and a 2 px active marker |
| Status bar | 24 | rendered status bars are 28–32 px; 24 is the denser IDE norm, still 2× the 11 px caption |
| Tree / list row | 24 | SK:251 |
| Palette row | 36 | SK:240 |
| Icons | 16, 20 on the rail | Section 11.4 |
| Focus ring | 2 px, Accent.Focus | Section 11.2 |
| Radii | controls 4, panels 6, overlays 8 | Section 11.4 (the concepts' 2 px is too tight for a visible focus ring) |

No card-in-card: panels are flush regions separated by hairlines, never boxes inside boxes
(Impeccable `nested-cards`, 67 static findings in the concepts).

## 9. Glass and overlays

Allowed on: command palette, dialogs, notifications, workspace switcher, floating inspector,
agent overlays (owner D2 list; Section 11.5). Never on editor, terminal, debugger, logs,
registers, hex or disassembly.

Glass is an appearance setting, **off by default** (Phase 1 review answer G1 in `DECISIONS.md`;
it revises D3 and Q2). Any palette can use it. One recipe, two bodies:

| Part | Setting off (default) | Setting on |
|---|---|---|
| Body | Surface.Overlay, opaque | Glass.Fill, a 70–85 % opaque palette tint |
| Backdrop | none | `canvas.Blur` under the body, radius Glass.Blur (20; Ink & Glass 24), rounded to 8 |
| Rim, edge, shadow, scrim | 1 px Glass.Rim, 1 px Glass.Edge on the top edge, one Glass.Shadow, Glass.Scrim behind modals | same |

Off, nothing shows through an overlay. On, what shows through is blurred, so text behind the
palette or the floating inspector never ghosts through legibly (Phase 1 review: at 94 % with no
blur it did). `TestGlassSetting` enforces both modes and `Tokens.WithGlass` applies the setting.
Fyne's own modal dialogs blur the window behind them through `modalBlurRadius`, which follows the
same setting. Ink & Paper keeps no specular edge in either mode (it is the flat theme).

## 10. Shadows

Only overlays cast a shadow: Glass.Shadow, blur 24–36, offset 8–16, drawn with
`canvas.Rectangle.Shadow` (Fyne 2.8). Docked panels never cast one. No coloured or glowing
shadows (Impeccable `dark-glow`, `gpt-thin-border-wide-shadow`).

## 11. Icons

Lucide line icons, one stroke style (24 px grid, 2 px round strokes), drawn at 16 px (20 px on the
rail), coloured with Text.Secondary by default and Text.Primary when active. Icons are never
accented. License and list: `FONT_LICENSES.md`. No emoji anywhere.

## 12. States

| State | Treatment |
|---|---|
| Default | surface colour, Text.Secondary for icons and inactive labels |
| Hover | State.Hover wash; icon and label to Text.Primary |
| Focus-visible | 2 px Accent.Focus ring inside the control's radius; keyboard focus only. Fyne's tree and list cannot draw one per row, so the explorer and the Log list draw it around the whole list (`kit.FocusFrame`) and Fyne marks the row with the hover wash. Fyne's own buttons and menu items show State.Focus, a neutral wash |
| Pressed | State.Pressed wash |
| Selected | State.Selected wash plus a 2 px Text.Primary marker on the leading edge (rail items for visible panels). Choice cards (Settings) show a Text.Primary check badge and a Border.Strong outline; a stroke would read as the focus ring (Phase 1 review) |
| Active tab | In the active region (the one with keyboard focus or last used): Accent.Primary label and 2 px underline. In every other region: Text.Primary label and underline. One tab bar is accented at a time (decision A1); the explorer has no tabs, so while it is active none is |
| Disabled | Text.Disabled; no hover or press response |
| Keyboard activation | Space, Enter and Return activate every PyxForge control; Tab and Shift+Tab move between them in workbench order (rail, explorer, editor, panel, inspector, status bar) |
| Loading | label stays; a determinate bar when progress is known; never a spinner without a label |
| Error / warning / success | Status colour on the icon and the leading marker; text stays Text.Primary |

## 13. Motion

No idle animation, no pulsing dots, no wiggle or lift. Colour and focus changes may transition in
up to 120 ms; Phase 1 switches instantly. Reduced-motion settings are honoured by having nothing
to reduce.

## 14. Accessibility

- `TestContrast` checks, for all 10 theme × accent combinations: every text level on every
  surface (4.5:1), status and syntax colours on base and sunken (4.5:1), accent as text (4.5:1),
  on-accent text (4.5:1), the focus ring on base, raised and overlay (3:1), and text on selection,
  selected and pressed rows (4.5:1).
- Full keyboard operation; the command palette reaches every command.
- 11 px floor for functional text.

## 15. Density

One density in Phase 1, the compact scale above. A "comfortable" density, if wanted, scales
spacing and row heights only, never type below the floor.

## 16. Fyne mapping

| Fyne colour name | Token |
|---|---|
| background | Surface.Base |
| button, headerBackground | Surface.Raised |
| disabledButton, inputBackground | Surface.Sunken |
| menuBackground, overlayBackground | Glass.Fill after the glass setting, so opaque Surface.Overlay by default (Fyne's own dialogs and menus float; Fyne draws their shadow but not the rim or specular edge) |
| foreground / placeholder / disabled | Text.Primary / Tertiary / Disabled |
| separator, innerWindowBorderInactive | Border.Hairline |
| inputBorder, innerWindowBorder | Border.Strong |
| hover / pressed | State.Hover / State.Pressed |
| primary, hyperlink / foregroundOnPrimary | Accent.Primary / Accent.OnPrimary |
| focus | State.Focus, a neutral wash (Fyne blends focus over button, menu-item and check backgrounds); `TestFocusIsNotSelection` keeps it at least ΔE 8 from selection |
| selection | Accent.Selection |
| shadow | Glass.Scrim (Fyne draws it behind modals) |
| success, warning, error / foregroundOn* | Status.* / Status.OnStatus |

Sizes: text 13, caption 11, subheading 14, heading 16, padding 4, inner padding 8, input and
button radius 4, card radius 6, dialog/popup/menu radius 8, modal blur radius = Glass.Blur after
the glass setting (0 by default).
