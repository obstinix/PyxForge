# PyxForge — Implementation PRD

**Status:** Draft v0.1 · **Scope:** v1, solo build, open-source-ready
**Source:** Reframes the original PyxForge vision doc (now committed as [`docs/vision/PYXFORGE_VISION.md`](vision/PYXFORGE_VISION.md)) into a buildable plan

---

## TL;DR

The original doc describes something with the combined scope of **CLion + PlatformIO + Android Studio + a cloud IDE** — a full custom editor, universal toolchain manager, 12+ cross-compilation targets, AI auto-fix, five emulators, a mobile app, cloud sync, and a plugin marketplace. Built solo, that's a multi-year effort that risks shipping nothing.

This PRD keeps the long-term vision intact (see Appendix A) but defines a **v1 you can actually build alone**: not a new IDE, but an opinionated VS Code extension that turns your current manual PyxisOS Track B workflow — NASM build → QEMU boot → GDB remote attach — into a one-click, repeatable pipeline. Everything else in the original doc becomes a later phase, gated on either your own growing needs or community contributions after open-sourcing.

**Core bet: don't build an editor — build the glue VS Code doesn't have.**

---

## 1. Problem Statement

From-scratch OS developers — you, right now, and the wider hobbyist osdev community you'd eventually open-source this for — assemble their toolchain by hand: separate NASM/cross-GCC/Rust/QEMU/GDB installs, hand-written Makefiles, manually-flagged QEMU debug sessions (`-s -S`), and manually reconfigured GDB sessions (e.g. `set architecture i8086` for real mode, switched again once you move to protected mode). None of it is remembered between sessions or shared between learners.

This tax gets paid again at every new curriculum phase — you're about to pay it for GDB+QEMU remote debugging right now, and you'll pay a version of it again for protected mode, paging, and multitasking later in Track B. Existing tools don't solve this: raw CLI is what you're doing today; VS Code + generic extensions gets you an editor but no OS-dev-aware debug workflow; CLion is heavyweight, commercial, and not bootloader/real-mode aware; PlatformIO is embedded-board-focused, not from-scratch-OS-focused.

## 2. Goals (v1)

1. Collapse "edit → assemble/compile → boot in QEMU → attach GDB → inspect state" from 5+ manual CLI steps to one keybind.
2. Make GDB+QEMU remote debugging — your flagged critical skill gap for the next lesson — a pre-built, guided workflow instead of something learned from raw documentation.
3. Ship something you actually use for the rest of Track B within weeks, not a comprehensive platform in years.
4. Structure the repo so it can be open-sourced cleanly the moment v1 is useful, without a rewrite — docs, license, and contribution model in place from day one.

## 3. Non-Goals (v1)

| Not building (yet) | Why |
|---|---|
| Custom editor (Monaco shell, multi-cursor, hex/binary viewers, Vim/Emacs modes) | VS Code already does this via existing extensions. Rebuilding it is itself a multi-year project with no differentiated payoff. |
| 12-toolchain / 12-target universal support | Solo bandwidth. Start with the ~5 tools and 1–2 targets you actually use (NASM, your Rust cross-toolchain, QEMU, GDB, Make); widen when you or a contributor needs the next one. |
| AI Build Doctor "auto-fix" | Auto-patching linker scripts/ABI mismatches is an open research problem and risky if wrong. v1 AI explains — it doesn't apply. |
| Android companion app | A second codebase, a sync protocol, and a remote-build security model — a project in its own right. Revisit only if PyxForge gets real community traction. |
| Cloud sync / multi-user backend / plugin marketplace | Needs auth, hosting, and a review pipeline before there's a user base to justify it. Local-first for v1. |
| Emulator Center beyond QEMU (Bochs, DOSBox, Android/RPi emulators) | Adds breadth, not depth, for the actual v1 persona — from-scratch x86 OS devs live in QEMU. |

## 4. Prior Art / Build vs. Buy

| Tool | Strength | Gap for this use case |
|---|---|---|
| VS Code + extensions (Cortex-Debug, Native Debug, rust-analyzer) | Editor, git, symbol nav, debug UI shell — all free | No OS-dev-specific build/boot/debug workflow, no real-mode-aware GDB setup |
| PlatformIO | Strong embedded board + toolchain management | Built for embedded product boards, not from-scratch bootloaders/kernels |
| CLion | Excellent C/C++/CMake + bundled GDB UI | Heavyweight, commercial, not OS-dev-specific |
| osdev.org + raw QEMU/GDB/Makefiles | What actually works, what you're doing now | Zero repeatability, zero sharing between learners |

**The gap:** nobody packages "QEMU + GDB + real/protected-mode-aware debugging + from-scratch OS build profiles" as one guided workflow. That's the wedge — and it's narrow enough to build solo.

## 5. Target Users

- **Primary (v1):** you, mid-curriculum on PyxisOS Track B.
- **Secondary (v1.1+):** other osdev.org-style hobbyists/students following similar bare-metal curricula.
- **Explicitly not v1:** embedded product teams, enterprise firmware orgs, mobile developers — the audience for the original doc's "Revolutionary Feature" (Android companion) section, deferred indefinitely.

## 6. User Stories (P0)

- As a from-scratch OS developer, I want to build my current project with one command, so I stop retyping assembler/linker invocations every iteration.
- As a from-scratch OS developer, I want QEMU to launch paused with the GDB stub open (`-s -S`) and have my editor auto-attach GDB with the correct architecture already set, so I don't re-derive debug flags for every phase.
- As a from-scratch OS developer, I want registers, flags, and memory/stack visible next to my source, so I can correlate execution with code without a separate GDB TUI.
- As a learner, I want reusable build profiles (bootloader / kernel / debug / release) so each new curriculum phase doesn't mean rewriting the Makefile from scratch.
- As a soon-to-be maintainer, I want a clean-clone setup (README, CONTRIBUTING, license) so opening the repo doesn't block early contributors.

## 7. Requirements

### P0 — Must-have for v1
- [x] One-click build task (NASM now; cross-GCC/Rust as Track B progresses), errors mapped to source lines (Done: implemented via core command dispatcher and DiagnosticCollection parsing inside VS Code)
- [x] One-click QEMU launch, pre-paused with GDB stub enabled (Done: implemented via QEMU backend launcher with `-s -S` flags)
- [x] Pre-configured GDB remote attach with architecture auto-set (i8086 for real mode; switch as your curriculum moves to protected/long mode) (Done: GDB adapter tracker automatically attaches using configuration GDB triple settings)
- [x] Register / flags / stack / memory inspector panel alongside source (Done: Custom webview panels stream inspector information via QEMU/GDB protocol bridge)
- [x] Build profile presets: bootloader / kernel / debug / release (Done: Select preset command overlays templates directly in local pyxforge.toml configuration)
- [x] Project scaffolding generator (Makefile with `run`/`debug`/`clean` targets, `.vscode/launch.json`, `.vscode/tasks.json`) (Done: Scaffold templates generated on `init` request via core binary)

### P1 — Fast-follow, not blocking v1
- [x] AI assist: explain-this-error, explain-this-asm, explain-this-register-state (read-only, suggests fixes, never auto-applies) (Done: Deeply integrated with vscode.lm chat participant APIs)
- [x] Binary/hex viewer for the compiled boot sector (nice symmetry with the 0xAA55 signature work you already did) (Done: Hex viewer custom panel reads local images and renders interactive hex grids)
- [x] Protected-mode / long-mode build + debug profiles for later Track B phases (Done: Presets and tool configs support x86, i386, and x86_64 architectures)
- [x] Rust cross-toolchain support (per your CONTRIBUTING.md's stated kernel language) (Done: Rust no_std scaffolding and cargo target compiling are fully functional)


### P2 — Future / design-for-later, not built now
- [ ] Additional targets (ARM Cortex-M, RISC-V, STM32, ESP32) — only if a contributor needs one
- [ ] Simple local extension points (not a marketplace) — leave the hooks, skip the infrastructure
- [ ] Mobile companion, cloud sync — parking lot

## 8. Success Metrics

**Leading (weeks):** you use it instead of raw CLI for the rest of Track B; time from "start debug session" to "breakpoint hit, registers visible" drops versus your current manual baseline.
**Lagging (months):** first external star/fork/issue once open-sourced; first external PR merged; PyxisOS's own `docs/architecture/` references PyxForge as the recommended dev setup.

## 9. Proposed v1 Architecture

```
                VS Code  (editor / git / symbol-nav — not rebuilt)
                        │
                 PyxForge Extension
        ┌───────────────┼────────────────┐
        │                │                │
  Build Profile      QEMU ⇄ GDB       AI Assist (P1)
    Manager             Bridge          explain-only
        │                │
 nasm / cross-gcc    qemu -s -S
   / rustc / make         │
                    gdb (arch-aware)
```

## 10. Repo & Workflow Conventions

Worth keeping this consistent with your PyxisOS setup rather than inventing new conventions:

- **Repo:** either a new `github.com/obstinix/PyxForge`, or a `tools/pyxforge/` folder inside the PyxisOS repo for M0, splitting out once it's independently useful
- **Branches:** same pattern as Track B — e.g. `pyxforge/<description>`
- **Commits:** Conventional Commits (`feat:`, `fix:`, `docs:`) — matches what you already use
- **Docs:** `docs/architecture/` for design notes, same as PyxisOS

*(I couldn't pull your current repo structure to confirm exact naming — worth double-checking CONTRIBUTING.md before locking this in.)*

## 11. Roadmap

| Milestone | Timeframe (solo, part-time) | Scope | Exit criteria |
|---|---|---|---|
| **M0 — Dogfood** | 4–8 weeks | P0 list, scoped to exactly your current bootloader/real-mode workflow | You use it for the rest of Phase 2 instead of raw CLI |
| **M1 — Public Alpha** | +2–3 months | Generalize P0 beyond your exact project; README, CONTRIBUTING, license; open the repo | A stranger can clone and use it with zero context from you |
| **M2 — Community Growth** | Open-ended, contributor-paced | P1 items; protected/long-mode profiles as Track B gets there | First external contribution merged |
| **M3+ — Platform Bets** | Only if community materializes | P2 items — more targets, extension hooks, eventually (maybe) the mobile/cloud features from the original doc | Sustained multi-contributor activity |

## 12. Key Risks

- **Scope creep back toward the original doc.** The fastest way to lose momentum is to let M0 grow toward "universal toolchain manager." Every addition needs a corresponding cut or an explicit timeline extension.
- **PyxForge competing with PyxisOS for your time.** You've been deliberate about depth over speed on the kernel/hypervisor curriculum itself — building a whole IDE is a second big engineering project that can quietly become the more tractable, more fun problem that displaces the harder one. Keep M0 as close to *configuration* (`tasks.json`/`launch.json` + a thin script) as possible rather than a "real" extension with lots of custom code, so it costs days, not months, before you're back to Phase 2 content.
- **Build-vs-buy reversal risk.** If VS Code's extension API turns out to be a hard limiter for the register/memory visualization you want, treat that as an explicit decision point at the end of M0, not a mid-M0 detour.

## 13. Open Questions

- ~~Editor foundation: VS Code extension (recommended) vs. a standalone shell later — revisit only if extension APIs hard-block M1's P1 features. *(resolve by end of M0)*~~ → **Resolved**, see below.
- License for open-sourcing (MIT / Apache-2.0 / GPL) — worth matching whatever PyxisOS itself uses if that's already set. *(resolve before M1)*
- AI assist backend for P1: call an LLM API vs. something local/rules-based first — cost, latency, and "does this need network access" tradeoffs. *(resolve during M1)*

### Resolved 2026-07-19: Editor foundation reopened

Decision: move from VS Code-extension-only to a standalone Desktop shell as
the primary product. The VS Code extension becomes a transitional
compatibility frontend (may eventually be archived) while PyxForge Desktop
becomes the primary architecture.

Reason: This is a deliberate strategic/product-direction decision, not a
response to a specific VS Code API blocker. The v1 PRD's choice to build as
a VS Code extension was the right pragmatic bet for shipping something
usable solo — it provided an editor, extension APIs, and debug UI for free,
letting effort go into the Rust backend, build system, diagnostics, and
QEMU/GDB integration instead of an IDE shell.

That backend is now substantially complete (Phases 0-12). Continuing to
treat the extension as the primary product would keep moving PyxForge
further from its original intended destination rather than closer to it —
native editor control, deep terminal integration, tightly coupled QEMU
lifecycle management, and custom debugging/memory-visualization UI are all
easier to design and evolve when PyxForge owns its own desktop shell.

Acknowledged tradeoff: this consciously reopens the risk named in §12 —
scope creep toward the original vision doc, and PyxForge competing with
PyxisOS for time. That risk is being knowingly accepted, not overlooked.

Mitigation: checkpoint at the end of Phase 15 (target ~10-12 weeks from
Phase 13 start, whichever comes first) — the first point where the Desktop
shell is usable enough to honestly compare against the working VS Code
extension baseline (kept alive specifically to make this comparison
possible). At that checkpoint, explicitly decide: continue into Phase 16+,
or fall back to the extension as primary. This is a reassessment point, not
an automatic kill switch — if progress is real but slower than estimated,
extend consciously rather than let the deadline pass silently.

The existing Rust core, protocol, build system, diagnostics, and debugger
integration are preserved and reused, not rewritten. The change is at the
frontend/architecture level, executed via the Phase 13+ roadmap, gated by
human review at each phase boundary.

## Appendix A — Original doc phases mapped to this roadmap

*(Nothing from the original vision is dropped, just resequenced. Note the original doc has two separate, overlapping numbering systems — "Major Features" Phase 1–10 and "Future Roadmap" Phase I–VI — that don't line up 1:1; worth consolidating into one sequence if you keep the original doc as a reference for future contributors.)*

| Original section | Where it lands here |
|---|---|
| Phase 1 — Intelligent Code Editor | Non-goal (v1) — delegated to VS Code |
| Phase 2 — Universal Toolchain Manager | P0/P1, narrowed to ~5 tools |
| Phase 3 — Universal Cross Compilation | P0 (1–2 targets) → P2 (rest, contributor-driven) |
| Phase 4 — Smart Build Profiles | P0 |
| Phase 5 — AI Build Doctor | P1 (explain-only); auto-fix not planned |
| Phase 6 — GPU-Accelerated Terminal | Non-goal — VS Code's terminal (or your existing Alacritty setup) covers this |
| Phase 7 — Visual Toolchain Explorer | P2 |
| Phase 8 — Binary Explorer | P1 (boot-sector hex view only) → P2 (full ELF/PE/Mach-O/WASM) |
| Phase 9 — Emulator Center | P0 (QEMU only) → P2 (Bochs/DOSBox/others) |
| Phase 10 — Integrated Debugging | P0 (GDB) → P1 (OpenOCD/JTAG once embedded targets exist) |
| Mobile Cross Compilation (Android) | Parking lot — M3+, contributor-gated |
| Plugin Marketplace / Cloud Sync | Parking lot — M3+, contributor-gated |
