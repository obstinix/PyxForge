# PyxForge: status and roadmap

As of 2026-11-28, `main` after Phase 7 of the 3.1 owner review.

PyxForge is a native, offline desktop environment for systems work. It has its own editor and
terminal, builds, Git, QEMU and GDB debugging, a project graph and notes, all in one Go program.

## Tech stack

| Area | Used |
|---|---|
| Language | Go 1.27 |
| UI | Fyne 2.8 (native OpenGL; no web technology) |
| Editor | PyxForge's own (`internal/ui/codeedit`); syntax colours from Chroma |
| Terminal | PyxForge's own emulator (`internal/vt`) on ConPTY (Windows) or a Unix pty |
| Language features | Language servers (LSP), optional per language |
| Machine | QEMU (QMP), GDB/MI |
| Graph drawing | PyxForge's own anti-aliased rasteriser (`internal/ui/graphview/raster.go`), drawn on a worker |
| Config | `pyxforge.toml` (BurntSushi/toml), the same format as 2.x |
| Platforms | Windows 11, Linux, macOS (CI on all three) |

## Dropped

| What | Why |
|---|---|
| Embedded Neovim (D13, D16) | Crashes in the terminal (#80) and a heavy runtime dependency; replaced by PyxForge's own editor |
| Neovim's `:terminal` (D17) | Same crash; PyxForge's own emulator keeps scrollback and the theme's colours |
| The 2.x stack: Tauri, web front end, VS Code extension (D19) | No web technology; one native program |
| `rasterx` for the graph (Phase 7) | It cleared a window-sized rasteriser per node: 7 minutes a frame on a large graph |
| AI features, telemetry | Out of scope by rule; the app is offline |

## Done

| Work | Why |
|---|---|
| 3.0 core: editor, terminal, builds with Problems, Git, workspace state, explorer | The base of an IDE without Neovim |
| Systems tools: QEMU, GDB, registers, memory, disassembly, boot-sector map, snapshots | Debugging bare-metal code is the product's reason to exist |
| 3.1 Phases 1–6: component kit, Settings modal, Liquid Glass (opt-in), terminal rework, Graph view, Forge Notes | The identity prompt |
| **Phase 7, stability** (owner review F1, F2, F4, F5, F6): Explorer clicks, a graph that draws in milliseconds, large files, work off the UI thread, crash logs, the broad-folder question, Welcome pins, Explorer drag and drop, Settings > Files & Explorer | The owner's hand test found freezes and crashes; [`docs/audits/phase-07-results.md`](docs/audits/phase-07-results.md) has the numbers |

## Progress

The weights are the owner's estimate of each part's share of 3.1, not a measured number.

| Part | Weight | Status |
|---|---|---|
| Phases 1–6 | 62 % | Done |
| Phase 7: stability | 10 % | Done; awaiting the owner's review |
| Phase 8: workbench | 13 % | Next |
| Phase 9: look (glass by default, fonts, themes) | 8 % | Pending |
| Phase 10: terminal skin, WSL profiles | 4 % | Pending |
| Phase 11: README, screenshots | 3 % | Pending |

## Next

1. **Phase 8, workbench:** ribbon and sidebars in the Settings style, Outline, Bookmarks, saved
   layouts, a machine status pill, the Inspector in panels with a hex view, headers, tabs and
   the command palette.
2. **Phase 9, look:** glass on by default with plates under text (D26), a font picker, new
   themes, procedural ambience, status dots with shapes as well as colours.
3. **Phase 10:** a terminal skin, WSL distribution profiles, a prompt preset.
4. **Phase 11:** README "Why PyxForge" and screenshots.

## Known gaps

- Owner review findings F3 and F7–F13 are open, for Phases 8–11.
- Phases 4–7 have not yet been tried by hand in the packaged app.
- Symbol links through a language server are untested (none installed here).
- With the Crimson accent, a running session's dot is hard to tell from a failed one's (Phase 9).
