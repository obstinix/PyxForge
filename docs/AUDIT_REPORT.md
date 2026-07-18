# PyxForge Audit Report

**Date:** 2026-07-19  
**Branch:** `pyxforge/phase-13-desktop-feasibility`  
**Latest Commit:** `6dc0b9a`

---

## 1. Current Architecture

PyxForge is currently structured as a client-server IDE system split across:
1. **VS Code Extension (`extension/`):** Written in TypeScript. Provides the user interface, command registry, panels (`CPU Inspector`, `Hex Viewer`), presets selector, syntax highlighting, and `vscode.lm`-based AI helper.
2. **Rust Core Backend (`core/`):** Written in Rust. Handles low-level toolchain orchestration (NASM, GCC, Clang, Cargo), project template scaffolding, QEMU subprocess management with graceful QMP shutdown, a GDB bridge, and a Hex dump formatter.
3. **Communication Interface:** JSON-RPC 2.0 protocol implemented over stdin/stdout pipelines (`protocol.rs` / `main.rs`). The VS Code extension spawns the Rust Core as a subprocess and writes serialized commands (e.g. `{"cmd":"build","profile":"bootloader","projectRoot":"..."}`) to its stdin, reading responses from stdout.

---

## 2. Repository & Phase State

- **Highest Merged Phase:** **Phase 12** (`pyxforge/phase-12-qemu-protocol-hardening` is fully merged, containing the typed RPC protocol, QMP client, register diffing, and preset-based configurations).
- **Current Active Sprint:** **Phase 13** (`pyxforge/phase-13-desktop-feasibility`).
- **Commits Check:** The latest main incorporates Phase 11 & Phase 12 changes.

---

## 3. Actual CI Status

- **Workflow File:** `.github/workflows/ci.yml` builds and tests across `ubuntu-latest`, `macos-latest`, and `windows-latest`.
- **Audit Findings:** 
  - `windows-latest` was passing.
  - `ubuntu-latest` and `macos-latest` were failing at the `Run Rust Core Tests` step because the `qemu::tests::test_build_qemu_args_default` and `qemu::tests::test_build_qemu_args_kernel_boot` tests hardcoded a Windows drive path (`C:\Projects\my-os`) which Unix filesystems treat as a single filename component, breaking path-joining expectations.
- **Remediation:** We modified `core/src/qemu.rs` to construct `project_root` and assert paths dynamically depending on the host OS. All 59 core backend tests now pass locally on all platforms.
- **Extension Tests:** All 10 integration tests build and pass cleanly (tested via `vscode-test` with `xvfb-run` on Linux).

---

## 4. VS Code API Coupling Points

The following architectural components are tightly coupled to the VS Code Extension Host:
1. **UI Views (Inspector & Hex Explorer):**
   - Implemented as `vscode.WebviewPanel` in `extension/src/inspectorPanel.ts` and `extension/src/hexPanel.ts`.
   - IPC uses `panel.webview.postMessage` and `webview.onDidReceiveMessage`.
2. **Preset Configuration UI:**
   - Prompting and selection are managed via `vscode.window.showQuickPick`.
3. **Task & Command Dispatcher:**
   - Commands are bound using `vscode.commands.registerCommand` in `extension/src/extension.ts`.
4. **AI Copilot:**
   - The AI assist command interacts directly with the `vscode.lm` APIs for language model responses.
5. **Diagnostics Engine:**
   - Build error parsing outputs are pushed to a `vscode.DiagnosticCollection` created during activation.

---

## 5. Technical Debt & Portability Issues

- **Subprocess Paths:** `extension.ts` resolves `pyxforge-core` by looking relative to `__dirname` under `../core/target/debug/`. This is suitable for development but assumes a fixed development folder structure.
- **Unified Launch Configuration:** Scaffolding generation creates `.vscode/launch.json` and `.vscode/tasks.json`, which are editor-specific.
- **GDB Attach:** Standard GDB integration uses VS Code's `vscode.debug.startDebugging` using custom launch configurations. Surviving outside VS Code requires the native desktop shell to implement its own GDB adapter/GDB/MI communication or embed a terminal connected to a GDB session.

---

## 6. Release Readiness

- **Extension:** High. The extension can be packaged as a `.vsix` immediately using `vsce package`.
- **Desktop IDE:** Low. Currently, no standalone desktop shell, window manager, docking panel system, or native editor exists. The codebase must be decoupled so that a standalone UI can invoke the same Rust Core over the JSON-RPC interface.
