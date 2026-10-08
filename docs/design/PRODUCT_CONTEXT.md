# Product

<!-- impeccable:product-schema 1 -->

<!--
Written 2026-10-08 in the shape of Impeccable's `init` record (reference/init.md, schema 1).
Facts come from the owner's brief (`refactor-v2.md`, which says "answer from this prompt"),
the answered decisions in `docs/architecture/DECISIONS.md`, and the 2.x PRD (`docs/PRD.md`).
No separate interview was run. Two tool limits apply:
- Impeccable reads only a `PRODUCT.md` in the project root, so it will not load this file
  automatically. The brief names this path; see the Phase 0 gate report.
- Impeccable's platform values are web, ios, android, adaptive. None describes a native
  desktop app, so the value below is outside its schema on purpose.
-->

## Platform

desktop

Native desktop application built with Go and Fyne. Windows and Linux are released targets;
macOS is compile-checked only (D5). It is not a web app and contains no web runtime.

## Users

Developers writing bootloaders, kernels, firmware and other bare-metal code. The first user is
the owner, working through a from-scratch OS curriculum alongside the sibling project PyxisOS;
the next audience is osdev hobbyists and students following similar paths (PRD §5). They work in
long sessions, mostly at the keyboard, often offline, moving between source, a build, a running
QEMU guest and a debugger.

## Product Purpose

One native environment for the systems-programming loop: edit, build, boot in QEMU, debug in
GDB, inspect the binary, commit. Success means the whole loop works with the network off and no
account, and the time from "start debugging" to "breakpoint hit, registers visible" is shorter
than doing it by hand with shell scripts and GDB init files (PRD §2, §8).

## Positioning

No other tool packages QEMU, GDB with real-mode, protected-mode and long-mode awareness, and
bare-metal build profiles as one workflow (PRD §4). PyxForge does it with a real Neovim process as
the editor, structured instrumentation (QMP, GDB/MI, ELF) rather than scraped terminal output,
and AI agents that work in isolated git worktrees and can only propose changes.

## Operating Context

- Development happens on Windows with WSL2 as the Linux path.
- Toolchains: `nasm`, `gcc`/`g++`/`clang`, `cargo`/`rustc`, `arm-none-eabi-gcc`, `ld`,
  `link.exe`; emulation in QEMU (`qemu-system-x86_64`, `-i386`, `-arm`); debugging with `gdb` or
  `gdb-multiarch`.
- Each project has a `pyxforge.toml` (build profiles, `[qemu]`, `[gdb]`), which must stay
  compatible across versions.
- Users are Vim users or expect Vim motions; everything must be reachable from the keyboard.

## Capabilities and Constraints

- Core loop works fully offline. No network request at startup unless the user enabled a
  networked feature. No account, login, telemetry or online license check.
- Real Neovim (UI protocol, Lua config, Tree-sitter, local LSP) is the editor.
- Build profiles and presets, diagnostics from GNU, Cargo JSON, rustc text, MSVC and GNU ld,
  QEMU with QMP and snapshots, GDB/MI with registers, memory, stack and disassembly, hex and ELF
  inspection, a 512-byte boot sector map and `0xAA55` signature check, native PTY terminals,
  local Git and worktrees, an optional agent layer.
- Forbidden in the application: HTML, CSS, JavaScript, TypeScript, any web framework, Electron,
  Tauri, WebView, CodeMirror, Monaco, Node.js at runtime.
- Licensed Apache-2.0. No GPL code may enter the repository.
- Undecided: Ink & Glass polarity, blur, Signal Yellow, Monochrome polarity, System mapping,
  accent derivation (Q1–Q6 in `DECISIONS.md`); ownership of the logo concept.

## Brand Commitments

- Name: PyxForge.
- Five user-selectable themes (Smoked Kraft, Ink & Paper, Ink & Glass, Verdigris Forge,
  Monochrome) with independent Crimson and Amber accents and a System mode; the identity is the
  design system, components, typography and interaction model, not one palette (owner, D2).
- No gradients in the workspace; glass only on overlays; no glow or lift on hover; no idle
  animation; no emoji; no decorative or fake metrics.
- At most one brand motif: the `{ }` bracket mark, or the bracket-sun logo concept if the owner
  confirms it.
- Voice comes from the domain (registers, sectors, QEMU, GDB), not from SaaS marketing.

## Evidence on Hand

- The 2.x codebase and its 59 passing Rust tests and 10 extension tests.
- Reference UI concepts under `target ui reference 2/`, `reference or similar target ui/` and
  `pyforge_ui_kit/`, analyzed in `docs/design/REFERENCE_UI_ANALYSIS.md`.
- `docs/cross-project/pyxisos-integration.md` describes the PyxisOS workflow.
- There are no user counts, testimonials, benchmarks or performance claims. None may be invented.

## Product Principles

1. Every control drives real state; a placeholder says so on screen.
2. The core loop never depends on the network.
3. Neovim owns text; PyxForge owns the workspace around it and the machine-level instruments.
4. Structured data over scraped text: QMP, GDB/MI, porcelain git, JSON diagnostics.
5. Density serves the work; decoration has to earn its place.

## Accessibility & Inclusion

Text contrast at least 4.5:1 (3:1 for large text and UI glyphs) in every theme and accent
combination; visible focus on every interactive element; full keyboard operation; reduced-motion
respected; no functional text below 11 px.
