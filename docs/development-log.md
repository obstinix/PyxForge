# PyxForge development log

Newest first. Each entry names its commits on `main`; the commit messages hold the detail.

## PyxForge 3.0

### 2026-10-11: PyxisOS, documentation

- `4e737ca` QEMU's own refusal ("Cannot load x86-64 image") is reported instead of a broken QMP
  connection. Found by booting PyxisOS v7.
- `8af2916` A Multiboot kernel's entry is decoded in 32-bit mode; `pyxforge inspect --arch`
  overrides an ELF file's mode.
- PyxisOS v7 built (`make build`), booted (`-kernel`, from an ELF32 copy) and debugged (GDB on its
  symbols, hardware breakpoint at `kernel_main`) through PyxForge; recorded in
  `docs/cross-project/pyxisos-integration.md`.
- 2.x documents moved to `docs/archive/2.x/`; roadmap, current state, README and this log
  rewritten for 3.0.

### 2026-10-10: stabilization, core IDE and systems features

- Process lifecycle: `f586a80` shutdown on SIGINT and SIGTERM, `395742b`/`f79df0b` child
  processes end with PyxForge after a forced exit (Windows Job Object, Linux Pdeathsig),
  `9590161` GDB exiting mid-session and back-to-back Debug launches, `2483628` workspace state
  saved while working, `87e5c4c` no console window when started from Explorer.
- Safety: `7ba0582` and `a90deef` nothing created outside the project through links or
  `output_dir`; Git can no longer run a repository's `core.fsmonitor` program; `83c45da`
  Neovim's log kept out of projects.
- Git: `0cb7b17`, `3dd7de7` branches (create, switch with a dirty-tree guard, delete with a merge
  check) and stash (save, apply, pop, drop).
- Build and Problems: `2c8845b` SARIF and GCC JSON diagnostics and messages without a line,
  `ca9e644` build errors inside Neovim, next and previous problem.
- Terminal: `caa0059` named sessions. Keys: `4cd5c26` chords for Problems, Git, Stop QEMU and a
  test that KEYMAP.md lists every chord.
- Systems: `ec490d5`, `3eba4e8` boot-sector map; `6a9dd67`, `ac0c85c`, `2a71869` diagnostic
  snapshots and QEMU machine states.
- Release: `a6e88c3` `pyxforge doctor --commands`, `b8f8971` reproducible release builds, CI
  artifacts and `docs/INSTALL.md`.
- Earlier the same day: the Terminal panel (`e7cbbfb`), the build engine and Build tab
  (`f211d2b`, `993ab86`), explorer and workspace persistence (`214222b`, `cba0a82`), the Git tab
  (`8d54584`, `2151329`), QEMU, GDB/MI and binary inspection (`b3899b9`, `d88dd03`, `e89298c`,
  `959151a`) and Run and Debug in the desktop app (`ad1afb8`).

Verification for this day: `go test -race ./...` on Windows and Linux (WSL2), with the QEMU, GDB
and Neovim integration tests running against real tools; the desktop app driven under Xvfb on
Linux; forced-exit checks on both platforms.

### 2026-10-09: command line and editor

- `23413c3`, `93ebeea`, `1bda113` toolchain detection, `pyxforge help/version/doctor/info`,
  `pyxforge.toml` with the 16 2.x cases as golden tests.
- `0c07501` through `fa8a710` the embedded Neovim: linegrid rendering, buffers as tabs, theme
  colours, an isolated configuration, `pyxforge setup editor`. ADRs 0005 and 0006 (`f912299`).
- `0adf614`, `076b0cb`, `47db609`, `8f2fdd1` CI on Ubuntu, Windows and macOS; `8c81bb8` a
  pre-commit hook for forbidden technology, formatting and secrets.

### 2026-10-08: design system and shell

- `0d1ca33` the Go module with Fyne; `9a10bd3` the forbidden-technology check; `680f414` the 2.x
  stack moved to `legacy/`.
- `e16a8d4` through `08f1bb7` the design system, five themes with two accents, the workbench
  shell, the command palette, keyboard policy (Neovim owns keys; Ctrl+Shift chords for the shell).

## PyxForge 2.x (Phase 11)

The entries below are from the 2.x line, whose code is now in `legacy/`.

### [2026-07-17T15:47:00Z] Portable Rust Toolchain Configuration

#### Files Modified
- [core/rust-toolchain.toml](file:///C:/Users/Piyush/Documents/antigravity/cool-oppenheimer/core/rust-toolchain.toml)
- [core/.cargo/config.toml](file:///C:/Users/Piyush/Documents/antigravity/cool-oppenheimer/core/.cargo/config.toml)
- [extension/src/extension.ts](file:///C:/Users/Piyush/Documents/antigravity/cool-oppenheimer/extension/src/extension.ts)

#### Commit Message
- `fix(toolchain): make Rust toolchain portable and native across host platforms`
- `fix(toolchain): dynamically locate core binary path in extension`

#### Reason for Change
- Removed the hardcoded Windows GNU specific toolchain default target and absolute wrapper paths.
- Added dynamic target binary detection (native target first, gnu target as fallback).
- Used target-specific `rustflags` link args to link `-lunwind` natively on Windows GNU instead of using custom absolute script wrapper.

#### Validation Performed
- Compiled Rust core successfully on Windows MSVC.
- Verified extension build compiled cleanly.

#### Push Confirmation
- Completed and pushed to remote main branch.

---

### [2026-07-17T15:47:50Z] Proper Build Diagnostics Integration

#### Files Modified
- [extension/src/diagnostics.ts](file:///C:/Users/Piyush/Documents/antigravity/cool-oppenheimer/extension/src/diagnostics.ts) (NEW)
- [extension/src/extension.ts](file:///C:/Users/Piyush/Documents/antigravity/cool-oppenheimer/extension/src/extension.ts)

#### Commit Message
`feat(diagnostics): implement compiler and linker diagnostics parsing pipeline`

#### Reason for Change
- Enabled structured problem logs mapping to VS Code Problems panel and editor gutter.
- Created robust diagnostic parsing logic matching Cargo/Rustc JSON formats, human-readable compiler errors, GCC/Clang standard errors, MSVC cl/link diagnostics, and GNU linker outputs.
- Hooked diagnostics to automatically clear before each build, and to wipe out diagnostic markers of deleted files.

#### Validation Performed
- Wrote diagnostic parser unit and integration tests.
- Extension built successfully.

#### Push Confirmation
- Completed and pushed to remote main branch.

---

### [2026-07-17T15:48:30Z] Build Profile Presets

#### Files Modified
- [extension/src/presets.ts](file:///C:/Users/Piyush/Documents/antigravity/cool-oppenheimer/extension/src/presets.ts) (NEW)
- [extension/package.json](file:///C:/Users/Piyush/Documents/antigravity/cool-oppenheimer/extension/package.json)
- [extension/src/extension.ts](file:///C:/Users/Piyush/Documents/antigravity/cool-oppenheimer/extension/src/extension.ts)

#### Commit Message
`feat(presets): add build profile presets command and templates`

#### Reason for Change
- Allowed developers to toggle project profiles easily using Command Palette and UI settings.
- Added templates for Bootloader, Kernel Debug, Kernel Release, Rust App, C App, C++ App, Embedded, Bare Metal, and Custom.
- Handled project name extraction to preserve custom configurations when switching presets.

#### Validation Performed
- Ran extension tests validating presets matching and parsing.
- Extension built successfully.

#### Push Confirmation
- Completed and pushed to remote main branch.

---

### [2026-07-17T15:49:50Z] Extension Integration Testing Suite

#### Files Modified
- [extension/src/test/extension.test.ts](file:///C:/Users/Piyush/Documents/antigravity/cool-oppenheimer/extension/src/test/extension.test.ts)

#### Commit Message
`test(extension): implement comprehensive suite covering diagnostics, presets, and commands`

#### Reason for Change
- Replaced Yeoman boilerplate extension tests with thorough functional and integration tests.
- Checked commands registration, diagnostics generation, presets parsing, theme settings, error logs parsing, and project metadata extraction.

#### Validation Performed
- Executed `npm test` inside the extension. All 9 tests passed successfully with 0 failures under the Electron VS Code test host.

#### Push Confirmation
- Completed and pushed to remote main branch.

---

### [2026-07-17T15:50:10Z] CI Infrastructure and Automation Matrix

#### Files Modified
- [.github/workflows/ci.yml](file:///C:/Users/Piyush/Documents/antigravity/cool-oppenheimer/.github/workflows/ci.yml)

#### Commit Message
`ci(actions): add matrix testing across Windows, Ubuntu, macOS with caching`

#### Reason for Change
- Extended the CI suite to test Rust builds, clippy lints, format checks, node packaging, extension type checks, lint checks, build, and integration tests on Windows, Linux, and macOS platforms.
- Configured caching actions to minimize compilation delays.

#### Validation Performed
- Checked bash configurations and linted workflows.

#### Push Confirmation
- Completed and pushed to remote main branch.

---

### [2026-07-17T20:44:00Z] QMP Client Status and Shutdown

#### Files Modified
- [core/src/main.rs](file:///C:/Users/Piyush/Documents/antigravity/cool-oppenheimer/core/src/main.rs)
- [core/src/qemu.rs](file:///C:/Users/Piyush/Documents/antigravity/cool-oppenheimer/core/src/qemu.rs)
- [core/src/qmp.rs](file:///C:/Users/Piyush/Documents/antigravity/cool-oppenheimer/core/src/qmp.rs)

#### Commit Message
`feat(core): implement QMP client for real status and graceful shutdown`

#### Reason for Change
- Added a structured QMP connection channel over unix sockets (non-Windows) or TCP ports (Windows).
- Enabled query-status and graceful shutdown command execution over QMP.
- Configured QEMU launch to pass the `-qmp` argument, enabling programmatic control of the QEMU target.
- Wired the `stop` command handler to attempt graceful powerdown via QMP before falling back to OS-level raw process termination.

#### Validation Performed
- Checked core build, lints, formatting, and unit tests using Cargo.
- Verified QMP TCP and Unix argument construction with test assertions.

#### Push Confirmation
- Completed and pushed to remote branch `pyxforge/phase-12-qemu-protocol-hardening`.

---

### [2026-07-17T20:46:00Z] Register Value Diffing in CPU Inspector

#### Files Modified
- [extension/src/extension.ts](file:///C:/Users/Piyush/Documents/antigravity/cool-oppenheimer/extension/src/extension.ts)
- [extension/src/inspectorPanel.ts](file:///C:/Users/Piyush/Documents/antigravity/cool-oppenheimer/extension/src/inspectorPanel.ts)

#### Commit Message
`feat(extension): implement register value diffing in inspector panel`

#### Reason for Change
- Added a `previousRegisterSnapshot` cache map in `extension.ts` to store CPU register values.
- Reset the cache map on debugger startup.
- Computed the `changed` property on the host extension side and passed it inside the `InspectorState` to the Webview.
- Updated the Register interface in the Webview panel to support `changed`.
- Added CSS styles to render changed registers with a distinct persistent left-accent border matching the active theme.

#### Validation Performed
- Ran extension lints and type-checks (`npm run lint`, `npm run check-types`).
- Verified all 9 integration tests pass via `npm test`.

---

### [2026-07-17T20:47:00Z] QEMU GDB Sanity Check

#### Files Modified
- [extension/src/extension.ts](file:///C:/Users/Piyush/Documents/antigravity/cool-oppenheimer/extension/src/extension.ts)

#### Commit Message
`feat(extension): implement QEMU gdbstub sanity checks on debugger stop`

#### Reason for Change
- Declared a `validatedSessions` Set in `extension.ts` to track validated debug sessions.
- Injected a one-time GDB query (`-exec maintenance packet Qqemu.sstepbits`) upon the first GDB stop event of a session.
- Alerted the user via a non-blocking `showWarningMessage` if the GDB session is not attached to a QEMU gdbstub.
- Cleaned up the session ID from `validatedSessions` in `onDidTerminateDebugSession`.

#### Validation Performed
- Ran extension lints and type-checks (`npm run lint`, `npm run check-types`).
- Verified all 9 integration tests pass via `npm test`.

---

### [2026-07-17T20:49:00Z] Typed Command Protocol

#### Files Modified
- [core/src/protocol.rs](file:///C:/Users/Piyush/Documents/antigravity/cool-oppenheimer/core/src/protocol.rs)
- [core/src/main.rs](file:///C:/Users/Piyush/Documents/antigravity/cool-oppenheimer/core/src/main.rs)
- [extension/src/extension.ts](file:///C:/Users/Piyush/Documents/antigravity/cool-oppenheimer/extension/src/extension.ts)

#### Commit Message
`refactor(core): migrate flat request envelope to strongly-typed tagged enum`

#### Reason for Change
- Replaced the flat, dynamic `Request` structure with a strongly-typed `Request` enum containing explicit parameters for each variant.
- Configured JSON-RPC serialization using `#[serde(tag = "cmd", rename_all = "camelCase")]`, converting command tags to camelCase format on the wire (breaking wire change).
- Refactored `handle_request` dispatcher and helper signatures to match directly on the tagged enum, eliminating manual option unwrapping.
- Aligned `extension.ts` calls to send camelCase tags (`qemuStatus`, `listProfiles`, `debugConfig`, `hexDump`).
- Updated unit tests in `core` to match the camelCase deserialization errors.

#### Validation Performed
- Built and ran all core backend unit tests (`cargo test`).
- Ran Clippy and format checks on core code.
- Tested extension using linting, typecheck, and integration tests (`npm test`).

---

### [2026-07-17T20:50:00Z] Registerable Presets Registry

#### Files Modified
- [extension/src/presets.ts](file:///C:/Users/Piyush/Documents/antigravity/cool-oppenheimer/extension/src/presets.ts)
- [extension/src/extension.ts](file:///C:/Users/Piyush/Documents/antigravity/cool-oppenheimer/extension/src/extension.ts)
- [extension/src/test/extension.test.ts](file:///C:/Users/Piyush/Documents/antigravity/cool-oppenheimer/extension/src/test/extension.test.ts)

#### Commit Message
`feat(extension): migrate hardcoded presets to registerable presets pattern`

#### Reason for Change
- Replaced the hardcoded static `PRESETS` array with a modular preset registry (`registry`).
- Implemented `registerPreset(preset)` and `getPresets()` in `presets.ts`.
- Registered all 9 presets at module load time.
- Updated calls in `extension.ts` (preset selection quickpick) and `extension.test.ts` (preset loop validations) to use `getPresets()`.

#### Validation Performed
- Ran extension lints and type-checks (`npm run lint`, `npm run check-types`).
- Verified all 9 integration tests pass via `npm test`.

---

### [2026-07-17T20:51:00Z] Sibling Project Cross-Link & Integration Study

#### Files Modified
- [docs/cross-project/pyxisos-integration.md](file:///C:/Users/Piyush/Documents/antigravity/cool-oppenheimer/docs/cross-project/pyxisos-integration.md)
- [README.md](file:///C:/Users/Piyush/Documents/antigravity/cool-oppenheimer/README.md)

#### Commit Message
`docs: add PyxisOS integration study and cross-project documentation`

#### Reason for Change
- Established `docs/cross-project/` directory.
- Authored a comprehensive `pyxisos-integration.md` guide covering PyxisOS custom target compilation, QEMU and GDB debug lifecycle management, mapping of modules, and step-by-step tutorial configurations.
- Cloned and analysed the sibling `obstinix/PyxisOS` repository (`native/lunar-core` Rust files, custom JSON target specs) before deletion.
- Added a section under `README.md` introducing the sibling project integration link.

#### Validation Performed
- Validated workspace status, checked formatting, and verified all 9 tests pass.
- Synchronized all 11 modified files sequentially into the `main` branch with granular, file-by-file commits under the verified Git identity (`obstinix <obstinix@gmail.com>`).



