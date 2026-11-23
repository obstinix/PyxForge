# Architecture Decision Record 0001: Core-Extension Split

## Context

PyxForge is built to assist in bare-metal systems development (assembly compilation, bootloader launching, real/protected mode debugging). We need to support VS Code workspace integration, but also process management and heavy systems orchestration tasks that must remain fast, reliable, and decoupled from the editor host.

## Decision

We split the codebase into two primary parts:
1. **Extension Shell (TypeScript)**: Interacts with the VS Code Extension API. It handles command palette commands, configuration, and editor tasks.
2. **Core Engine (Rust)**: Executes systems operations like launching and managing QEMU, communicating with GDB, running build tasks, and parsing protocols.

Communication between the extension and the core is handled via a simple JSON-over-stdio IPC protocol (spawned as a child process per call).

## Alternatives Considered

- **All-TypeScript**: Rejected. Systems-level task automation, QEMU process orchestration, and binary format parsing (for future milestones) would lack performance, strong compiler guarantees, and reuse value outside of VS Code.
- **All-Rust**: Rejected. VS Code does not support native Rust extensions. The editor extension shell must be in JavaScript/TypeScript.

## Status

Approved.
