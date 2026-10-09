# ADR 0005: Native Go and Fyne application

- **Status:** Accepted (supersedes ADR 0003)
- **Date:** 2026-10-09
- **Decision owner:** obstinix (decision D1, 2026-10-08)

## Context

PyxForge 2.x shipped two frontends over a Rust core: a Tauri 2 desktop shell rendering HTML,
CSS and TypeScript in a webview, and a VS Code extension (ADR 0003). The 3.0 brief requires a
native application with no web technology in application code or build configuration, offline
operation, and real Neovim as the editor. ADR 0003 chose Tauri; it does not meet those
requirements.

## Decision

PyxForge 3.0 is one Go program built with Fyne 2.8:

- The desktop app and the command line are the same binary (`cmd/pyxforge`), sharing packages
  for configuration, tool detection and workspace discovery.
- The UI is Fyne widgets drawn with OpenGL through cgo (GLFW 3.4). No webview, HTML, CSS or
  JavaScript; `tools/forbidcheck` enforces this in CI and in the pre-commit hook.
- The 2.x Rust core is not linked. Its behaviour is ported to Go package by package, with its
  tests as golden cases (`internal/config` carries the first sixteen).
- The 2.x Tauri app and VS Code extension stay in `legacy/` until feature parity, then are
  deleted (decision D8).

## Evidence

- The shell, design system, command palette, CLI and configuration run on Windows 11; CI builds
  and tests them on Ubuntu and Windows and compiles them on macOS.
- Fyne needs a C toolchain: MinGW-w64 on Windows; on Linux GLFW's X11 and Wayland headers
  (`libx11-dev`, `xorg-dev`, `libwayland-dev`, `libxkbcommon-dev`), as listed in
  `docs/development/SETUP.md`.

## Consequences

- One language for the application, and a single static binary per platform.
- cgo is required on every platform; cross-compiling the desktop app needs a C cross-toolchain.
- Fyne has no accessibility tree for screen readers (recorded in `docs/design/PHASE1_REVIEW.md`).
