# PyxForge Desktop — Phase Roadmap

> This roadmap supersedes the original extension-centric phase plan following the editor-foundation decision resolved in [`docs/PRD.md` §13](PRD.md#13-open-questions). The existing Phases 0–12 (VS Code extension + Rust core) are complete and preserved. Phases 13+ drive the Desktop migration.

---

## Completed Phases

| Phase | Scope | Status |
|---|---|---|
| 0–3 | Core backend: JSON-RPC protocol, toolchain orchestration, project scaffolding | ✅ Complete |
| 4–6 | Build profiles, diagnostics pipeline, QEMU launcher + QMP client | ✅ Complete |
| 7–9 | CPU Inspector webview, Hex Viewer webview, AI Assist panel | ✅ Complete |
| 10 | Theme system (contrast, hybrid, mono CSS overlays) | ✅ Complete |
| 11 | P0 closure & hardening sprint | ✅ Complete |
| 12 | QEMU protocol hardening | ✅ Complete |
| 13 | Feasibility spike + UI stack ADR | ✅ Complete |
| 14 | Core/extension decoupling verification | ✅ Complete |
| 15 | Desktop shell: workspace, docking, project explorer (existing panels genuinely ported and running) | ✅ Complete |
| 16 | Integrated terminal (PTY + xterm.js rendering) | ✅ Complete |
| 17 | Debugger UX deepening (pwndbg-inspired views) | ✅ Complete |
| 18 | Emulator manager expansion (QMP snapshot management + console) | ✅ Complete |
| 19 | Plugin SDK (thin runtime loader and extension hooks) | ✅ Complete |

---

## ⛔ CHECKPOINT 1 — Phase 15 Exit Gate (PASSED)

> **See [`docs/architecture/CHECKPOINTS.md`](architecture/CHECKPOINTS.md) for the formal gate decision.**

- **Decision:** **CONTINUE** (Recorded 2026-07-19)
- **Status:** Checkpoint successfully passed. Desktop shell is the primary target. VS Code extension preserved as baseline.

---

## Active & Future Phases (Post-Checkpoint)

| Phase | Scope | Exit Criteria | Non-Goals |
|---|---|---|---|
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
