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
| `go test -race ./...` on Windows 11 | The full suite passes, including the real-tool integration tests (Neovim 0.12.5, QEMU 11.1, GDB 17.2, Git 2.55) |
| The same on Ubuntu 24.04 (WSL2) | All pass (Neovim 0.9.5, QEMU 8.2, GDB 15) |
| GitHub Actions | Go on Ubuntu (with Neovim, clangd, QEMU, qemu-img, GDB, NASM) and Windows, macOS compile check, Lua lint, legacy 2.x; release builds kept as artifacts |
| Desktop app on Linux | Launched under Xvfb, file opened in Neovim, Debug in QEMU and Step driven with real key presses, screenshot checked; no process left after SIGTERM |
| Process clean-up | `pyxforge run` killed with `taskkill /F` (Windows) and `kill -9` (Linux): QEMU ended with it |
| Release builds | Two builds of one commit give one SHA-256 on Windows and on Linux |
| `examples/boot-sector` | Builds with NASM, boots in QEMU ("PyxForge boot sector OK"), debugs from 0x7c00 |
| PyxisOS v7 | Built, booted and debugged through PyxForge ([details](../cross-project/pyxisos-integration.md)) |

## Known limitations

- macOS is compile-checked only; Linux is verified on x86-64 Ubuntu.
- Screen readers: Fyne exposes no accessibility API. Every control is reachable by keyboard.
- GDB decodes real-mode code as 32-bit against QEMU's x86-64 register layout; PyxForge decodes it
  itself in the Disasm tab and `pyxforge inspect`.
- Machine snapshots need `snapshots = true` under `[qemu]`, `qemu-img`, and a `boot_image`;
  they are lost when the image is rebuilt (the overlay is made again).
- rustc human-readable and MSVC diagnostics are not parsed yet.
- The integrated terminal is Neovim's `:terminal`; processes started in it end with PyxForge.
- Agents (Phases 6–7) are not started.

The open items, with what finishes each, are in the [roadmap](../ROADMAP.md).
