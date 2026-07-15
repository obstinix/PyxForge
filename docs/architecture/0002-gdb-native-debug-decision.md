# ADR 0002: Use Native Debug Extension for GDB Integration

**Status:** Accepted  
**Date:** 2026-07-15  
**Decision Makers:** Piyush (obstinix)

## Context

Phase 3 of PyxForge requires integrating GDB remote debugging with QEMU. The QEMU process is already launched with `-s -S` flags (Phase 2), exposing a GDB stub on a configurable port. The question is how to bridge VS Code's debug UI to GDB.

Two approaches were evaluated:

1. **Native Debug extension (`webfreak.debug`)** — An existing, maintained VS Code extension that implements the Debug Adapter Protocol (DAP) for GDB. It supports remote attach via `"remote": true` and `"target": ":<port>"` in its launch configuration.

2. **Custom Rust DAP** — Build a GDB/MI parser and Debug Adapter Protocol server from scratch in the PyxForge core binary.

## Decision

**Use the Native Debug extension (`webfreak.debug`) for v1.**

PyxForge dynamically generates the correct Native Debug configuration (architecture, GDB path, target port, setup commands) and launches it via `vscode.debug.startDebugging()`. No static `launch.json` is required — the configuration is computed from `pyxforge.toml` at runtime.

## Rationale

- **PRD alignment:** "Don't build an editor — build the glue." Native Debug already handles the complex GDB/MI protocol and DAP implementation. Building that from scratch adds months of work with no differentiated value for v1.
- **Maturity:** Native Debug is battle-tested with GDB remote debugging, the exact workflow PyxForge needs.
- **Low coupling:** PyxForge only depends on Native Debug's launch configuration schema, not its internals. If Native Debug ever becomes limiting, the switch to a custom DAP requires changing only the extension-side launch logic — the core's `debug-config` command and architecture detection remain unchanged.

## Consequences

- PyxForge declares `extensionDependencies: ["webfreak.debug"]` in its `package.json`.
- Users must install the Native Debug extension alongside PyxForge.
- The register/memory inspector panel (P1) may require a custom DAP in the future if Native Debug's variable presentation is too limited. This is a known deferred decision point.

## Alternatives Considered

| Alternative | Why rejected |
|---|---|
| Custom Rust DAP | Multi-month effort to implement GDB/MI parsing + DAP server. No differentiated value over Native Debug for the remote-attach use case. Deferred to Phase 4+ if needed. |
| CodeLLDB | Primarily for LLDB, not GDB. Our users are in the GDB ecosystem (cross-GCC, NASM, QEMU). |
| Cortex-Debug | Focused on ARM Cortex-M with OpenOCD/J-Link. Wrong target architecture. |
