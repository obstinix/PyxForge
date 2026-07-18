# ADR 0003: Desktop UI Stack Selection

## Context

We are migrating **PyxForge** from a VS Code extension to a standalone desktop IDE (**PyxForge Desktop**) to serve as the primary development platform for **PyxisOS**. We must select a cross-platform desktop UI library that balances performance, memory usage, native Rust integration, and developers' productivity.

Crucially, we must preserve and reuse the existing, working frontend codebases:
1. **CPU Register Inspector** (custom HTML/CSS/TypeScript view)
2. **Hex Viewer** (custom HTML/CSS/TypeScript view)
3. **Theme Configurations** (dynamic stylesheet overlays)

Rebuilding these rich panels from scratch in a native Rust UI framework would constitute severe scope creep and violate the core directive of preserving existing work.

## Options Considered

1. **Tauri v2** (HTML/CSS/JS frontend rendered via platform Webview; Rust backend orchestrator)
2. **egui** (Immediate-mode native Rust GUI)
3. **Iced** (Retained-mode native Rust GUI inspired by Elm)
4. **Slint** (Declarative UI markup compiled to native Rust code)
5. **Electron** (Chromium + Node.js frontend)

---

## Comparison Matrix

| Criteria | Tauri v2 | egui | Iced | Slint | Electron |
|---|---|---|---|---|---|
| **Portability** | High (WebView2, WebKitGTK, WKWebView) | High (Software/GL) | High (WGPU) | High (Software/GL) | High (Embedded Chromium) |
| **Memory Footprint** | Low/Medium (~30-60 MB) | Extremely Low (<20 MB) | Extremely Low (<20 MB) | Extremely Low (<20 MB) | Very High (150-300+ MB) |
| **Rust Integration** | Native (Tauri Command bridge) | 100% Rust | 100% Rust | 100% Rust | Poor (Requires Neon/N-API bridges) |
| **Existing UI Reuse** | **100% (Directly usable)** | **0% (Requires total rewrite)** | **0% (Requires total rewrite)** | **0% (Requires total rewrite)** | **100% (Directly usable)** |
| **Plugin Capability** | High (Standard JS/CSS or Rust plugins) | Low/Custom | Low/Custom | Low/Custom | High |
| **License** | MIT / Apache-2.0 | MIT / Apache-2.0 | Apache-2.0 | GPLv3 / Commercial | MIT |

---

## Decision

We select **Tauri v2** as the desktop UI stack for **PyxForge Desktop**.

### Rationale

1. **Maximum Code Reuse (Preservation of Work):** Tauri allows us to import the existing webview layouts (CPU Inspector and Hex Viewer) and styling engines developed during the extension phase with minimal structural modifications. This allows us to deliver a complete, highly-refined desktop IDE environment without throwing away months of working frontend code.
2. **First-Class Rust Integration:** The backend of a Tauri application is written in native Rust. This enables seamless, zero-copy interaction with the existing `pyxforge-core` backend, GDB controllers, and QMP client modules.
3. **Low Overhead:** Unlike Electron, which bundles a heavy Chromium binary, Tauri delegates window rendering to the host operating system's native webview engine. This yields small installer sizes and minimal run-time memory consumption, satisfying low-level developer environments.
4. **Permissive Licensing:** Both Tauri and PyxForge are licensed under Apache-2.0, avoiding any license contamination with copyleft system binaries (e.g. QEMU).

---

## Honesty Check — Phase 13 Spike Reuse Assessment

*Added 2026-07-19 per §2 of the Phase 13 execution prompt.*

The Phase 13 feasibility spike built a working desktop shell with a simulated CPU register inspector panel. However, this section documents what was **actually reused** versus **rewritten** from the existing VS Code extension codebase.

### Files assessed

| Extension source file | Lines | Reused in desktop spike? | Notes |
|---|---|---|---|
| `extension/src/inspectorPanel.ts` | 598 | **No — rewritten** | The desktop `main.ts` implements a new, simpler register panel from scratch. The extension's `PyxForgeInspectorPanel` class (including its `Register`/`InspectorState` interfaces, `vscode.WebviewPanel` lifecycle, theme-aware HTML generation, memory dump rendering, register-diff highlighting, and custom-address memory inspection) was not ported. |
| `extension/src/hexPanel.ts` | 334 | **No — not present** | The desktop spike does not include a hex viewer at all. The extension's `PyxForgeHexPanel` (with `HexDumpLine`/`HexDumpData` interfaces, boot-sector annotation, and theme-aware hex grid rendering) remains only in the extension. |
| `extension/themes/contrast.css` | — | **No — not ported** | The desktop spike uses a new standalone CSS file. |
| `extension/themes/hybrid.css` | — | **No — not ported** | Same as above. |
| `extension/themes/mono.css` | — | **No — not ported** | Same as above. |
| `extension/src/presets.ts` | 281 | **No — rewritten** | The desktop `main.ts` calls a `listProfiles` RPC but implements its own UI rendering. The extension's preset registry logic was not ported. |

### What was actually reused

- **The Rust core backend** (`core/`) is genuinely reused — the desktop's `call_core` Tauri command spawns the same `pyxforge-core` binary and speaks the same JSON-RPC protocol.
- **The architectural pattern** (webview rendering HTML/CSS/JS communicating with a Rust backend over a bridge) is preserved, validating the Tauri choice.
- **No extension source files** were directly imported, adapted, or ported into the desktop frontend.

### Assessment

The spike succeeded at proving that Tauri can host a webview frontend talking to the real Rust core over IPC. It **did not** succeed at demonstrating actual code reuse from the extension's rich panel implementations.

This means:
1. The "100% Existing UI Reuse" claim in the comparison matrix above is **aspirational, not yet validated.** The extension panels contain substantial VS Code API coupling (`vscode.WebviewPanel`, `vscode.Uri`, `getNonce()` CSP patterns) that will require non-trivial adaptation work — not a simple copy.
2. The rewrite cost should be factored into the **Phase 15 checkpoint decision** (see PRD §13 resolved rationale). If porting the real panels proves to take as much effort as rewriting them, the Tauri-over-egui/Iced advantage shrinks to "familiar tech stack" rather than "reuse existing code."
3. **Phase 15 exit criteria should include:** at least one real extension panel (`inspectorPanel.ts` or `hexPanel.ts`) genuinely ported and running in the desktop shell, not just a visual lookalike.

