# Anti-AI-Slop Rules — PyxForge

> Structure and several P0 rules descend from the public `refero_skill` anti-slop pattern (MIT) as adapted by the `open-design` project. That version references a daemon and lint infra (`apps/daemon/src/lint-artifact.ts`, `data-od-id`, `.ph-img`) that don't exist in this repo — this version is rewritten against PyxForge's actual files, tokens, and CI, not copied verbatim. If you're an agent working from a cached copy of the original, use *this* file, not that one.

PyxForge has two failure modes, not one. Rule sets 1 and 2 below both matter.

## 0. Two kinds of slop in this repo

1. **Visual slop** — the generic-AI-dashboard look: indigo/violet gradients, glassmorphism, glow buttons, emoji icons. This is what most anti-slop checklists (including the one this file is adapted from) are written for.
2. **Functional slop** — a control, panel, or button that *renders* like it works but isn't backed by anything real. This project has hit this exact failure mode repeatedly and concretely: the CPU Inspector's register/memory views were built against simulated data rather than a live GDB session, the "✨ Explain CPU" button was a hardcoded stub, and (until Phase 22 of this plan) the entire "Virtual Workspace Explorer" fed a hex viewer with `Math.random()` bytes instead of real file contents while a working `HexDump` RPC sat unused in `core/`. Visual polish cannot cover for this, and a design pass that only fixes rule-set 1 while leaving rule-set 2 in place has not actually fixed the slop — it's made it prettier.

## 1. Cardinal sins — P0, must-fix

**Visual**

1. **Off-brand accent color — P0 (enforced).** The only accent this repo uses is `--accent: #00D4FF` (defined in `desktop/src/tokens.css` and mirrored via CSS custom properties in the extension webviews). Any of the following hardcoded anywhere outside `tokens.css` itself is a violation, whether it's stock Tailwind indigo or the specific blue/purple this codebase already shipped once: `#6366f1`, `#4f46e5`, `#4338ca`, `#3730a3`, `#8b5cf6`, `#7c3aed`, `#a855f7`, `#3b82f6`, `#2563eb`, `#1d4ed8`, `#1e1b4b`, `#cba6f7`.
2. **Gradients on background, text-fill, or buttons — P0 (enforced).** No `linear-gradient` / `radial-gradient` anywhere in `desktop/src/` or `extension/src/`. A flat surface plus intentional type beats a two-stop "trust gradient" every time — this is exactly what the original `.brand-name` text-fill and `.app-container` radial background were, and both are gone as of Phase 21.
3. **`backdrop-filter` anywhere — P0 (enforced).** No glassmorphism, no exceptions for "just this one modal."
4. **Emoji as functional UI — P0 (enforced).** No emoji inside `<button>`, `<h*>`, `.preset-name`, `.form-label`, or any element representing a file, action, or status. Use `lucide-static` SVGs (2px stroke, `currentColor`) per Appendix A §4.
5. **Wrong or CDN-loaded fonts — P0 (enforced).** Display text uses `var(--font-display)` (Space Grotesk), code/data uses `var(--font-mono)` (JetBrains Mono), both self-hosted via `@fontsource/*`. No `fonts.googleapis.com` import, no hardcoded `Outfit`, `Inter`, `Roboto`, or `system-ui` stack in a display context — those are the "friendly AI SaaS" defaults this project already had to remove once.

**Functional (PyxForge-specific)**

6. **A control that doesn't drive real state — P0 (enforced where grep-detectable, guidance otherwise).** If a dropdown, button, or panel exists, it must change something real. Two concrete, bannable patterns:
   - `document.documentElement.setAttribute('data-theme', ...)` (or equivalent) with no corresponding `var(--...)` consumer anywhere in the stylesheet it's meant to affect — this exact bug shipped once.
   - `Math.random()` used to fabricate data presented as read from disk, a debugger, or any other real source, anywhere in `desktop/src/main.ts` or `extension/src/*.ts`. Randomness is fine for actual randomness (jitter, IDs); it is never fine standing in for unread real data.
7. **A label claiming provenance it doesn't have.** Comments/UI copy like "pwndbg-inspired" (`desktop/index.html`, disassembly context card) or "genuinely ported" are claims — verify them against the thing they cite before merging, don't just keep the comment because it sounds credible. **(guidance, not auto-checked)**

## 2. Enforcement script

See `scripts/lint-slop.sh`.

## 3. Soft tells — P1, should fix (guidance, not auto-checked)

- **Every panel looking equally dense.** The Inspector, Terminal, and Hex Viewer are structurally different kinds of information (live state vs. scrollback vs. static bytes) — let them look different, not like three reskins of the same card component.
- **More than ~12 raw hex values outside `tokens.css`.** If you're hardcoding a color, the token system isn't being used.
- **`var(--accent)` used more than 2–3 times per screen.** Cyan marks the one or two things that matter (an active tab, a focused input, a live status) — if half the screen is cyan, nothing on it is emphasized.
- **Copy that could belong to any dev tool.** "Backend Connection" is fine because it's accurate; "Unlock powerful debugging" would not be, because it's marketing voice bolted onto a tool built for one person's own OS-dev workflow.

## 4. Polish tells — P2, nice to fix (guidance, not auto-checked)

- Decorative geometric background shapes with no informational purpose.
- Perfectly symmetric panel widths with no visual weighting toward the panel the user actually spends time in (the center workspace should read as primary, not equal to the two side rails).

## 5. How to add soul without breaking the rules

Aim for mostly proven patterns plus a few genuinely distinctive choices, and put the distinctive 20% where it's actually earned:

- **One typographic move**, not five — e.g. the register names in the CPU Inspector staying in mono while their values get a subtle weight contrast on change, instead of a new font somewhere random.
- **Voice from the actual domain.** "Respawn Shell" (already in the repo) is a good example — specific to what the button does, not generic ("Restart"). Keep hunting for this register/pwndbg/QEMU-flavored vocabulary instead of defaulting to generic SaaS verbs.
- **One micro-interaction tied to a real event** — the existing `register-value.changed` flash-red animation on a real register diff is the right shape: it exists *because* something real happened, not as decoration.
- **A detail only someone who's actually debugged a bootloader would add** — e.g. the boot-signature (`0xAA55`) callout in the hex viewer, once it's reading a real file instead of injecting a fake one.

If a screenshot of PyxForge could be mistaken for a generic AI-generated dev-tool mockup, it isn't done. If a screenshot of a *panel* could be mistaken for working when it isn't wired to anything real, that's the other, more important way this project has shipped slop before — check for both.
