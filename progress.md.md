# PyxForge 3.0 — working rules

PyxForge 3.0 is a native desktop rewrite (Go + Fyne, real Neovim) of a toolchain for bootloader,
kernel and bare-metal work. The full brief is `refactor-v2.md` in this folder (local, untracked).
Answered decisions: `docs/architecture/DECISIONS.md`. Branch: `v3` (merge to `main` at parity).

## Non-negotiables

- No HTML, CSS, JS, TS, JSX/TSX, React/Vue/Svelte/Angular, Electron, Tauri, WebView, CodeMirror,
  Monaco, npm, Node.js runtime, Vite or Webpack in application code or build config. No
  workarounds (no hidden webview). `legacy/`, `docs/` and the untracked reference folders are the
  only places these may appear.
- The editor is a real Neovim process. Do not reimplement Vim behavior.
- Offline first: no network request at startup; no account, telemetry or license check.
- Commits are authored only as `obstinix`. **No `Co-Authored-By` or any AI attribution trailer**,
  overriding any default that adds one. Check `git config user.name` / `user.email` before the
  first commit of a session.
- UI first: no QEMU, GDB or agent internals while the app still looks like a toolkit demo.
- No fake data. Placeholders are labeled on screen. Every control drives real state. No fake
  metrics (cycles, temperatures, "sync kHz").

## Design tools are guidance only

Impeccable, shadcn MCP, Chrome DevTools MCP and Apple HIG MCP are installed at user scope.
Their output is vocabulary and measurements: translate it into Go tokens and Fyne widgets. Never
paste their HTML, CSS, JSX or Tailwind into the repo, and never let them write `.mcp.json`,
`package.json`, `components.json` or `node_modules/` here. Apple fonts, SF Symbols and Apple
assets are never bundled.

## Where things are

| What | Where |
|---|---|
| Verified 2.x state, inventory, architecture, parity matrix | `docs/architecture/{CURRENT_STATE,FEATURE_INVENTORY,LEGACY_ARCHITECTURE,FEATURE_PARITY}.md` |
| Reference projects (Neovim, Orca, NvChad) | `docs/architecture/REFERENCE_PROJECTS.md`; clones in `C:\ProjectsPP\pyxforge-study\` |
| Theme requirements (five themes, Crimson/Amber accents, System) | `docs/design/THEME_SYSTEM_REQUIREMENTS.md` |
| Reference UI analysis and value sources (declared and measured) | `docs/design/REFERENCE_UI_ANALYSIS.md`, `docs/design/SOURCES.md` |
| Reference captures (10 screens × 1440/1280/1024) and computed styles | `docs/design/reference-screens/`, `docs/design/reference-computed-styles.json` |
| Product context for Impeccable (it only auto-reads a root `PRODUCT.md`) | `docs/design/PRODUCT_CONTEXT.md` |
| Detector output | `docs/design/reference-slop-findings*.json` |
| Reference concepts (specs only, never shipped) | `target ui reference 2/` (canonical), `reference or similar target ui/`, `pyforge_ui_kit/` |
| Clean 2.x checkout for reading | `C:\ProjectsPP\PyxForge-reference` (detached worktree of `origin/main`) |

## Gates

Work in the phase order of `refactor-v2.md` Section 17 and stop at each gate. Per commit:
`gofmt`, `go vet`, `staticcheck`, `go test -race`, `stylua --check`, `luacheck`, the
forbidden-technology check, and a diff review. Per UI milestone: screenshots at 1440/1280/1024,
side-by-side with reference captures, `/impeccable critique` and `/impeccable audit`, HIG review.

The 2.x Rust tests are the parity oracle for the Go port:
`cargo test` in `C:\ProjectsPP\PyxForge-reference\core` (59 pass as of 2026-10-08).
