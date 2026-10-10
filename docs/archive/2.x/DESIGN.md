# PyxForge Design System Specification

## 1. Diagnosis: why the current UI reads as AI slop

**1.1 — The palette contradicts your own design system.**
`desktop/src/styles.css` runs blue→purple throughout: the app background is `radial-gradient(circle at 50% 0%, #1e1b4b 0%, #090d16 80%)`, the brand wordmark is a `linear-gradient(to right, #3b82f6, #8b5cf6)` text-fill, and every primary button is `linear-gradient(135deg, #2563eb, #1d4ed8)`. This is the single most recognizable "AI-generated SaaS dashboard" palette on the internet right now. It has nothing to do with cyan `#00D4FF`, which is the accent across every other one of your projects.

**1.2 — Glassmorphism, explicitly.** The titlebar uses `backdrop-filter: blur(12px)` over `rgba(15,23,42,0.8)`; every panel is `rgba(15,23,42,0.4)`; cards, badges, and the connection-status pill are all translucent `rgba` stacks. This is the exact pattern you've banned everywhere else.

**1.3 — Glow-and-lift buttons.** `.btn:hover` does `transform: translateY(-1px)` plus a blue drop shadow that intensifies (`box-shadow: 0 6px 16px rgba(37,99,235,0.35)`). Combined with the gradient fill, this is a stock "generated dashboard" button, indistinguishable from a thousand other AI scaffolds.

**1.4 — Wrong typeface, loaded from a CDN.** `styles.css` imports `Outfit` (a rounded geometric sans that's become a default "friendly AI app" font) from `fonts.googleapis.com` at runtime — an external network dependency inside a desktop app that's supposed to work offline. JetBrains Mono is already there and correctly used for monospace fields, but the display font never matches your standard (Space Grotesk) anywhere in this repo.

**1.5 — Emoji as UI icons.** The file explorer entries use `📄`, and the CPU Inspector's AI button is literally `✨ Explain CPU`. Emoji-as-icon is one of the fastest tells of unreviewed AI output. Meanwhile the titlebar already has a perfectly good custom SVG bracket mark (`.brand-logo`, two polylines) that nothing else in the app follows the style of.

**1.6 — Three uncoordinated design languages in one product.**
- Desktop shell (`desktop/src/styles.css`): blue/purple/indigo, described above.
- Extension webviews (`extension/src/inspectorPanel.ts`, lines ~159–165): a Catppuccin-Mocha-derived palette — `--background-color:#1e1e2e`, `--accent-color:#cba6f7` (mauve/purple), `--header-background:#11111b`.
- The theme system (`desktop/src/themes/{mono,hybrid,contrast}.css`): a third, *unused* set of tokens.

These don't share a single hex value. A user moving between the desktop app and the VS Code extension is looking at two products that don't visually acknowledge they're related.

**1.7 — The theme switcher switches nothing.** `index.html` ships a "Theme: Hybrid / Mono / Contrast" dropdown, and `main.ts` faithfully sets `document.documentElement.dataset.theme` when you change it. But `desktop/src/styles.css` — the stylesheet that actually renders every component — contains **zero** `var(--...)` references (`grep -c "var(--" styles.css` → `0`). The theme files define `--accent-color`, `--background-color` etc., but nothing downstream reads them. It's a fully wired control connected to nothing. That's worse than not having the feature — it's a feature that *looks* real and isn't, which is exactly the pattern flagged before in this project (simulated CPU Inspector, stubbed "Explain CPU" button).

**1.8 — The shell itself was never rebranded out of scaffold defaults.**
- `desktop/src-tauri/tauri.conf.json`: `"productName": "desktop"`, window `"title": "desktop"`, default `800×600` size.
- `desktop/src-tauri/Cargo.toml`: `description = "A Tauri App"`, `authors = ["you"]` — the literal placeholder text.
- `desktop/src-tauri/icons/*`: every icon slot (`32x32.png` through `icon.icns`/`icon.ico`) is still the stock Tauri gem/hexagon logo. Confirmed by inspection, not just filename — it's the default asset.

None of this is a matter of taste — it's the difference between "someone built this" and "someone ran `create-tauri-app` and never went back."

## 2. Design principles for this pass

1. **One accent, used sparingly.** Cyan `#00D4FF` is the only saturated color in the UI. Status colors (success/error/warning) are the only exceptions, because they carry information, not branding.
2. **Flat, not glowing.** No `backdrop-filter`, no gradient fills, no drop-shadow glows on idle or hover states. Depth comes from a one-step lightness change and a 1px hairline border, not blur or shadow.
3. **Real hierarchy from type and spacing, not gradients.** Space Grotesk carries weight/size contrast for headers and labels; JetBrains Mono stays on anything that is data, code, or a value a developer might copy.
4. **Every control that renders must do something.** A theme switcher that doesn't theme anything, or an icon that doesn't map to a real state, is a liability, not a feature. If it's not wired end-to-end, cut it for this pass rather than ship it inert.
5. **Icons are line-drawn SVG, one stroke weight, matching the existing brand mark.** No emoji, no filled glyphs mixed with outline glyphs.

## 3. Design tokens

```css
:root {
  /* Surface */
  --bg-void:            #0A0A0A;   /* app background — flat, no gradient/radial */
  --bg-surface:         #111111;   /* panels — solid, not rgba */
  --bg-surface-raised:  #161616;   /* cards, hover targets */
  --bg-inset:           #050505;   /* terminal / log / hex viewer wells */

  /* Borders */
  --border-hairline:    rgba(255,255,255,0.08);
  --border-hairline-strong: rgba(255,255,255,0.16);

  /* Text */
  --text-primary:       #EDEDED;
  --text-secondary:     #8A8F98;
  --text-tertiary:      #5A5F68;

  /* Accent — cyan only, no second hue */
  --accent:             #00D4FF;
  --accent-dim:         rgba(0, 212, 255, 0.10);   /* hover/active fills */
  --accent-border:      rgba(0, 212, 255, 0.35);

  /* Status (functional, not brand) */
  --status-success:     #10B981;
  --status-error:       #EF4444;
  --status-warning:     #F59E0B;

  /* Type */
  --font-display: 'Space Grotesk', sans-serif;
  --font-mono:    'JetBrains Mono', monospace;

  /* Radius — modest, consistent, unchanged in scale from today */
  --radius-sm: 4px;
  --radius-md: 6px;
}
```

Fonts are self-hosted via `@fontsource/space-grotesk` and `@fontsource/jetbrains-mono` (both real, current npm packages) — imported in TS, not fetched from Google Fonts at runtime. This matters specifically because PyxForge Desktop is meant to work offline while debugging a QEMU VM.

**Explicitly banned, repo-wide:** `backdrop-filter`, any `linear-gradient` or `radial-gradient` on background/text/button fill, `box-shadow` used for glow (a shadow used purely for elevation, e.g. `0 1px 2px rgba(0,0,0,0.4)`, is fine — a shadow that changes color to match the accent on hover is not), emoji as functional icons, `translateY` hover lift.

## 4. Component specs

**Titlebar / brand.** Solid `--bg-surface` with a single `1px` bottom hairline — no blur. Wordmark is solid `--text-primary`, not a gradient fill. Keep the existing SVG bracket mark; drop its `drop-shadow` glow filter.

**Buttons.**
- Primary: `background: var(--accent-dim)`, `border: 1px solid var(--accent-border)`, `color: var(--accent)`. On hover, `background` steps up to `rgba(0,212,255,0.18)` — no transform, no shadow.
- Secondary: `background: var(--bg-surface-raised)`, `border: 1px solid var(--border-hairline)`, `color: var(--text-secondary)`.
- Both keep the current `6px` radius and padding; only fill/border/shadow logic changes.

**Inputs.** Keep current structure; focus ring becomes `0 0 0 1px var(--accent)` (a hairline, not a soft glow spread).

**Panels / cards.** Solid `var(--bg-surface)`, `1px solid var(--border-hairline)`, no `rgba` transparency stacking, no blur.

**Status dot.** Keep the dot; drop `box-shadow: 0 0 8px` glow — a plain filled circle with the status color is enough at this size.

**Tabs.** Active tab gets a `2px` bottom border in `--accent` and `--text-primary` label color; inactive tabs use `--text-secondary`. No background pill, no gradient underline.

**File tree items.** Replace `📄` with a single-stroke SVG file glyph (differentiate directories vs. files vs. binary via a second small glyph or by dimming binary/unknown files — not by emoji swap).

**Icon set.** Use `lucide-static` (real npm package, MIT-licensed SVGs, consistent 2px stroke) for all functional icons — file/folder, save, run, step, terminal, chip (Inspector), search. Import the raw SVG strings; no React dependency required since this is a vanilla TS app.

## 5. Cross-frontend policy

Desktop is the primary product per the 2026-07-19 checkpoint decision (`docs/architecture/CHECKPOINTS.md`); the VS Code extension is the preserved baseline. They don't need pixel-identical UI — VS Code webviews should still feel native to VS Code — but the **accent color and status colors should match** (`#00D4FF`, `#10B981`, `#EF4444`) so a screenshot of either one is recognizably "PyxForge." Right now the extension's mauve `#cba6f7` accent guarantees they don't. This is a small, low-risk change confined to the CSS custom properties block in `inspectorPanel.ts`, `hexPanel.ts`, and `aiPanel.ts` — swap the accent variable only, don't restructure the webviews.

## 6. Do-not-reintroduce list

- Decorative controls that don't affect render output (the current theme selector). If theming comes back later, it must actually flow through `var(--...)` in `styles.css`, or don't ship the dropdown.
- Any second saturated hue alongside cyan "for variety."
- `backdrop-filter` anywhere, including "just this one modal."
- Emoji in place of an icon component, including in placeholder/TODO UI.

## 7. Shell branding fixes

- `tauri.conf.json`: `productName` → `"PyxForge"`, window `title` → `"PyxForge"`, bump default window size to something a 3-pane IDE layout actually needs (e.g. `1280×800`).
- `Cargo.toml`: `description` → an actual one-line description, `authors` → `["obstinix"]`.
- `icons/*`: regenerate from a real PyxForge mark (the existing bracket SVG from the titlebar is the obvious source) via `tauri icon`, replacing every file in the directory — not just the largest one.
