# ADR 0006: Embedded Neovim as the editor

- **Status:** Accepted (resolves decision D4; supersedes ADR 0004)
- **Date:** 2026-10-09
- **Evidence:** `tools/nvimspike`, `internal/neovim` tests

## Context

Decision D4 chose real Neovim as PyxForge's editor, rendered natively, on condition that a spike
showed grid rendering, cursor, input, resize and a 2,000-line file at smooth scroll. The
fallback was Neovim in a terminal pane. ADR 0004 (CodeMirror in a webview) no longer applies.

## Options

1. **Embedded Neovim, drawn by PyxForge.** Start `nvim --embed`, speak msgpack-RPC on its stdio,
   attach as a UI with `ext_linegrid`, and draw the grid in a Fyne widget.
2. **Neovim in a terminal pane.** Run Neovim inside a PTY and a terminal emulator widget.
   Needs a full VT parser, keeps Neovim's own colours and cursor, and loses structured
   events (modified state, diagnostics) unless they are scraped.
3. **A Go reimplementation of Vim editing.** Rejected by the brief: never reimplement Vim.

## Decision

Option 1. PyxForge runs Neovim as a child process and is its UI:

- `internal/neovim` owns the process and the RPC client (`github.com/neovim/go-client`, pinned),
  applies `redraw` batches to a grid model, and reports buffer events (enter, modified, write,
  diagnostics) through autocommands that call back over RPC.
- `internal/ui/editor` draws the grid as runs of text and background, maps Fyne input to
  Neovim key notation, sends mouse and wheel events, and asks Neovim to resize when the view
  does.
- Neovim starts with `--clean -n`: never the user's configuration. PyxForge's own Lua
  configuration will load with an explicit `-u` and isolated `XDG_*` folders.
- Shell chords (Ctrl+Shift, `KEYMAP.md`) stay with the shell; every other key goes to Neovim.

## Evidence

`go run ./tools/nvimspike` drives a real window through the same input paths a user does.
Windows 11, Neovim 0.12.5, three runs on 2026-10-09:

| Check | Result |
|---|---|
| First redraw after launch | 96–104 ms, 152×43 grid |
| Open a 2,000-line file | 15–16 ms to first screen |
| Page through it with Ctrl+F (51 pages) | 477–549 ms total; key to redraw 4.1–5.4 ms on average; one outlier of 69–123 ms per run |
| Build one frame from the grid | 0.27–0.31 ms on average, under 1.6 ms worst |
| Type through the view, including `<` | passes |
| Modified flag, save to disk, diagnostics with positions | pass, through autocommand events |
| `:terminal` job (ConPTY) | output drawn in the grid |
| Shell chord kept from Neovim; Ctrl+W sent to it | passes |
| Resize the window | grid follows the view's cell size |
| Close | the Neovim process exits |

`internal/neovim` tests repeat the editing, saving, diagnostics, terminal, resize and exit
checks headlessly against a real Neovim. CI installs Ubuntu's Neovim package for the Linux job
so they run there as well (they skip where Neovim is absent); macOS is compile-checked (D5).

## Consequences and open work

- Neovim is a runtime dependency of the desktop app's editor; `pyxforge doctor` reports it.
- Frame building is cheap; the worst-case key outlier needs profiling before release (startup
  cost of highlighting a new region is the first suspect).
- Not yet built: Neovim's clipboard provider wired to the system clipboard, IME composition,
  `ext_cmdline`/`ext_popupmenu` drawn as PyxForge UI, multiple grids, and synchronising
  Neovim's colours with the PyxForge theme (parity N12).
- On Windows, AltGr arrives as Ctrl+Alt plus a character; the view sends only the character, so
  Ctrl+Alt mappings in Neovim do not fire on Windows.
