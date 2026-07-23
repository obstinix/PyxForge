# ADR 0004: Native Code Editor Engine Architecture

- **Status:** Approved
- **Date:** 2026-07-23
- **Deciders:** PyxForge Core Team
- **Technical Story:** Phase 22 Editor Architecture Study

---

## 1. Context & Problem Statement

PyxForge Desktop relies on a hybrid Tauri 2.0 shell (Rust backend + Webview frontend). Following the successful completion of Phase 15–20 (Desktop workspace docking, PTY integration, QMP snapshot control, and CPU inspection), PyxForge requires an integrated, high-performance code editor for bootloader and OS development (`.asm`, `.c`, `.h`, `.ld`, `Makefile`, `.toml`).

The key architectural question is: **How should text buffer management, rendering, syntax highlighting, and filesystem operations be structured across the Rust backend and TypeScript frontend?**

---

## 2. Options Evaluated

### Option A: Webview-Based Editor Component (CodeMirror 6 / Monaco Editor)
*   **Description:** Embed a modern, componentized web editor engine directly within the Tauri webview renderer layer.
*   **Pros:**
    *   Immediate compatibility with Tauri's web environment and Vite bundle pipeline.
    *   Extensible extension/plugin architecture and syntax highlighting parsers (`@codemirror/lang-cpp`, `@codemirror/state`, etc.).
    *   Lightweight memory footprint compared to full Monaco bundle when using CodeMirror 6 (~150KB vs ~4MB).
    *   Native integration with existing CSS design tokens (`--bg-surface`, `--accent`, `--font-mono`).
*   **Cons:** Text buffer resides in JS heap unless synchronized with Rust backend via Tauri IPC RPCs.
*   **Verdict:** Selected.

### Option B: Rust Core Hybrid Engine (`ropey` + `tree-sitter` in Rust, Custom TS Canvas Render)
*   **Description:** Implement text buffer storage using Rust's `ropey` crate and incremental AST generation via `tree-sitter-c` / `tree-sitter-x86` in `src-tauri/`, communicating line view windows to a custom Canvas/DOM renderer in TypeScript.
*   **Pros:** Ultra-low memory consumption, zero-copy buffer operations on multi-megabyte files, native Rust AST query capabilities.
*   **Cons:** High implementation overhead for cursor handling, multi-caret selection, IME, line wrapping, and virtualized scrolling.
*   **Verdict:** Rejected for Phase 23 (excessive duplication of editor view state), but retained as the backend AST/diagnostics indexing pathway for future Phase releases.

### Option C: Reopen ADR 0003 for Native GPU Window Rendering (Lapce / Floem / Zed approach)
*   **Description:** Abandon Tauri webview architecture in favor of a full native Rust UI framework (wgpu / Floem / GPUI).
*   **Pros:** Maximum rendering framerates (120+ FPS) and zero webview overhead.
*   **Cons:** Invalidates all existing webview components (xterm.js PTY, CPU Inspector, Hex Viewer, AI Assist panel) built across Phases 15–20.
*   **Verdict:** Rejected (violates ADR 0003 and post-checkpoint stability guarantees).

---

## 3. Decision & Rationale

We decide to adopt **Option A using CodeMirror 6** for PyxForge Desktop's editor engine in Phase 23, augmented by RPC filesystem operations (`read_workspace_file`, `write_workspace_file`, `list_workspace_files`) in the Tauri Rust core (`desktop/src-tauri`).

### Key Reasons:
1.  **Modularity & Weight:** CodeMirror 6's functional state architecture allows selective composition of editor features without shipping heavy IDE monoliths.
2.  **Aesthetic Alignment:** Integrates smoothly into PyxForge's flat, cyan-accented design system tokens (`--bg-surface`, `--bg-inset`, `--accent`, `--font-mono`) without iframe/shadow-DOM CSS isolation battles.
3.  **Low Friction & High Stability:** Preserves the Tauri architecture validated in Phase 15.

---

## 4. Architectural Topology

```
+-------------------------------------------------------------------------+
|                      PyxForge Desktop Webview                           |
|                                                                         |
|  +--------------------+   +-------------------+   +------------------+  |
|  | Real File Tree     |   | CodeMirror 6      |   | xterm.js PTY     |  |
|  | Workspace Explorer |   | Code Editor       |   | Terminal         |  |
|  +---------+----------+   +---------+---------+   +--------+---------+  |
|            |                        |                      |            |
+------------|------------------------|----------------------|------------+
             | Tauri IPC (RPC)        | Tauri IPC (RPC)      | PTY Stream
             v                        v                      v
+-------------------------------------------------------------------------+
|                      Tauri Rust Backend (`src-tauri`)                   |
|                                                                         |
|  +-------------------------------------------------------------------+  |
|  | Filesystem Commands: `list_workspace_files`, `read_workspace_file`, |  |
|  | `write_workspace_file`, `HexDump`                                 |  |
|  +-------------------------------------------------------------------+  |
|                                     |                                   |
|                                     v                                   |
|                          Host Operating System Disk                     |
+-------------------------------------------------------------------------+
```

---

## 5. RPC Boundary & Data Flow

1. **Workspace Tree Listing:**
   `list_workspace_files(path: String)` -> Returns array of `FileNode { path: String, is_dir: bool, name: String }`.
2. **File Read:**
   `read_workspace_file(path: String)` -> Reads raw file contents and returns UTF-8 text string (or error if binary).
3. **Binary Fallback:**
   If file is detected as binary, automatically delegates to the `HexDump` RPC in `core/` and switches active view to the Hex Viewer panel.
4. **File Save:**
   `write_workspace_file(path: String, content: String)` -> Writes UTF-8 text directly to filesystem and updates dirty state indicator.

---

## 6. Consequences & Risks

*   **Positive:** Enables editing of real PyxisOS bootloader (`boot.asm`), kernel (`kernel.c`), and build configuration (`Makefile`, `pyxforge.toml`) files directly within PyxForge Desktop.
*   **Mitigation for Large Files:** Files larger than 2MB fallback to streaming RPC or binary hex inspection to maintain UI responsiveness.
