# PyxForge 3.0 Decisions

Answered by the owner (`obstinix`) on 2026-10-08. Precedence when instructions conflict:
master prompt Section 2 (non-negotiables), then the answers below, then everything else.

| ID | Decision | Answer | Notes |
|---|---|---|---|
| D1 | Backend language | **Go for everything.** Port `pyxforge-core` into Go packages. | The 59 Rust unit tests (all passing on 2026-10-08, see `CURRENT_STATE.md`) and the extension's 10 diagnostic/preset tests become golden fixtures. Fall back to a Rust sidecar only per module, with a recorded reason. |
| D2 | Visual direction | **Five first-class themes plus an independent accent system.** Smoked Kraft, Ink & Paper, Ink & Glass, Verdigris Forge, Monochrome. Accents: at least Crimson and Amber, selectable independently of theme. Plus a System mode. | Full owner text: `docs/design/THEME_SYSTEM_REQUIREMENTS.md`. Supersedes the recommended default and Section 7's "at most 3 themes". Open questions below. |
| D3 | Glass | **Tinted near-opaque overlay surfaces with a 1 px specular edge, no blur, as the baseline.** | Native blur (DWM Mica/acrylic, macOS vibrancy) is an optional later enhancement behind an interface. Glass on overlays only. See Q2 for how this meets Ink & Glass's "subtle blur where supported". |
| D4 | Editor engine | **Real Neovim over the UI protocol** (`nvim --embed`, `nvim_ui_attach`, `ext_linegrid`), rendered natively in Fyne, gated by a Phase 3 spike. | Fallback: Neovim in a PTY pane, with the reason recorded in `docs/decisions/`. CodeMirror is retired either way. |
| D5 | Platforms for 3.0 | **Windows and Linux released; macOS compile-checked only.** | WSL2 is the Linux development path. |
| D6 | Repository delivery | **Work on branch `v3`; merge to `main` at the parity gate; push `v3` after every milestone.** | `v3` was created from `origin/main` at `9d273f5` on 2026-10-08. |
| D7 | Commit attribution | **Commits authored only as `obstinix`; no AI co-author trailers.** | Resolved by the owner; not open for change. |
| D8 | Legacy code | **Move the Tauri desktop and the VS Code extension into `legacy/` until parity.** | The forbidden-technology check excludes `legacy/` by path. `legacy/` is deleted in the commit that closes the parity gate. |
| D9 | JS plugin loader | **Drop the JS loader; Lua plugins hosted by Neovim, deferred until after Phase 5.** | The current loader runs arbitrary files through `new Function("ctx", code)` with no sandbox (`desktop/src/main.ts:504`). |

## Superseded decisions from 2.x

| 2.x record | Status in 3.0 |
|---|---|
| ADR 0003, Tauri v2 as desktop UI stack | Superseded by Go + Fyne (master prompt Section 2, Section 8). A new ADR must say so before Phase 2. |
| ADR 0004, CodeMirror 6 editor | Superseded by D4 (Neovim). A new ADR must say so before Phase 3. |
| PRD §13 / Checkpoint 1, "Desktop is primary, extension is baseline" | Both frontends move to `legacy/` (D8); the 3.0 Fyne app becomes the only frontend. |
| `docs/DESIGN.md`, cyan `#00D4FF` single accent | Superseded by D2 (Crimson and Amber accents, five themes). |

## Open questions raised by the D2 answer

These need an owner answer or confirmation before the Phase 1 gate. Each has a proposed default.

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

## Tooling record (Phase -1)

- All design tools, Node 22.23.1, Chrome 154 and Claude Code run on the Windows side, not WSL2.
- Impeccable 4.1.0 installed globally with `--no-hooks` so no hook fires in unrelated sessions.
- shadcn, Chrome DevTools (`--isolated --no-usage-statistics --no-performance-crux`) and
  `hig-mcp` added at user scope with a `cmd /c npx` wrapper (required on native Windows).
  `claude mcp list` reported all three Connected. They load only in a new session.
- No tool wrote into the repository; root-anchored ignore rules for their artifacts are in `.gitignore`.
