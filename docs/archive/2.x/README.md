# PyxForge 2.x documents (archived)

These documents describe PyxForge 2.x: a Rust core driven over JSON on stdio, a VS Code
extension, and a Tauri desktop shell with CodeMirror and xterm.js. PyxForge 3.0 replaced that
stack with Go, Fyne and an embedded Neovim (ADR 0005 and 0006 in `docs/architecture/`). They are
kept as the record of what 2.x planned, built and checked, and as the reference the 3.0 port was
measured against; they do not describe the current program.

| Document | What it was |
|---|---|
| `PRD.md` | The 2.x implementation PRD |
| `ARCHITECTURE_V2.md` | The planned Tauri desktop architecture |
| `CHECKPOINTS.md` | 2.x architecture gate decisions |
| `AUDIT_REPORT.md` | The 2026-07-19 2.x audit |
| `DECOUPLING_VERIFICATION.md` | Core/extension decoupling check |
| `DESIGN.md` | The 2.x design specification (cyan accent, CSS) |
| `REFERENCE_ANALYSIS.md` | Study of reference IDEs for the Tauri shell |
| `CURRENT_STATE-2026-10-08.md` | What 2.x actually did, checked against its code before the rewrite |
| `pyxisos-integration-study.md` | The 2.x plan for building PyxisOS with PyxForge (VS Code Problems panel, presets) |

The 2.x code itself is in `legacy/` until 3.0 reaches parity (`docs/architecture/FEATURE_PARITY.md`);
`docs/architecture/LEGACY_ARCHITECTURE.md` explains how it was built.
