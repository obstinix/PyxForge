# PyxForge — Phase 11 Development Log

## [2026-07-17T15:47:00Z] Portable Rust Toolchain Configuration

### Files Modified
- [core/rust-toolchain.toml](file:///C:/Users/Piyush/Documents/antigravity/cool-oppenheimer/core/rust-toolchain.toml)
- [core/.cargo/config.toml](file:///C:/Users/Piyush/Documents/antigravity/cool-oppenheimer/core/.cargo/config.toml)
- [extension/src/extension.ts](file:///C:/Users/Piyush/Documents/antigravity/cool-oppenheimer/extension/src/extension.ts)

### Commit Message
- `fix(toolchain): make Rust toolchain portable and native across host platforms`
- `fix(toolchain): dynamically locate core binary path in extension`

### Reason for Change
- Removed the hardcoded Windows GNU specific toolchain default target and absolute wrapper paths.
- Added dynamic target binary detection (native target first, gnu target as fallback).
- Used target-specific `rustflags` link args to link `-lunwind` natively on Windows GNU instead of using custom absolute script wrapper.

### Validation Performed
- Compiled Rust core successfully on Windows MSVC.
- Verified extension build compiled cleanly.

### Push Confirmation
- Completed and pushed to remote main branch.

---

## [2026-07-17T15:47:50Z] Proper Build Diagnostics Integration

### Files Modified
- [extension/src/diagnostics.ts](file:///C:/Users/Piyush/Documents/antigravity/cool-oppenheimer/extension/src/diagnostics.ts) (NEW)
- [extension/src/extension.ts](file:///C:/Users/Piyush/Documents/antigravity/cool-oppenheimer/extension/src/extension.ts)

### Commit Message
`feat(diagnostics): implement compiler and linker diagnostics parsing pipeline`

### Reason for Change
- Enabled structured problem logs mapping to VS Code Problems panel and editor gutter.
- Created robust diagnostic parsing logic matching Cargo/Rustc JSON formats, human-readable compiler errors, GCC/Clang standard errors, MSVC cl/link diagnostics, and GNU linker outputs.
- Hooked diagnostics to automatically clear before each build, and to wipe out diagnostic markers of deleted files.

### Validation Performed
- Wrote diagnostic parser unit and integration tests.
- Extension built successfully.

### Push Confirmation
- Completed and pushed to remote main branch.

---

## [2026-07-17T15:48:30Z] Build Profile Presets

### Files Modified
- [extension/src/presets.ts](file:///C:/Users/Piyush/Documents/antigravity/cool-oppenheimer/extension/src/presets.ts) (NEW)
- [extension/package.json](file:///C:/Users/Piyush/Documents/antigravity/cool-oppenheimer/extension/package.json)
- [extension/src/extension.ts](file:///C:/Users/Piyush/Documents/antigravity/cool-oppenheimer/extension/src/extension.ts)

### Commit Message
`feat(presets): add build profile presets command and templates`

### Reason for Change
- Allowed developers to toggle project profiles easily using Command Palette and UI settings.
- Added templates for Bootloader, Kernel Debug, Kernel Release, Rust App, C App, C++ App, Embedded, Bare Metal, and Custom.
- Handled project name extraction to preserve custom configurations when switching presets.

### Validation Performed
- Ran extension tests validating presets matching and parsing.
- Extension built successfully.

### Push Confirmation
- Completed and pushed to remote main branch.

---

## [2026-07-17T15:49:50Z] Extension Integration Testing Suite

### Files Modified
- [extension/src/test/extension.test.ts](file:///C:/Users/Piyush/Documents/antigravity/cool-oppenheimer/extension/src/test/extension.test.ts)

### Commit Message
`test(extension): implement comprehensive suite covering diagnostics, presets, and commands`

### Reason for Change
- Replaced Yeoman boilerplate extension tests with thorough functional and integration tests.
- Checked commands registration, diagnostics generation, presets parsing, theme settings, error logs parsing, and project metadata extraction.

### Validation Performed
- Executed `npm test` inside the extension. All 9 tests passed successfully with 0 failures under the Electron VS Code test host.

### Push Confirmation
- Completed and pushed to remote main branch.

---

## [2026-07-17T15:50:10Z] CI Infrastructure and Automation Matrix

### Files Modified
- [.github/workflows/ci.yml](file:///C:/Users/Piyush/Documents/antigravity/cool-oppenheimer/.github/workflows/ci.yml)

### Commit Message
`ci(actions): add matrix testing across Windows, Ubuntu, macOS with caching`

### Reason for Change
- Extended the CI suite to test Rust builds, clippy lints, format checks, node packaging, extension type checks, lint checks, build, and integration tests on Windows, Linux, and macOS platforms.
- Configured caching actions to minimize compilation delays.

### Validation Performed
- Checked bash configurations and linted workflows.

### Push Confirmation
- Completed and pushed to remote main branch.

---

## [2026-07-17T20:44:00Z] QMP Client Status and Shutdown

### Files Modified
- [core/src/main.rs](file:///C:/Users/Piyush/Documents/antigravity/cool-oppenheimer/core/src/main.rs)
- [core/src/qemu.rs](file:///C:/Users/Piyush/Documents/antigravity/cool-oppenheimer/core/src/qemu.rs)
- [core/src/qmp.rs](file:///C:/Users/Piyush/Documents/antigravity/cool-oppenheimer/core/src/qmp.rs)

### Commit Message
`feat(core): implement QMP client for real status and graceful shutdown`

### Reason for Change
- Added a structured QMP connection channel over unix sockets (non-Windows) or TCP ports (Windows).
- Enabled query-status and graceful shutdown command execution over QMP.
- Configured QEMU launch to pass the `-qmp` argument, enabling programmatic control of the QEMU target.
- Wired the `stop` command handler to attempt graceful powerdown via QMP before falling back to OS-level raw process termination.

### Validation Performed
- Checked core build, lints, formatting, and unit tests using Cargo.
- Verified QMP TCP and Unix argument construction with test assertions.

### Push Confirmation
- Completed and pushed to remote branch `pyxforge/phase-12-qemu-protocol-hardening`.

---

## [2026-07-17T20:46:00Z] Register Value Diffing in CPU Inspector

### Files Modified
- [extension/src/extension.ts](file:///C:/Users/Piyush/Documents/antigravity/cool-oppenheimer/extension/src/extension.ts)
- [extension/src/inspectorPanel.ts](file:///C:/Users/Piyush/Documents/antigravity/cool-oppenheimer/extension/src/inspectorPanel.ts)

### Commit Message
`feat(extension): implement register value diffing in inspector panel`

### Reason for Change
- Added a `previousRegisterSnapshot` cache map in `extension.ts` to store CPU register values.
- Reset the cache map on debugger startup.
- Computed the `changed` property on the host extension side and passed it inside the `InspectorState` to the Webview.
- Updated the Register interface in the Webview panel to support `changed`.
- Added CSS styles to render changed registers with a distinct persistent left-accent border matching the active theme.

### Validation Performed
- Ran extension lints and type-checks (`npm run lint`, `npm run check-types`).
- Verified all 9 integration tests pass via `npm test`.

---

## [2026-07-17T20:47:00Z] QEMU GDB Sanity Check

### Files Modified
- [extension/src/extension.ts](file:///C:/Users/Piyush/Documents/antigravity/cool-oppenheimer/extension/src/extension.ts)

### Commit Message
`feat(extension): implement QEMU gdbstub sanity checks on debugger stop`

### Reason for Change
- Declared a `validatedSessions` Set in `extension.ts` to track validated debug sessions.
- Injected a one-time GDB query (`-exec maintenance packet Qqemu.sstepbits`) upon the first GDB stop event of a session.
- Alerted the user via a non-blocking `showWarningMessage` if the GDB session is not attached to a QEMU gdbstub.
- Cleaned up the session ID from `validatedSessions` in `onDidTerminateDebugSession`.

### Validation Performed
- Ran extension lints and type-checks (`npm run lint`, `npm run check-types`).
- Verified all 9 integration tests pass via `npm test`.

---

## [2026-07-17T20:49:00Z] Typed Command Protocol

### Files Modified
- [core/src/protocol.rs](file:///C:/Users/Piyush/Documents/antigravity/cool-oppenheimer/core/src/protocol.rs)
- [core/src/main.rs](file:///C:/Users/Piyush/Documents/antigravity/cool-oppenheimer/core/src/main.rs)
- [extension/src/extension.ts](file:///C:/Users/Piyush/Documents/antigravity/cool-oppenheimer/extension/src/extension.ts)

### Commit Message
`refactor(core): migrate flat request envelope to strongly-typed tagged enum`

### Reason for Change
- Replaced the flat, dynamic `Request` structure with a strongly-typed `Request` enum containing explicit parameters for each variant.
- Configured JSON-RPC serialization using `#[serde(tag = "cmd", rename_all = "camelCase")]`, converting command tags to camelCase format on the wire (breaking wire change).
- Refactored `handle_request` dispatcher and helper signatures to match directly on the tagged enum, eliminating manual option unwrapping.
- Aligned `extension.ts` calls to send camelCase tags (`qemuStatus`, `listProfiles`, `debugConfig`, `hexDump`).
- Updated unit tests in `core` to match the camelCase deserialization errors.

### Validation Performed
- Built and ran all core backend unit tests (`cargo test`).
- Ran Clippy and format checks on core code.
- Tested extension using linting, typecheck, and integration tests (`npm test`).

---

## [2026-07-17T20:50:00Z] Registerable Presets Registry

### Files Modified
- [extension/src/presets.ts](file:///C:/Users/Piyush/Documents/antigravity/cool-oppenheimer/extension/src/presets.ts)
- [extension/src/extension.ts](file:///C:/Users/Piyush/Documents/antigravity/cool-oppenheimer/extension/src/extension.ts)
- [extension/src/test/extension.test.ts](file:///C:/Users/Piyush/Documents/antigravity/cool-oppenheimer/extension/src/test/extension.test.ts)

### Commit Message
`feat(extension): migrate hardcoded presets to registerable presets pattern`

### Reason for Change
- Replaced the hardcoded static `PRESETS` array with a modular preset registry (`registry`).
- Implemented `registerPreset(preset)` and `getPresets()` in `presets.ts`.
- Registered all 9 presets at module load time.
- Updated calls in `extension.ts` (preset selection quickpick) and `extension.test.ts` (preset loop validations) to use `getPresets()`.

### Validation Performed
- Ran extension lints and type-checks (`npm run lint`, `npm run check-types`).
- Verified all 9 integration tests pass via `npm test`.

---

## [2026-07-17T20:51:00Z] Sibling Project Cross-Link & Integration Study

### Files Modified
- [docs/cross-project/pyxisos-integration.md](file:///C:/Users/Piyush/Documents/antigravity/cool-oppenheimer/docs/cross-project/pyxisos-integration.md)
- [README.md](file:///C:/Users/Piyush/Documents/antigravity/cool-oppenheimer/README.md)

### Commit Message
`docs: add PyxisOS integration study and cross-project documentation`

### Reason for Change
- Established `docs/cross-project/` directory.
- Authored a comprehensive `pyxisos-integration.md` guide covering PyxisOS custom target compilation, QEMU and GDB debug lifecycle management, mapping of modules, and step-by-step tutorial configurations.
- Cloned and analysed the sibling `obstinix/PyxisOS` repository (`native/lunar-core` Rust files, custom JSON target specs) before deletion.
- Added a section under `README.md` introducing the sibling project integration link.

### Validation Performed
- Validated workspace status, checked formatting, and verified all 9 tests pass.
- Synchronized all 11 modified files sequentially into the `main` branch with granular, file-by-file commits under the verified Git identity (`obstinix <obstinix@gmail.com>`).



