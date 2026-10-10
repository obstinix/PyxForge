# PyxForge Desktop — Reference Analysis

This document presents the architectural study of the selected reference projects to guide the design of **PyxForge Desktop**.

---

## 1. Alacritty (`alacritty/alacritty`)

- **License:** Apache-2.0
- **Strengths:** 
  - Extremely fast, GPU-accelerated terminal emulator.
  - Modularity: The terminal emulation state engine is isolated in the `alacritty_terminal` crate, separate from the windowing and rendering logic.
- **Weaknesses:**
  - Rendering and windowing are tightly bound to `winit` and Glutin/OpenGL, which makes embedding the full graphical window as a sub-widget in another UI library complex.
- **Reusable Ideas:**
  - Utilizing `portable-pty` (from the WezTerm project) or `alacritty_terminal` to manage PTY spawning and state parsing.
- **Unsuitable Ideas:**
  - Embedding Alacritty's full GPU renderer directly into our desktop UI, which would create windowing and thread-safety conflicts.
- **Recommendation:**
  - If we use a web-based UI stack (like Tauri), we should use `xterm.js` for rendering inside the frontend, communicating with a backend PTY process managed in Rust using the `portable-pty` crate.
  - If we use a native Rust UI stack (like egui/iced), we should use a custom PTY reader/writer along with standard canvas-text rendering.

---

## 2. QEMU (`qemu/qemu`)

- **License:** GPLv2 (Copyleft — **do not vendor or copy source code**)
- **Strengths:**
  - Industry-standard machine emulator with rich automation controls (QMP, monitor, serial/parallel output redirections).
- **Weaknesses:**
  - Command-line arguments are highly complex and differ across architectures.
- **Reusable Ideas:**
  - **QMP (QEMU Monitor Protocol):** PyxForge should extend its existing QMP client (`qmp.rs`) to send JSON commands for CPU state queries, snapshot management, and VM pause/resume.
  - **Human Monitor Console:** Interfacing with QEMU's monitor over QMP via the `human-monitor-command` RPC call.
- **Unsuitable Ideas:**
  - Direct linking or library embedding. QEMU must always run as a separate external subprocess to maintain license separation and process isolation.
- **Recommendation:**
  - Keep QEMU as a detached subprocess. Connect to it via local Unix domain sockets (Unix) or TCP localhost sockets (Windows) using the typed QMP protocol.

---

## 3. Pwndbg (`pwndbg/pwndbg`)

- **License:** MIT
- **Strengths:**
  - Advanced visualization of GDB context for exploit development and systems debugging.
  - Excellent layout of registers, flags, stack frames, and dereferenced memory pointers.
- **Weaknesses:**
  - Purely CLI/TUI-based; tightly coupled to GDB Python scripting APIs.
- **Reusable Ideas:**
  - Register diffing: highlighting registers in different colors (e.g. red/green) when they change between debugger steps.
  - Dereference chains: showing where a pointer in a register points (e.g., `rax -> 0x7fffffffe000 -> 0x555555554000 ("my string")`).
  - Flag expansion: splitting flags registers (like EFLAGS) into their constituent bits (ZF, CF, SF) for easy reading.
- **Unsuitable Ideas:**
  - TUI/CLI text wrapping. We have a graphical webview/UI dashboard, so we can render these as an interactive graphical tree or table.
- **Recommendation:**
  - Integrate pwndbg's register highlighting, flag parsing, and pointer dereference visualizations directly into the existing `CPU Inspector` panel.

---

## 4. Lapce (`lapce/lapce`)

- **License:** Apache-2.0 / MIT
- **Strengths:**
  - Standalone high-performance code editor written in Rust.
  - Uses a data-driven model and has an elegant panel docking system.
- **Weaknesses:**
  - Custom UI library (Floem) is still young, has a steep learning curve, and lacks rich pre-existing widgets compared to mature web-based or native desktop toolkits.
- **Reusable Ideas:**
  - Modular workspace pane configuration (left bar for file explorer, bottom for terminal/console, right for inspector/hex).
- **Unsuitable Ideas:**
  - Forking or attempting to match Lapce's custom rendering architecture.
- **Recommendation:**
  - Use Lapce's panel and docking system layout as a visual UX blueprint. Implement this layout using our chosen desktop UI toolkit.

---

## 5. CodeAssist (`tyron12233/CodeAssist`)

- **License:** GPLv3 (Copyleft — **do not vendor**)
- **Strengths:**
  - Fully featured mobile IDE for building Android apps on Android devices.
- **Weaknesses:**
  - Mobile code editor UX is very different from desktop systems programming workflows.
- **Reusable Ideas:**
  - None for the immediate roadmap.
- **Unsuitable Ideas:**
  - Building mobile application interfaces at this stage.
- **Recommendation:**
  - Keep Android support as an explicit **parking-lot** item as outlined in `docs/PRD.md`. Do not allocate engineering resources to it in this sprint.
