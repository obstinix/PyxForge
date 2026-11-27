# Anti-slop rules — PyxForge 3

> Structure and several rules descend from the public `refero_skill` anti-slop pattern (MIT) as
> adapted by the `open-design` project. This version was rewritten on 2026-11-27 for PyxForge 3
> (one Go program, Fyne, no web technology). The 2.x version cited files that no longer exist
> (`desktop/src/tokens.css`, `scripts/lint-slop.sh`, a cyan `#00D4FF` accent); it is in git
> history before this date. If an agent works from a cached copy of either original, use this
> file.

PyxForge has two failure modes, not one. Both rule sets matter.

## 0. Two kinds of slop

1. **Visual slop**: the generic AI-tool look. Indigo or violet gradients, glows behind content,
   translucent panes under text, emoji as icons, every panel the same card.
2. **Functional slop**: a control, panel or button that looks as if it works but is not backed by
   anything real. 2.x shipped this repeatedly (registers drawn from simulated data, an "Explain
   CPU" button that was a stub, a hex viewer fed `Math.random()` bytes while a real hex dump sat
   unused). Visual polish cannot cover for it. A design pass that fixes rule set 1 and leaves
   rule set 2 has made the slop prettier, not fixed it.

## 1. Cardinal sins (P0, must fix)

### Visual

1. **Colour from anywhere but the tokens.** Every colour a widget draws comes from
   `theme.Current()` (`internal/ui/theme/tokens.go`): surfaces, text, borders, states, status,
   syntax and the accent of the user's choice (Crimson or Amber, D2). Colours computed from
   tokens (blends, the xterm 256-colour cube in `internal/ui/termview/palette.go`) are fine; a
   hard-coded brand colour is not. Enforced by review; `TestContrast` and
   `TestPalettesAreDistinct` check the tokens themselves.
2. **Gradients and glows in the workspace.** None (D18). Two narrow exceptions, both recorded:
   the 1–1.5 px rim of the opt-in Liquid Glass material on floating surfaces (D21), and the
   graph view's importance halo, which encodes data and can be switched off (D23).
3. **Translucency under text people read.** Never under the editor's or the terminal's body
   text, the sidebars or the status bar. Glass is opt-in (Material: Flat by default) and only on
   floating surfaces (D21). Enforced by `TestFlatOverlays` and, once D21 lands, `TestGlassIsOptIn`
   and `TestGlassContrast`.
4. **Emoji as interface.** No emoji in labels, buttons, titles or statuses. Icons are the bundled
   Lucide line icons (`internal/ui/icons`, 2 px stroke, recoloured from tokens), each listed in
   `docs/design/FONT_LICENSES.md`.
5. **Fonts from anywhere but the bundle.** Geist (interface), Syne (panel and dialog titles) and
   JetBrains Mono (code, terminal, data) are compiled in (`internal/ui/theme/fonts.go`), so the
   app looks the same on every machine and contacts nothing.
6. **Web technology.** None in the application. Enforced by `go run ./tools/forbidcheck`.

### Functional

7. **A control that does not drive real state.** If a button, switch, dropdown or panel exists,
   it changes something real, and a test says so. Concretely:
   - no switch or page for a feature that is not built yet (D22: a Core features page is added in
     the phase that builds its feature);
   - no value presented as read from disk, a debugger, a language server or QEMU that was not
     read from it;
   - a feature that cannot work in the current situation says why (cmd.exe and command blocks,
     D24; a missing language server) instead of drawing an empty or fake result.
8. **A label claiming what it does not have.** Status words in the docs (VERIFIED,
   IMPLEMENTED BUT UNVERIFIED, PLANNED…) and copy in the app ("works offline", "checked against
   clangd") are claims. Each names its evidence: a test, a render that was looked at, or a
   recorded check.

## 2. Enforcement

| Rule | Check |
|---|---|
| 1 | `TestContrast`, `TestContrastReport`, `TestPalettesAreDistinct`, `TestFocusIsNotSelection` (`internal/ui/theme`) |
| 2, 3 | `TestFlatOverlays`; `TestGlassIsOptIn` and `TestGlassContrast` with D21 |
| 6 | `tools/forbidcheck` |
| 7, 8 | review, every feature's named test in `docs/FEATURES.md`, `PROJECT_STATE.md` labels |
| all | the snapshot renders (`go run ./tools/snapshot`), looked at in every theme and accent before a phase is called done |

## 3. Soft tells (P1, should fix)

- **Every panel looking equally dense.** The editor, terminal, Inspector and graph hold different
  kinds of information (text, scrollback, live state, a spatial model); let them look different
  rather than reskinning one card.
- **The accent used more than once or twice per screen.** It marks the active tab of the focused
  region, keyboard focus, and at most one primary action (A1). If half the screen is accented,
  nothing is.
- **Copy that could belong to any tool.** Prefer the domain's words: "Run in QEMU", "Step
  Instruction", "boot signature 55 AA", not "Unlock powerful debugging".

## 4. Polish tells (P2)

- Decorative shapes with no information.
- Symmetric side panels with no weighting toward the editor, where the time is spent.
- Motion that is not tied to a change of state, or that ignores Reduce Motion.

## 5. Where character comes from

Mostly proven patterns, plus a few distinctive choices where they are earned:

- **Voice from the domain.** Respawn Shell, Step Over Instruction, the boot-sector map.
- **One micro-interaction tied to a real event**, such as a register value marked when a step
  changed it, never as decoration.
- **Details only someone who has debugged a bootloader would add**: the `55 AA` signature on the
  map, the real-mode disassembly, QEMU serial output beside the code.

If a screenshot of PyxForge could be mistaken for a generic AI-generated dev-tool mock-up, it is
not done. If a screenshot of a panel could be mistaken for working when it is not wired to
anything real, that is the worse failure. Check for both.
