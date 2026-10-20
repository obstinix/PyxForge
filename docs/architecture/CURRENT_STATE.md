# PyxForge current state (verified 2026-10-11)

PyxForge 3.0 on `main`: one Go program, `pyxforge`, that is a Fyne desktop app and a command-line
tool, with a real Neovim embedded as the editor. The 2.x stack (Rust core, VS Code extension,
Tauri shell) is in `legacy/` until parity; what it did is recorded in
[`../archive/2.x/CURRENT_STATE-2026-10-08.md`](../archive/2.x/CURRENT_STATE-2026-10-08.md).

## Architecture

| Layer | Packages | Talks to |
|---|---|---|
| Entry point | `cmd/pyxforge` | `internal/cli` (commands), `internal/ui/shell` (desktop app) |
| Command line | `internal/cli` | the engine packages below |
| Desktop app | `internal/ui/shell` (workbench, panels, inspector), `internal/ui/editor` (Neovim grid view), `internal/ui/explorer`, `internal/ui/commandpalette`, `internal/ui/kit`, `internal/ui/theme`, `internal/ui/icons` | Fyne; the engine packages |
| Editor | `internal/neovim` (process, msgpack-RPC, grid, events), `nvim/` (the Lua configuration, embedded) | Neovim `--embed` |
| Project | `internal/config` (`pyxforge.toml`), `internal/workspace` (discovery, state, path containment) | the file system |
| Build | `internal/build` (profiles, runner, diagnostics parsers) | the tools profiles name |
| Run and debug | `internal/qemu` (launch, QMP, machine snapshots), `internal/gdb` (GDB/MI), `internal/inspect` (hex, boot sector, map, x86 disassembly, ELF), `internal/snapshot` (diagnostic snapshots) | QEMU, `qemu-img`, GDB |
| Source control | `internal/git` | the `git` command |
| Platform | `internal/toolchain` (tool detection), `internal/proc` (child process containment), `internal/buildinfo` | the OS |

Every external tool runs with an argument array, never through a shell. The application has no
web technology; `tools/forbidcheck` fails the build if any appears outside `docs/` and `legacy/`.

## Verified

| Check | Result |
|---|---|
| `go test -race ./...` and `go test ./...` on Windows 11 | The full suite passes in both modes (275 tests on 2026-10-20), including the real-tool integration tests (Neovim 0.12.6, QEMU 11.1, GDB 17.2, Git 2.55, git-lfs 3.7, rustc 1.97, Clang 23, Python 3.11, Node 22, Go 1.27); 1 skip: Java, no JDK installed |
| The same on Ubuntu 24.04 (WSL2), `./internal/...` | All pass (Neovim 0.9.5, QEMU 8.2, GDB 15, Git 2.43): 245 passed on 2026-10-20; 10 skip, for tools not installed there: rustc, Clang, clangd, git-lfs, g++, Node.js, a JDK |
| GitHub Actions | Go on Ubuntu (with Neovim, clangd, QEMU, qemu-img, GDB, NASM) and Windows, macOS compile check, Lua lint, legacy 2.x; release builds kept as artifacts |
| Desktop app on Linux | Launched under Xvfb, file opened in Neovim, Debug in QEMU and Step driven with real key presses, screenshot checked; no process left after SIGTERM |
| Process clean-up | `pyxforge run` killed with `taskkill /F` (Windows) and `kill -9` (Linux): QEMU ended with it |
| Release builds | Two builds of one commit give one SHA-256 on Windows and on Linux |
| `examples/boot-sector` | Builds with NASM, boots in QEMU ("PyxForge boot sector OK"), debugs from 0x7c00 |
| PyxisOS v7 | Built, booted and debugged through PyxForge, the desktop Debug flow included, on Windows; booted and debugged on Linux from a kernel built on Windows ([details](../cross-project/pyxisos-integration.md)) |
| Shutdown | Closing the window during a build or while QEMU starts leaves no tool running (`TestShutdownStopsARunningBuild`, `TestShutdownWhileQEMUStarts`, both platforms) |
| Windows installer (2026-10-20, on the development machine, not a clean one) | Silent per-user install into a folder with spaces and non-ASCII letters; Start menu and desktop shortcuts start the windowed program on `Documents\PyxForge Projects`, no console; the editor runs on the bundled Neovim with `PATH` cut to `System32`; Doctor lists found and missing tools; upgrade over a running copy closes it and keeps settings and projects; silent uninstall removes the program, shortcuts and registration and keeps settings and projects; the portable zip runs from a spaced path; no network connection during startup |

## Known limitations

- macOS is compile-checked only; Linux is verified on x86-64 Ubuntu.
- The Windows installer and programs are not code-signed, so SmartScreen warns. It has been
  tested on a development machine, not on a clean Windows installation or virtual machine; the
  uninstaller's "delete settings" answer was not exercised there (it would have removed the
  developer's own settings).
- Screen readers: Fyne exposes no accessibility API. Every control is reachable by keyboard.
- GDB decodes real-mode code as 32-bit against QEMU's x86-64 register layout; PyxForge decodes it
  itself in the Disasm tab and `pyxforge inspect`.
- Machine snapshots need `snapshots = true` under `[qemu]`, `qemu-img`, and a `boot_image`;
  they are lost when the image is rebuilt (the overlay is made again).
- MSVC-format diagnostics are tested against Clang's `-fdiagnostics-format=msvc` output and
  documented `cl`, `link` and `ml` lines; Microsoft's own compiler has not been run.
- The integrated terminal is Neovim's `:terminal`; processes started in it end with PyxForge.
- The Git tab lists a submodule when it is at another commit, not changes inside its working
  tree. In a repository whose own configuration names a filter, merge driver or signing
  program, staging, committing, switching and stashing are left to a terminal (decision D14),
  unless the user's global or system configuration sets the same key to the same value, as
  `git lfs install --local` does; fetch, pull and push likewise refuse the repository's own
  ssh, askpass, proxy, credential, upload-pack and receive-pack programs and `protocol.allow`.
- Fetch, pull and push are tested against remotes on the same machine, with and without Git
  LFS (`TestFetchPullPush`, `TestGitLFSRoundTrip`, `TestGitRemoteWorkflow`); network and
  credential failures are classified from Git's messages, checked against messages from Git
  2.43–2.55, an unreachable port and GitHub's answer for a missing repository. No push to a
  hosted server with credentials is part of the automated tests. Pull is `--ff-only`.
- Agents (Phases 6–7) are not started.

The open items, with what finishes each, are in the [roadmap](../ROADMAP.md).
