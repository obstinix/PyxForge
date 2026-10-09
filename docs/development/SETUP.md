# Development setup

What a machine needs to build PyxForge 3.0, run its tests, and use the tools it drives. PyxForge
itself never installs these; it detects them and says when one is missing.

## To build and test the application

| Tool | Version | Why |
|---|---|---|
| Go | 1.27.1 or newer (`go.mod`) | the application |
| C compiler for cgo | any recent gcc or clang | Fyne's OpenGL driver |
| staticcheck | 2026.2.1 (as in CI) | static analysis gate |
| gitleaks | v8 | optional; the pre-commit hook scans staged changes with it |

Platform notes:

- **Windows:** install a MinGW-w64 toolchain (for example LLVM-MinGW or WinLibs) and put its
  `bin` folder on `PATH`, so `gcc` resolves.
- **Linux (Debian, Ubuntu):** `sudo apt-get install gcc libgl1-mesa-dev xorg-dev libxkbcommon-dev`.
  On Windows, WSL2 Ubuntu is the supported Linux path.
- **macOS:** Xcode command-line tools. macOS is compile-checked, not released (decision D5).

## Tools PyxForge drives

| Tool | Used for | Phase |
|---|---|---|
| Neovim 0.12 or newer | the editor (`nvim --embed`, `ext_linegrid`) | 3 |
| StyLua, Luacheck | formatting and linting PyxForge's Lua configuration | 3 |
| Language servers: gopls, rust-analyzer, clangd, lua-language-server, marksman, taplo | editor intelligence; each is optional | 3 |
| NASM, gcc or clang, GNU ld or lld | building boot sectors, kernels and bare-metal code | 4 |
| Git | repository status, diffs, worktrees | 4 |
| QEMU (`qemu-system-x86_64`, `qemu-system-i386`; `qemu-system-arm` for embedded targets) | running and snapshotting targets, QMP control | 5 |
| GDB with the target's architecture (`gdb-multiarch` for ARM) | debugging through GDB/MI | 5 |
| objdump or llvm-objdump | disassembly | 5 |

On Windows the QEMU installer (`winget install --id SoftwareFreedomConservancy.QEMU --exact`)
needs administrator approval and does not add itself to `PATH`; add `C:\Program Files\qemu`.

## The 2.x stack in `legacy/`

| Part | Needs | Checks |
|---|---|---|
| Rust core (`legacy/core`) | Rust stable with rustfmt and Clippy | `cargo fmt --check`, `cargo clippy --all-targets --all-features -- -D warnings`, `cargo test` |
| VS Code extension (`legacy/extension`) | Node.js 22 or newer | `npm ci`, `npm run check-types`, `npm run lint`, `npm run compile`, `npm test` |

The Node.js requirement applies only to `legacy/`; PyxForge 3.0 contains no JavaScript.
