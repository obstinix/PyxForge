# Phase 1 implementation progress

Checkpoint for the defect remediation work; the defects themselves are in
[defect-register.md](defect-register.md). Updated at each milestone.

## Repository

- Work branch: `integration`, published with `git push origin integration:main` (a
  fast-forward of `main`; no pull requests, no force pushes).
- Phase 1 started from `cec2934` (Phase 0 defect map) on top of `origin/main` `f3e0ee0`.
- The owner's own change to `examples/boot-sector/boot.asm` stays unstaged in the working tree
  and is in no commit.

## Done

| Step | Item | Commit |
|---|---|---|
| 1 | Repository state and Phase 0 findings checked; baseline vet clean, all tests pass | — |
| 2 | Git LFS staging regression (PXF-DEF-022) | 4a4fdca |
| 3 | Binary files never rewritten on save (PXF-DEF-011) | de10543 |
| 4 | Memory growth from the region accent (PXF-DEF-002), wrong tab after a close (PXF-DEF-024) | 5e598d0 |
| — | Neovim's children outlived Close, flaky terminal tests (PXF-DEF-025) | f2ad857 |
| — | Toast-expiry race in shell tests (PXF-DEF-026) | d9b4449 |
| — | Close ends what Neovim leaves behind (PXF-DEF-025, second half) | cc20c81 |
| 5 | Editor focus on open, explorer reopen, changed-on-disk dialog (PXF-DEF-005, 027, 012) | bef5fe6 |
| — | Binary check on Neovim 0.9 (`vim.loop`), CI failure on a2a1307 (PXF-DEF-011) | 7ef2408 |
| 6 | Keyboard policy K2; Open File, Save As, Close All (PXF-DEF-010, 009, partly) | a98ae51 |
| 7 | Esc and Enter in every dialog (PXF-DEF-021) | 25dc08d |
| 7 | Window centred and kept on screen; resizable explorer, dock, inspector (PXF-DEF-008, 007, 016) | 3c563a8 |
| 7 | Notifications wrap; errors stay until dismissed (PXF-DEF-006 partly, 021) | 17703d4 |
| 9 | Terminal output kept after the shell exits, statuses named; process tree on Linux (PXF-DEF-015, 025) | 8da1538 |
| 6 | New File (untitled), Find and Replace (PXF-DEF-009, 010) | f0ff90d |
| 8 | Menu bar, Open Recent, tooltips (PXF-DEF-009, 018, 019, 020) | 3fff970 |
| 8 | Menus on PyxForge's overlay material; Open Recent in the palette (PXF-DEF-009, 020) | 7724b6c |
| — | Status-bar race in `TestEditorTabsFollowNeovimBuffers` (test only) | 6f93fd0 |
| — | The windowed build opened a console window for every program it ran (PXF-DEF-028) | 22d587a |
| — | Clipboard through PyxForge on Neovim 0.9; CI failure on 2112dbd (PXF-DEF-029) | 5aefb10 |
| 10 | Git: Initialize (asks first), Fetch/Pull/Push, Publish, Add Remote, failures explained, conflicts and Abort, Git menu; D14 for the network (PXF-DEF-030) | 8991b91 |
| 11 | Set Up Machine for projects without [qemu]; Run explains a broken pyxforge.toml; QEMU command line one option to a line (PXF-DEF-023, 006) | 40c5dc1 |
| 12 | Neovim's history (ShaDa) per workspace (PXF-DEF-001, history part) | bb8ff6a |
| 12 | Run Current File: interpreters, runners and compilers per language, in a Terminal session; Doctor lists them (PXF-FEAT-003) | 40dcd72 |
| 13 | Narrow panels: terminal exit strip, Git bar, editor empty state (PXF-DEF-006) | 8cc6e07 |
| 12 | Debug Current File: GDB (C, C++), pdb, node inspect, Delve in a Terminal session; GDB without paging or the debuginfod question (PXF-FEAT-004, first part) | 9caac91 |

## Next

1. Debugging beyond GDB on QEMU (FEAT-004), language tooling (FEAT-002); the DEF-001 startup
   report needs the owner's steps.

## Tests

Gate on `9caac91`'s tree (Windows 11, Go 1.27.1, Neovim 0.12.6, Git 2.55, git-lfs 3.7.1,
QEMU 11.1, GDB 17.2): forbidcheck, gofmt, `go vet ./...` (also `GOOS=linux` for
`./internal/...`, `GOOS=darwin` for proc and neovim), staticcheck and `stylua --check nvim/`
clean; `go test -race ./...` every package ok; `go test ./...`: 275 passed, 0 failed,
1 skipped (Java in `TestRunsForReal`: no JDK installed). Under Linux (WSL Ubuntu 24.04, Neovim 0.9.5, files copied to the Linux disk) all of
`./internal/...` on 5aefb10: 222 passed, 0 failed, 4 skipped (clang, rustc, clangd, git-lfs not
installed); on 8991b91 `./internal/git`, `./internal/ui/shell` and `./internal/proc`: 71 passed, 3
skipped (the Git LFS tests); on 40c5dc1 `./internal/ui/shell`: 52 passed, 1 skipped (Git LFS); all of `./internal/...` on
40dcd72's tree: 245 passed, 0 failed, 10 skipped (clang, rustc, clangd, git-lfs, g++, node, JDK
not installed there). CI (GitHub Actions: Ubuntu, Windows, macOS, Lua) passed on 7ef2408, 1c29865 and 25dc08d. It
failed on a2a1307 (Neovim 0.9 has no `vim.uv`, fixed in 7ef2408), and on 5844bee and a01a361:
the window clamp of 3c563a8 read the CI runner's 1024x768 screen in TestWorkspaceStateRestores
(fixed by keeping test windows off the real screen); and on 2112dbd, Ubuntu job: the Edit
menu's Copy needed a clipboard tool with Neovim 0.9 (DEF-029, fixed in 5aefb10). CI passed on
48a7fe8, 9aa65c9 and 9ddf2ee. On 4093a46 both Go jobs passed; the macOS compile check was
cancelled by GitHub ("The job was not acquired by Runner of type hosted even after multiple
attempts": macOS runner capacity), so that commit has no macOS result.

- Known intermittent failures: none open. The terminal tests (DEF-025) passed 20 of 20
  repeated runs after cc20c81; they failed 2 of 6 (`TestTerminalRestart`) and 1 of 10
  (`TestTerminalSessions`) on the baseline.
- Not run here: luacheck (not installed locally; CI runs it), the Linux and macOS CI jobs.
- Skipped by design when a tool is missing: Java (no JDK on this machine), git-lfs tests where
  git-lfs is absent.

GUI checks on the Windows build: tab bars in all three regions, accent moving between them,
close marks; working set over three 20-file explorer cycles (DEF-002); a click on a file then
typing edits it (DEF-005); a change on disk under unsaved edits shows PyxForge's dialog, and
Reload from Disk shows the new text (DEF-012); Ctrl+O, Ctrl+P, Ctrl+Shift+S and Ctrl+W in the
editor (DEF-010), after fixing a crash the first check found; Esc on Save As (DEF-021); dragging
the explorer's edge (DEF-007); the menu bar and File menu, Open Recent and a tooltip (DEF-009,
018, 020), after making the menus opaque. The windowed build (`-H windowsgui`, as released) on a
Git repository: no console windows while starting, opening a file, opening a terminal and showing
the Git tab, against about 70 before 22d587a (DEF-028). The Git tab (DEF-030): Fetch then Pull ↓1,
a diverged Pull explained, the Git menu, a merge conflict with its banner, message, markers and
Commit disabled. The QEMU tab without a machine and the Set Up Machine form (DEF-023). Run Current File
(FEAT-003): a C file compiled and run, a Python script answering its prompt. Measurement workloads and numbers are in the
register.
