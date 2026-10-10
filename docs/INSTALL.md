# Installing PyxForge

PyxForge 3.0 is one program, `pyxforge`, that is both the desktop app and a command-line tool.
It drives tools you install yourself: Neovim for editing, an assembler and compiler for building,
QEMU for running, GDB for debugging, Git for version control. Once they are installed, nothing
needs the network; PyxForge never downloads or installs programs on its own.

## Platforms

| Platform | Status | What has been verified |
|---|---|---|
| Windows 11, x86-64 | Supported | Build, the full test suite, the desktop app (editor, terminal, build, Git, Run and Debug in QEMU), process clean-up after a forced exit |
| Linux, x86-64 (Ubuntu 24.04) | Supported | Build, the full test suite in CI and WSL2, the desktop app under X11 (Xvfb), Run and Debug in QEMU, clean-up after `kill -9` |
| macOS | Not supported yet | Compiles and passes `go vet` in CI; never run |

The desktop app needs an OpenGL 2.1 driver (any GPU driver from the last decade; on Windows
servers and VMs install the vendor or Mesa driver).

## Tools PyxForge uses

Only Neovim is needed for the desktop app's editor; everything else enables one feature, and
PyxForge says what is missing when you use that feature. `pyxforge doctor` checks them all.

| Tool | Needed for | Required? |
|---|---|---|
| Neovim 0.9 or newer (0.10+ for the system clipboard, 0.11+ for extra Tree-sitter parsers) | Editing, the Terminal panel | For the editor |
| NASM, a C compiler (GCC or Clang), a linker, Make | Building the profiles in `pyxforge.toml` | What your profiles name |
| QEMU (`qemu-system-x86_64`, also `-i386`, `-arm`) | Run and Debug | For Run and Debug |
| `qemu-img` | Machine snapshots (`snapshots = true`) | Optional |
| GDB (`gdb`, or `gdb-multiarch` for ARM) | Debug | For Debug |
| Git | The Git tab and status bar | Optional |
| clangd, gopls, rust-analyzer, asm-lsp, lua-language-server | Completion and diagnostics in the editor | Optional |
| tree-sitter CLI and a C compiler | `pyxforge setup editor` (extra parsers) | Optional |

### Windows

```powershell
winget install --id Neovim.Neovim --exact
winget install --id NASM.NASM --exact
winget install --id MartinStorsjo.LLVM-MinGW.UCRT --exact     # C compiler, linker, make, clangd
winget install --id BrechtSanders.WinLibs.POSIX.UCRT --exact   # GDB, objdump
winget install --id SoftwareFreedomConservancy.QEMU --exact    # QEMU and qemu-img
winget install --id Git.Git --exact
```

The QEMU installer does not add itself to `PATH`; PyxForge also looks in
`C:\Program Files\qemu`. `pyxforge doctor --commands` prints the commands for whatever is still
missing on your machine, for you to review and run.

### Ubuntu and Debian

```sh
sudo apt-get install neovim nasm gcc binutils make clangd git \
  qemu-system-x86 qemu-utils gdb gdb-multiarch
```

Ubuntu 24.04 ships Neovim 0.9.5, which works; the system clipboard needs 0.10 or newer from
neovim.io.

## Build from source

Requirements: Go 1.27.1 or newer and a C compiler for cgo (Fyne draws with OpenGL).

On Linux, also install Fyne's build dependencies:

```sh
sudo apt-get install gcc libgl1-mesa-dev libx11-dev xorg-dev libwayland-dev libxkbcommon-dev
```

Then:

```sh
git clone https://github.com/obstinix/PyxForge.git
cd PyxForge
go build -o pyxforge ./cmd/pyxforge     # pyxforge.exe on Windows
./pyxforge doctor
./pyxforge                              # opens the desktop app on the current folder
```

### Release builds

```sh
go run ./tools/release -version 3.0.0
```

builds `dist/pyxforge-<version>-<os>-<arch>` with `-trimpath` and an empty build ID, builds it a
second time to check the bytes match, and writes an archive (zip on Windows, tar.gz elsewhere)
with the binary, README, LICENSE and this guide, and a `SHA256SUMS` file. Fyne needs cgo, so each
platform is built on that platform; CI builds Windows and Linux archives on every push.

## First run

```sh
pyxforge doctor                 # which tools were found, and how to get the rest
pyxforge doctor --commands      # the install commands for this platform, to review and run
pyxforge setup editor           # once, online: the pinned Tree-sitter plugin and parsers
pyxforge path/to/project        # the desktop app on a project
```

`pyxforge setup editor` is the only command that uses the network. Without it the editor still
works, with Neovim's own parsers (C, Lua, Markdown, Vimscript).

## Where PyxForge keeps things

PyxForge writes nothing into your projects except what your build tools write. Its own files:

| What | Where |
|---|---|
| Workspace layout and open files | user config folder `PyxForge/workspaces` |
| Diagnostic snapshots (at most 50 per workspace) | user config folder `PyxForge/snapshots/<workspace>` |
| QEMU snapshot overlays (machine snapshots) | user cache folder `PyxForge/qemu/<workspace>` |
| PyxForge's Neovim configuration (unpacked from the binary) | user cache folder `PyxForge/nvim-runtime` |
| Neovim's log | user cache folder `PyxForge/nvim.log` |
| Neovim's plugins, parsers, undo history, and your `user.lua` | Neovim's folders for `NVIM_APPNAME=pyxforge` (`:echo stdpath("config")` shows one) |
| Theme and preferences | Fyne's preferences for `io.github.obstinix.pyxforge` |

The user config folder is `%AppData%` on Windows and `~/.config` on Linux; the user cache folder
is `%LocalAppData%` and `~/.cache`. Your own Neovim configuration is never read or changed.

To uninstall, delete the binary and the `PyxForge` and `pyxforge` folders above.

## Troubleshooting

| Symptom | Cause and fix |
|---|---|
| "Editor unavailable" or files open as placeholders | Neovim is not on `PATH`: install it, then `pyxforge doctor` |
| The window does not open on Linux | No OpenGL: install Mesa (`libgl1-mesa-dri`); under WSL use WSLg |
| "qemu-system-x86_64 is not installed" on Windows | Install QEMU; PyxForge finds it in `C:\Program Files\qemu` even off `PATH` |
| Debug says the GDB port is in use | Another QEMU holds `gdb_port` (1234 by default): stop it, or set another port under `[qemu.debug]` |
| GDB disassembles 16-bit code as 32-bit | GDB uses QEMU's x86-64 register layout; the Disasm tab and `pyxforge inspect --disasm` decode real mode correctly |
| "no block device can store vmstate" | Machine snapshots need `snapshots = true` under `[qemu]` and `qemu-img` |
| A build error has no file | Linker and tool messages without a source line are listed under the tool's name; select one to see the Build output |

## Safety

Opening a project runs nothing from it: PyxForge reads `pyxforge.toml`, lists files and runs
`git status` with Git's file-system-monitor hook disabled. Language servers start for files you
open. Build, Run and Debug run the tools `pyxforge.toml` names, with their arguments passed
directly (never through a shell), only when you ask; review that file in a project you did not
write. Builds cannot create their output folder outside the project.
