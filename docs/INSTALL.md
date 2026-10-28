# Installing PyxForge

PyxForge 3.0 is one program that is both the desktop app and a command-line tool. It drives
tools you install yourself: an assembler and compiler for building, QEMU for running, GDB for
debugging, Git for version control. Once they are installed, nothing needs the network;
PyxForge never downloads or installs programs on its own.

## Windows: the installer

Download `PyxForge-Setup-<version>.exe` from the
[releases page](https://github.com/obstinix/PyxForge/releases) and check it against
`SHA256SUMS.txt` from the same release:

```powershell
Get-FileHash .\PyxForge-Setup-3.0.0.exe -Algorithm SHA256
```

Run it. It installs for your account only, without administrator rights, into
`%LOCALAPPDATA%\Programs\PyxForge` (choose "all users" on the first page to install into
`Program Files` instead; that needs an administrator). It adds a Start menu entry and, if you
tick it, a desktop shortcut. Setup downloads nothing and runs nothing but PyxForge itself, and
only if you tick "Launch PyxForge" at the end.

The installer is not code-signed yet, so Windows SmartScreen may say it "protected your PC":
choose **More info**, check the publisher and file name, then **Run anyway** if the checksum
matched.

What is installed:

| File | What it is |
|---|---|
| `PyxForge.exe` | The desktop app. The Start menu entry and desktop shortcut run it; it opens no console window |
| `pyxforge.com` | The same program as a console app, for `pyxforge doctor`, `pyxforge build` and the other commands in a terminal (Windows runs `.com` before `.exe` when you type `pyxforge`) |
| `nvim\` | Neovim 0.12.6, the official Windows build, unchanged, which PyxForge's editor uses |
| `LICENSE.txt`, `THIRD-PARTY-NOTICES.txt` | PyxForge's license and the licenses of everything it includes |

The Start menu entry opens `Documents\PyxForge Projects`, where setup copies the
`boot-sector` example the first time. **File: Open Folder in New Window…** in the command
palette (Ctrl+Shift+P) opens any other folder.

Not bundled: Git, QEMU, GDB, NASM, compilers and language servers. They are large, separately
licensed and updated, so PyxForge uses the ones you install (below) and **Tools: Doctor…** in
the command palette shows which it found, what each is for, how to get it, and lets you choose
an executable by hand. Each one enables its own feature; PyxForge starts and edits without any
of them.

To use the command line from any terminal, add the install folder to your `PATH` (Settings >
System > About > Advanced system settings > Environment Variables), or run
`"%LOCALAPPDATA%\Programs\PyxForge\pyxforge.com" doctor`.

**Upgrading:** run the newer installer. It finds the existing installation, closes a running
PyxForge (asking it to save first), and replaces the program files. Settings, tool paths, editor
data and projects are kept.

**Uninstalling:** Settings > Apps > PyxForge > Uninstall. It removes the program files and
shortcuts, then asks whether to delete your settings, caches and editor data too (the answer
defaults to No). Projects are never removed.

### Portable archive

`PyxForge-Portable-<version>.zip` holds the same files plus `examples\boot-sector` and
`PORTABLE.txt`, a one-page guide. Unzip it anywhere and run `PyxForge.exe`; there is nothing to
install or uninstall. Settings still go to
your profile (the folders under [Where PyxForge keeps things](#where-pyxforge-keeps-things)), so
delete those as well to remove every trace.

## Platforms

| Platform | Status | What has been verified |
|---|---|---|
| Windows 11, x86-64 | Supported | Build, the full test suite, the desktop app (editor, terminal, build, Git, Run and Debug in QEMU), process clean-up after a forced exit |
| Linux, x86-64 (Ubuntu 24.04) | Supported | Build, the full test suite in CI and WSL2, the desktop app under X11 (Xvfb), Run and Debug in QEMU, clean-up after `kill -9` |
| macOS | Not supported yet | Compiles and passes `go vet` in CI; never run |

The desktop app needs Windows 10 version 1809 or later (x64) or Linux, and an OpenGL 2.1
driver: any GPU driver from the last decade. Windows' basic display driver in some virtual
machines and remote sessions only offers OpenGL 1.1; install the GPU vendor's driver there, or
Mesa's `opengl32.dll` (the
[mesa-dist-win](https://github.com/pal1000/mesa-dist-win) build) beside `PyxForge.exe`.

## Tools PyxForge uses

Only Neovim is needed for the desktop app's editor, and the Windows installer includes it;
everything else enables one feature, and PyxForge says what is missing when you use that
feature. `pyxforge doctor` and **Tools: Doctor…** check them all.

PyxForge looks for each tool in this order: the path chosen in **Tools: Doctor…**, a copy
shipped with PyxForge (Neovim in the Windows installer), `PATH`, then the folders the tool's own
installer uses (`C:\Program Files\qemu`, `C:\Program Files\Git\cmd`). Paths chosen in Doctor
are saved per user in `PyxForge\tools.json` in the user config folder, never in a project.

| Tool | Needed for | Required? |
|---|---|---|
| Neovim 0.9 or newer (0.11+ for extra Tree-sitter parsers) | Editing, the Terminal panel | For the editor |
| NASM, a C compiler (GCC or Clang), a linker, Make | Building the profiles in `pyxforge.toml` | What your profiles name |
| QEMU (`qemu-system-x86_64`, also `-i386`, `-arm`) | Run and Debug | For Run and Debug |
| `qemu-img` | Machine snapshots (`snapshots = true`) | Optional |
| GDB (`gdb`, or `gdb-multiarch` for ARM) | Debug | For Debug |
| Git | The Git tab and status bar | Optional |
| clangd, gopls, rust-analyzer, asm-lsp, lua-language-server | Completion and diagnostics in the editor | Optional |
| tree-sitter CLI and a C compiler | `pyxforge setup editor` (extra parsers) | Optional |
| Python, Node.js, Go, Rust (Cargo or rustc), a C or C++ compiler, a JDK 11+, Deno or tsx, PowerShell, Lua, Ruby, Perl, PHP | Run Current File (Ctrl+Shift+F9), each for its own language | Optional |

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

Ubuntu 24.04 ships Neovim 0.9.5, which works, the system clipboard included: Neovim copies and
pastes through PyxForge, so no xclip, xsel or wl-copy is needed.

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
./pyxforge                              # opens Welcome with no workspace selected
```

### Windows installer and portable archive

On Windows, with Go, a C compiler for cgo, Git and [Inno Setup 6](https://jrsoftware.org/isinfo.php)
(`winget install JRSoftware.InnoSetup`):

```powershell
go run ./tools/winpkg -version 3.0.0
```

writes `dist\windows\PyxForge-Setup-3.0.0.exe`, `PyxForge-Portable-3.0.0.zip`,
`SHA256SUMS.txt` and the staged folder `PyxForge-3.0.0`. It:

- builds `PyxForge.exe` (no console) and `pyxforge.com`, each twice, and stops unless both
  builds are byte-identical;
- downloads the official Neovim Windows build once into the user cache, and stops unless its
  SHA-256 is the pinned one;
- adds the icon and version information (with go-winres) and every third-party license;
- runs the checks: the subsystems, the version information, `pyxforge.com version`, the
  bundled Neovim's version, and that the archive holds every required file.

`-no-installer` skips Inno Setup. The installer itself is not byte-reproducible (Inno Setup
stamps the build time); the programs and the portable archive are. The
`.github/workflows/release-windows.yml` workflow runs the same build on a version tag and
attaches the files to a draft release for a maintainer to publish. A draft's files appear
only to maintainers, on the Releases page; the tag's page shows them once the release is
published. To build an existing tag again and attach or check its files, run the workflow by
hand (Actions > Windows release > Run workflow) with that tag. It uploads only to a release
with none of the files, leaves a complete one alone, stops on a partial one rather than
replacing anything, and then downloads the attached files and checks them against
`SHA256SUMS.txt`; the run's summary lists them.

### Release builds

```sh
go run ./tools/release -version 3.0.0
```

builds `dist/pyxforge-<version>-<os>-<arch>` with `-trimpath` and an empty build ID, builds it a
second time to check the bytes match, and writes an archive (zip on Windows, tar.gz elsewhere)
with the binary, README, LICENSE and this guide, reads the archive back to check it holds that
binary, and writes a `SHA256SUMS` file (`sha256sum -c` checks it). The version must be a
semantic version such as `3.0.0` or `3.1.0-rc.1`. If any step fails, the files it wrote are
removed. Fyne needs cgo, so each platform is built on that platform; CI builds Windows and Linux
archives on every push and keeps them as workflow artifacts. Nothing is published.

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
| The editor's history per workspace (Neovim's ShaDa: jumplist, marks, old files, searches) | user config folder `PyxForge/workspaces/<workspace>.shada` |
| Diagnostic snapshots (at most 50 per workspace) | user config folder `PyxForge/snapshots/<workspace>` |
| QEMU snapshot overlays (machine snapshots) | user cache folder `PyxForge/qemu/<workspace>` |
| PyxForge's Neovim configuration (unpacked from the binary) | user cache folder `PyxForge/nvim-runtime` |
| Neovim's log | user cache folder `PyxForge/nvim.log` |
| Neovim's plugins, parsers, undo history, and your `user.lua` | Neovim's folders for `NVIM_APPNAME=pyxforge` (`:echo stdpath("config")` shows one) |
| Theme and preferences | Fyne's preferences for `io.github.obstinix.pyxforge` (`%AppData%\fyne\io.github.obstinix.pyxforge` on Windows) |
| Tool paths chosen in Doctor | user config folder `PyxForge/tools.json` |
| Projects started from the Start menu (Windows installer) | `Documents\PyxForge Projects` |

The user config folder is `%AppData%` on Windows and `~/.config` on Linux; the user cache folder
is `%LocalAppData%` and `~/.cache`. Your own Neovim configuration is never read or changed.

On Windows the editor's Neovim data is in `%LocalAppData%\pyxforge-data` (its configuration
folder `%LocalAppData%\pyxforge` stays unused, since PyxForge passes its configuration
explicitly). PyxForge also leaves an empty `%LocalAppData%\fyne`, and its Neovim version check
an empty `nvim.log` in `%LocalAppData%\nvim-data`, which belongs to any Neovim you use on your
own and is left alone. The installer's uninstaller offers to delete PyxForge's folders, never
your projects. Without the installer, delete the program and the folders above.

## Troubleshooting

| Symptom | Cause and fix |
|---|---|
| "Editor unavailable" or files open as placeholders | Neovim was not found: **Tools: Doctor…** shows where PyxForge looked; install Neovim or choose its `nvim.exe` there. The Windows installer includes it |
| The window does not open on Windows, or closes at once | No OpenGL 2.1 driver (typical of virtual machines and some remote sessions): install the GPU vendor's driver, or put Mesa's `opengl32.dll` beside `PyxForge.exe`. `pyxforge.com` in a terminal still works and prints errors |
| SmartScreen blocks the installer | It is not code-signed yet: **More info**, then **Run anyway** after checking `SHA256SUMS.txt` |
| A tool is installed but Doctor says missing | It is not on `PATH` and not in its usual folder: **Choose…** in Doctor and pick its `.exe` |
| A path chosen in Doctor stopped working | The program moved or was uninstalled; Doctor says so. **Use Default** goes back to finding it automatically |
| `pyxforge` in a terminal does nothing | You ran `PyxForge.exe`, which has no console: run `pyxforge.com` (or put the install folder on `PATH` and type `pyxforge`) |
| The window does not open on Linux | No OpenGL: install Mesa (`libgl1-mesa-dri`); under WSL use WSLg |
| "qemu-system-x86_64 is not installed" on Windows | Install QEMU; PyxForge finds it in `C:\Program Files\qemu` even off `PATH` |
| Debug says the GDB port is in use | Another QEMU holds `gdb_port` (1234 by default): stop it, or set another port under `[qemu.debug]` |
| GDB disassembles 16-bit code as 32-bit | GDB uses QEMU's x86-64 register layout; the Disasm tab and `pyxforge inspect --disasm` decode real mode correctly |
| "no block device can store vmstate" | Machine snapshots need `snapshots = true` under `[qemu]` and `qemu-img` |
| A build error has no file | Linker and tool messages without a source line are listed under the tool's name; select one to see the Build output |

## Safety

Opening a project runs nothing from it: PyxForge reads `pyxforge.toml`, lists files and runs
`git status` with Git's file-system-monitor hook and the repository's own filters disabled
(decision D14). Language servers start for files you
open. Build, Run and Debug run the tools `pyxforge.toml` names, with their arguments passed
directly (never through a shell), only when you ask; review that file in a project you did not
write. Builds cannot create their output folder outside the project.
