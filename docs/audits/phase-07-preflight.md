# Phase 7 preflight audit (2026-11-28, `main` at `7c9d921`)

This audit checks the owner's review findings F1, F2, F4 and F5 against the code before
anything changes.

**Method.** The screen-control tool could not attach to PyxForge: it resolves installed apps
only, and this is a development build. Findings were therefore reproduced through the code:

- Fyne's own event dispatch in its GL driver was read.
- A throwaway harness, not committed, opened the real shell on the largest folder in the
  owner's recent list (`C:\aegis environment`: 13,542 files, 1.7 GB, with Rust `target`
  folders) and timed each step.

Timings are from Windows 11 on an Intel Core Ultra 9 275HX.

## Prompt versus code

| Prompt says | Code says | Resolution |
|---|---|---|
| One branch per phase | The standing rule is `main` only | Asked the owner: **stay on `main`** |
| Glass on by default is "D22" | D22–D25 already exist (Settings, Graph, Terminal, Notes) | New decisions start at **D26** |
| "Fyne has no backdrop blur" | Fyne 2.8 has `canvas.Blur`, measured live at 0.2–3.2 ms a frame (Phase 3 spike) | Phase 9 builds on `canvas.Blur` |
| `STATUS.md` | The status lives in `docs/PROJECT_STATE.md` and `STATUS_AND_ROADMAP.md` | Both are updated |
| F2: "static entries" | The entries the owner saw are this repository's real top-level folders | It is a click bug, not static data; see F2 |

## F2: Explorer rows cannot be selected or opened. Root cause found.

- Each tree row's content is a PyxForge `row` widget. It implements `fyne.SecondaryTappable`
  (for the context menu) but not `fyne.Tappable` (`internal/ui/explorer/explorer.go:236`).
- Fyne's GL driver gives a click to the topmost object that takes *any* mouse event, right-click
  only included (`fyne.io/fyne/v2/internal/driver/glfw/window.go:510`). It then calls `Tapped`
  only if that object is `Tappable` (`window.go:649`).
- So in the real app every left-click lands on `row` and is dropped. The tree underneath never
  sees it, and the Explorer looks static.
- The tests pass because `test.Tap` taps Fyne's tree node directly and bypasses the hit-test.
- The same applies to `go run` and the packaged build. It is not a packaging difference.
- **Fix:** rows handle the primary click themselves: one click selects, a double click or
  Enter opens. A regression test taps through the canvas (`test.TapCanvas`), as the real
  driver does.

## F5: the Graph freezes or crashes the app. Root cause found.

- `graphview.View.draw` fills each node through a `rasterx` scanner whose rasterizer spans the
  **whole window**. Each `Draw`/`Clear` therefore costs work proportional to the window's
  pixels, not the node's.
- **Measured** on the Aegis graph (3,102 nodes, 459 links) at 2880×1800 device pixels, one
  draw took **7 min 1.5 s** with glow on hubs and **2 min 27 s** with no glow.
- The layout goroutine asks for up to 60 redraws a second, each on the UI thread, so the
  window never responds again.
- Phase 5's budget test timed layout ticks only (1.2 ms), never a draw, so its gate missed
  this.
- The raster is window-sized, not graph-sized. The prompt's 20k × 20k guess is not the cause.
- **Fix (7.4):**
  - Draw all nodes of one colour, and all links, as one path each: about a dozen `Draw`s a
    frame, not thousands.
  - Clip to the viewport.
  - Rasterise off the UI thread, with stale frames dropped.
  - Cap the nodes, collapsing clusters by folder.
  - Add a draw-time budget to the tests.

## F4: large files crash the app. Root cause found.

- `textbuf.Load` reads the whole file, then keeps it as `[][]rune`: 4 bytes per character plus
  a 24-byte header per line (`internal/textbuf/buffer.go:54`).
- The first colouring runs **synchronously on the UI thread over the whole text**, whatever
  its size (`internal/ui/codeedit/editor.go:280`). Later colourings use a goroutine only
  above 3,000 lines.
- **Measured:**
  - A 100 MB log took 3.0 s on the UI thread, and the heap grew by about 575 MB (323 → 898 MB).
  - A 2 MB single-line file took 0.47 s.
  - A larger file, or several open at once, runs out of memory.
- An 18.7 MB binary (`fuzzer.lib`) opened in 13 ms, because binaries go to the read-only
  binary view. That path is fine.
- **Fix (7.3):**
  - Large-file thresholds: above 2 MB, no Chroma, LSP or diagnostics; above 50 MB, read-only
    and paged; above 512 MB, hex only.
  - A piece-table buffer.
  - Colouring of the visible range only, on a worker.
  - Long lines truncated in the display.

## F1: freeze or crash on launch, "opens a whole drive". Not reproduced; the causes are bounded.

**What the code does.** With no argument, the shell opens with no folder (the Welcome tab);
it never falls back to the current folder.

**What the installer does.** Its shortcuts pass `Documents\PyxForge Projects`
(`packaging/windows/pyxforge.iss:80`), and `workspace.Discover` keeps that folder as the
workspace. No `pyxforge.toml` or `.git` sits above it on this machine.

**So no code path opens a drive on its own.** What exists instead:

- **No guard** when a drive root or the home folder is opened by hand (Open Folder, a
  command-line argument). The owner's recent list includes several large folders.
- **Walks on the UI thread.** Quick Open walks up to 20,000 files on the UI thread
  (`shell.go:900`); measured at 36 ms on Aegis, far longer on a drive.
- **A self-sustaining freeze.** Opening the Graph on a large folder freezes the app by F5,
  which reads as "the app freezes". After a forced quit the next launch is normal, unless the
  owner reopens the same folder.

**Fix (7.1):**
- A Welcome screen with recent workspaces.
- A confirmation that defaults to Cancel for a drive root or the home folder.
- "Reopen last workspace" in Files & Explorer.
- No walk on the UI thread.
- Crash logs (7.5), so the next freeze leaves a stack.

## Walks and where they run

| Walk | Where it runs | Phase 7 |
|---|---|---|
| Explorer listing (`explorer.go:208`) | UI thread, one folder per expand | Moves to a worker with a loading row |
| Quick Open (`explorer.Files`) | **UI thread**, up to 20,000 files | Moves to a worker |
| Graph build (`graph.Build`) | Worker | Kept; cancellable |
| Notes index (`notes.Build`) | Worker | Kept |
| Git status | Worker (`gitPanel.refresh`) | Kept |
| Colouring a file | **UI thread** the first time | Moves to a worker, visible range only |
| Graph draw | **UI thread**, O(nodes × window pixels) | Rewritten (F5) |
