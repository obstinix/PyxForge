# Changelog

PyxForge 3.0 is a native rewrite (Go, Fyne and an embedded Neovim) of the 2.x desktop app and VS
Code extension, which stay under `legacy/` until the 3.0 release. Versions follow
[Semantic Versioning](https://semver.org/); pre-releases are tagged `v3.0.0-rc.N`.

## Unreleased (3.0.0-rc.3)

Since 3.0.0-rc.2: the Windows x64 installer, the portable archive and the programs on their own,
each with SHA-256 sums, signed build provenance and an SPDX SBOM, each installed and used in a
clean Windows container before it is attached.

### New

- **An editor first.** A project opens with the editor, the explorer and a panel of Terminal,
  Build, Problems, Git and Log. The register Inspector and the QEMU, GDB and Snapshots tabs come
  with projects that boot in QEMU (a `[qemu]` table) or when asked for. The Welcome page starts
  with New File, Open File and Open Folder.
- **A new identity.** The forge mark (an anvil, the piece being worked and a spark) as the app,
  installer and executable icon, in the About dialog and on the Welcome page, and a start-up
  screen that plays it in 800 ms, still when motion is reduced.
- **Glass by default.** The side panes, the panel, the menu and status bars are translucent over
  a soft glow of the accent; the editor stays solid. Settings > Glass turns it off.
- **Type.** A type scale with nothing under 12 units, and text sizes of their own for the editor
  and the terminal (Settings > Type), kept across restarts.
- **Languages.** Run Current File builds a C or C++ program from all the sources of its folder
  without a `main` of their own, and compiles Java with `javac`, packages included. On Windows,
  PyxForge installs Python 3.14, a Java JDK (Temurin 25) and Go 1.27 itself, beside GCC, GDB,
  NASM, Git and QEMU, each pinned and checked against its publisher's checksum.
- **Run and Debug Current File** for C, C++, Go, Rust, Python, JavaScript, TypeScript, Java,
  shell, PowerShell, batch, Lua, Ruby, Perl, PHP and assembly; debugging under GDB, pdb, Delve,
  node inspect and jdb, stopping at the editor's breakpoints.
- **Breakpoints** with F9 or a click in the gutter, kept per workspace and passed to every
  debugger and to Debug in QEMU.
- **Git**: fetch, pull (merge or rebase), push, Continue and Abort, worktrees, stashes;
  passwords, key passphrases and new host keys asked for in a dialog.
- **Workspace trust**: a project's builds, runs and repository hooks ask first in a folder you
  haven't trusted.
- **Explorer, editor and terminal**: rename, move to the Recycle Bin, Go to Line, Find and
  Replace, Save As, Close All, Open Recent; Clear Terminal and transcripts of exited sessions.
- **Linux**: a `.deb`, a portable archive and an AppImage, installed and run in CI.
- **Updates**: Help > Check for Updates, and an opt-in weekly check that only says so.

### Fixed

- The installer and the portable archive shipped a Neovim that could not start on a Windows
  without Microsoft's Visual C++ runtime (0xC0000135). The runtime now goes beside it, and the
  packager checks every program's DLL imports against what a clean Windows has.
- Doctor reported a program that could not start as found. It now says it is broken, and why.
- The terminal panel's Neovim crashing (0xC0000005) on Windows: closing a session right after a
  resize, and taking a terminal down while a resize was pending. The cause of the rare crash that
  remains is now known (#80).
- MSVC diagnostics for files in folders with non-ASCII names pointed at files that didn't exist:
  the build output is read in the console's code page.
- The theme cards in Settings were drawn over the next section, and long captions ran off the
  edge.
- Esc closes a dialog even when one of its buttons has focus; a Microsoft Store "python3" alias is
  no longer taken for Python; Neovim's own intro screen no longer shows in an empty editor; and
  many smaller fixes.

### Security and supply chain

- Go 1.27.4; govulncheck runs on every push and weekly.
- Every GitHub Action is pinned to a commit; release files carry signed SLSA provenance and an
  SPDX SBOM (`gh attestation verify <file> --repo obstinix/PyxForge`).
- Remote operations are tested against real SSH and HTTP servers with credentials, and parsers
  of untrusted input are fuzzed on each push and nightly.

### Known limitations

- The terminal panel's Neovim can still crash on Windows, rarely, when a program in a terminal
  exits at the moment the panel or window is resized. It is a Neovim bug (#80); the panel offers
  Restart and the editor is not affected.
- The installer and the executables are not code-signed yet (#33): SmartScreen may warn.
- Screen readers can't read the Fyne interface (docs/ACCESSIBILITY.md).
- macOS is tested in CI but not released; 21 of its tests fail (#89).

## 3.0.0-rc.2 (2026-10-20) and earlier

See the [releases page](https://github.com/obstinix/PyxForge/releases) and
[`docs/audit/defect-register.md`](docs/audit/defect-register.md).
