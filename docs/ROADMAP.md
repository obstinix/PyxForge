# PyxForge Desktop — Phase Roadmap

> This roadmap supersedes the original extension-centric phase plan following the editor-foundation decision resolved in [`docs/PRD.md` §13](PRD.md#13-open-questions). The existing Phases 0–12 (VS Code extension + Rust core) are complete and preserved. Phases 13+ drive the Desktop migration.

---

## Completed Phases (Extension Era)

| Phase | Scope | Status |
|---|---|---|
| 0–3 | Core backend: JSON-RPC protocol, toolchain orchestration, project scaffolding | ✅ Complete |
| 4–6 | Build profiles, diagnostics pipeline, QEMU launcher + QMP client | ✅ Complete |
| 7–9 | CPU Inspector webview, Hex Viewer webview, AI Assist panel | ✅ Complete |
| 10 | Theme system (contrast, hybrid, mono CSS overlays) | ✅ Complete |
| 11 | P0 closure & hardening sprint | ✅ Complete |
| 12 | QEMU protocol hardening | ✅ Complete |

---

## Active Phases (Desktop Migration)

| Phase | Scope | Exit Criteria | Non-Goals |
|---|---|---|---|
| **13** | Feasibility spike + UI stack ADR | Working Tauri prototype talking to real core; ADR with honesty check (§2) committed; PRD rationale resolved | Full desktop shell; porting all panels |
| **14** | Core/extension decoupling verification | Extension unmodified in behavior and tests passing; core proven frontend-agnostic (no `vscode.*` API deps) | Rewriting core logic |
| **15** | Desktop shell: workspace, docking, project explorer | At least one real extension panel (`inspectorPanel.ts` or `hexPanel.ts`) genuinely ported and running in desktop shell; basic workspace navigation functional | New panel types not in extension |

---

## ⛔ CHECKPOINT — Phase 15 Exit Gate

> **See [`docs/architecture/CHECKPOINTS.md`](architecture/CHECKPOINTS.md) for the formal gate definition.**

At the end of Phase 15 (or ~10-12 weeks from Phase 13 start, whichever comes first), explicitly decide:

- **Continue** into Phase 16+ (desktop becomes primary product)
- **Fall back** to VS Code extension as primary (desktop becomes experimental)

This is a reassessment point, not an automatic kill switch. The VS Code extension is kept alive specifically to make this comparison possible.

---

## Future Phases (Post-Checkpoint, Contingent on Continue Decision)

| Phase | Scope | Exit Criteria | Non-Goals |
|---|---|---|---|
| **16** | Integrated terminal | PTY spawning + rendering working against a real build/QEMU session | Forking Alacritty |
| **17** | Debugger UX deepening | Inspector gains pwndbg-inspired views (dereference chains, flag expansion) on existing GDB bridge | New debugger backend |
| **18** | Emulator manager expansion | QMP client gains snapshot management and monitor console access | Non-QEMU emulators |
| **19** | Plugin SDK (thin) | One real plugin loads, runs, and extends the UI | Marketplace infrastructure |
| **20** | PyxisOS toolchain polish + parity check | Desktop shell used for a real PyxisOS Track B session, unassisted | Android targets, AI Consensus Engine (parking lot) |

---

## Parking Lot (M3+, Contributor-Gated)

These items from the [original vision](vision/PYXFORGE_VISION.md) are intentionally deferred:

- Mobile companion (Android)
- Cloud build infrastructure
- Plugin marketplace
- Cloud sync & collaboration
- AI Consensus Engine
- Educational platform
