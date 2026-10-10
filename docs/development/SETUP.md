# Development setup

What a machine needs to build PyxForge 3.0, run its tests, and use the tools it drives. PyxForge
itself never installs these; it detects them and says when one is missing. For installing and
running PyxForge as a user, see [`../INSTALL.md`](../INSTALL.md).

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
- **Linux (Debian, Ubuntu):** `sudo apt-get install gcc libgl1-mesa-dev libx11-dev xorg-dev libwayland-dev libxkbcommon-dev`
  (GLFW builds both its X11 and Wayland backends).
  On Windows, WSL2 Ubuntu is the supported Linux path.
- **macOS:** Xcode command-line tools. macOS is compile-checked, not released (decision D5).

## Tools PyxForge drives

| Tool | Used for | Phase |
|---|---|---|
| Neovim 0.9 or newer (0.10+ clipboard, 0.11+ for `pyxforge setup editor` parsers) | the editor and the Terminal panel (`nvim --embed`, `ext_linegrid`) | 3 |
| StyLua, Luacheck | formatting and linting PyxForge's Lua configuration | 3 |
| Language servers: gopls, rust-analyzer, clangd, lua-language-server, marksman, taplo | editor intelligence; each is optional | 3 |
| NASM, gcc or clang, GNU ld or lld | building boot sectors, kernels and bare-metal code | 4 |
| Git | the Git tab: status, diffs, stage, commit, branches, stash | 4 |
| QEMU (`qemu-system-x86_64`, `qemu-system-i386`; `qemu-system-arm` for embedded targets) | Run and Debug, QMP control, the monitor | 5 |
| `qemu-img` | machine snapshots (`snapshots = true`) | 5 |
| GDB with the target's architecture (`gdb-multiarch` for ARM) | debugging through GDB/MI | 5 |
| objdump or llvm-objdump | optional: PyxForge disassembles x86 itself (`golang.org/x/arch`) | 5 |

On Windows the QEMU installer (`winget install --id SoftwareFreedomConservancy.QEMU --exact`)
needs administrator approval and does not add itself to `PATH`; PyxForge looks in
`C:\Program Files\qemu` as well.

## Checks

The same as CI, in this order; `.githooks/pre-commit` runs the fast ones on every commit
(`git config core.hooksPath .githooks`).

```sh
go run ./tools/forbidcheck
gofmt -l .
go vet ./...
staticcheck ./...
go test -race ./...                         # the integration tests skip tools that are missing
(cd nvim && stylua --check . && luacheck .)
go run ./tools/release                      # optional: a reproducible release build in dist/
```

The integration tests run real Neovim, Git, QEMU, `qemu-img`, GDB and NASM when they are
installed and report a skip, not a pass, when they are not (`go test -v` shows which).

## The 2.x stack in `legacy/`

| Part | Needs | Checks |
|---|---|---|
| Rust core (`legacy/core`) | Rust stable with rustfmt and Clippy | `cargo fmt --check`, `cargo clippy --all-targets --all-features -- -D warnings`, `cargo test` |
| VS Code extension (`legacy/extension`) | Node.js 22 or newer | `npm ci`, `npm run check-types`, `npm run lint`, `npm run compile`, `npm test` |

The Node.js requirement applies only to `legacy/`; PyxForge 3.0 contains no JavaScript.
