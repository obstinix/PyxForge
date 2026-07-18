# PyxForge — Architecture Checkpoints

This document tracks formal decision gates in the PyxForge Desktop migration. Each checkpoint is a mandatory human-review point — work past it does not proceed without an explicit recorded decision.

---

## Checkpoint 1: Phase 15 Exit Gate

**Trigger:** End of Phase 15 implementation, OR ~10-12 weeks from Phase 13 start (2026-07-19), whichever comes first.

**Estimated deadline:** ~2026-10-01 (adjustable — see below).

### Decision to record

- [ ] **CONTINUE** into Phase 16+ — Desktop shell becomes the primary product.
- [ ] **FALL BACK** to VS Code extension as primary — Desktop becomes experimental/archived.
- [ ] **EXTEND** — progress is real but slower than estimated; consciously extend the deadline to a new specific date: `____________`

### Evaluation criteria

| Criterion | Pass condition |
|---|---|
| Real panel port | At least one extension panel (`inspectorPanel.ts` or `hexPanel.ts`) genuinely ported and running in the desktop shell — not a visual lookalike rewrite |
| Core IPC | Desktop shell communicates with `pyxforge-core` over the same JSON-RPC protocol, executing real build/QEMU commands |
| Extension baseline | VS Code extension still builds, passes all tests (currently 10), and is usable for a real PyxisOS workflow |
| Workspace navigation | Desktop shell has functional project explorer, panel docking, and file navigation |
| Honest comparison | A dated written comparison of the Desktop shell vs. extension baseline, documenting what works better, what works worse, and what's missing |

### Decision record

> *This section is filled in at checkpoint time. Do not pre-fill.*

**Date:** _______________  
**Decision:** _______________  
**Rationale:** _______________  
**Next action:** _______________  
**Recorded by:** _______________

---

## Checkpoint history

| Date | Checkpoint | Decision | Link |
|---|---|---|---|
| *(none yet)* | | | |

---

*This document is referenced by [`docs/ROADMAP.md`](../ROADMAP.md) and [`docs/PRD.md`](../PRD.md) §13 resolved rationale.*
