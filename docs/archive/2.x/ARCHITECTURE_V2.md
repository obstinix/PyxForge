# PyxForge Desktop V2 Architecture

This document describes the target decoupled architecture for **PyxForge Desktop** as a standalone, cross-platform IDE.

---

## 1. High-Level Architecture Diagram

```
                 ┌──────────────────────────────────────┐
                 │       PyxForge Desktop Shell         │
                 │   (Tauri Webview / HTML / JS / TS)   │
                 └──────────┬─────────────────▲─────────┘
                            │                 │
                            │ IPC Commands    │ Core Events
                            │ (Tauri Bridge)  │ (JSON-RPC)
                            ▼                 │
                 ┌────────────────────────────┴─────────┐
                 │          Tauri Rust Backend          │
                 │      (desktop/src-tauri/src/)        │
                 └──────────┬─────────────────▲─────────┘
                            │                 │
                            │ stdin stream    │ stdout stream
                            │ (JSON-RPC)      │ (JSON-RPC)
                            ▼                 │
                 ┌────────────────────────────┴─────────┐
                 │          Native Rust Core            │
                 │     (core/src/ - pyxforge-core)      │
                 └──────────────────────────────────────┘
```

---

## 2. Component breakdown

### 2.1 PyxForge Desktop Shell (Frontend)
Located in `desktop/`. Built using web technologies (HTML, CSS, TypeScript) and served via Tauri's native OS webview container.
- **Editor Panel:** Provides code syntax editing. (Will transition to standard web-based editor components like Monaco/CodeMirror or integrated text containers in Phase 15).
- **CPU Register Inspector:** Visual CPU register dashboard displaying states and diff highlights. Reuses the existing HTML/CSS code from the VS Code extension's webview.
- **Hex Dump Explorer:** Interactive grid mapping compiled binaries. Reuses the existing hex grid template.
- **Workspace Navigation:** Docking system handling panels (Left: File Explorer; Center: Code Editor / Terminal Log; Right: CPU Inspector / Hex Viewer).

### 2.2 Tauri Rust Backend (Bridge Layer)
Located in `desktop/src-tauri/`. Written in Rust.
- **IPC Handlers:** Implements the `#[tauri::command]` functions (e.g. `call_core`) to bridge the webview frontend to native operating system capabilities.
- **Process Orchestrator:** Spawns and manages the `pyxforge-core` background process as a persistent or command-driven lifecycle node.
- **PTY Session Manager:** Integrates standard terminal spawn utilities (`portable-pty` library) to manage stdin/stdout pipes for the integrated console view.

### 2.3 Native Rust Core (Systems Engine)
Located in `core/`. Written in Rust. Remains completely frontend-agnostic and IDE-independent.
- **Toolchain Orchestration (`build.rs`):** Executes compiler suites (NASM, GCC, Clang, Cargo) in dependency order based on project configuration files (`pyxforge.toml`).
- **Scaffold Generator (`scaffold.rs`):** Populates project folders with templates.
- **QEMU Subprocess Manager (`qemu.rs`):** Detaches QEMU instances with custom memory/debugging arguments.
- **GDB Remote Protocol Client:** Interfaces with GDB server stubs.
- **QMP Client (`qmp.rs`):** Drives VM state queries and graceful VM shutdowns over UNIX or TCP sockets.

---

## 3. Communication Protocols

1. **Frontend-to-Backend IPC:** Tauri's `invoke` command is used to serialize and send command envelopes from the JavaScript webview to the Rust backend:
   ```typescript
   const result = await invoke("call_core", { requestJson: JSON.stringify(request) });
   ```
2. **Backend-to-Core Bridge:** The Rust backend writes the JSON request string with a trailing newline `\n` to the stdin pipe of `pyxforge-core`, and reads the matching JSON line from the stdout pipe:
   ```
   // Request
   {"cmd":"ping"}
   
   // Response
   {"status":"ok","message":"pong","version":"0.1.0"}
   ```
3. **Core-to-Emulator QMP:** The Core communicates with QEMU using QMP JSON-RPC over TCP sockets (Windows) or Unix Domain Sockets (macOS/Linux).

---

## 4. Portability & Future Scalability

- **Unified Build Engine:** Since `core` has no dependencies on VS Code APIs, it compiles natively on Windows, macOS, and Linux, ensuring that the same systems build logic executes identically in both the standalone desktop IDE and the VS Code compatibility extension.
- **Webview Agnosticism:** Reusing webview code for panel UI ensures that we don't lock the app to any single platform-native layout library. If we decide to migrate to a native Rust UI stack in the future (e.g. egui/iced), we can swap out the Tauri wrapper while maintaining the same JSON-RPC core protocol.
