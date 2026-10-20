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

## Next

1. **Step 6, next:** application shortcuts (Ctrl+N/O/S/Shift+S/W/Shift+W/F/H/P/Tab), K1
   replaced (DEF-010).
2. Step 7: dialog Esc/Enter, wrapping notifications, window clamp, resizable panels
   (DEF-021, 006, 008, 007, 016).
3. Step 8: menus, New/Open/Save As/Recent, tooltips (DEF-009, 020, 018, 019).
4. Step 9: terminal output after exit, status names (DEF-015, 017).
5. Steps 10–13: Git workflow (Initialize, fetch/pull/push), QEMU/GDB configuration (DEF-023),
   per-workspace ShaDa (DEF-001), toolchain/build/run/debug (FEAT-001…004).

## Tests

Gate on `bef5fe6`'s tree (Windows 11, Go 1.27.1, Neovim 0.12.6, Git 2.55, git-lfs 3.7.1,
QEMU 11.1, GDB 17.2): forbidcheck, gofmt, `go vet ./...` (also `GOOS=linux` for
`./internal/...`, `GOOS=darwin` for proc and neovim), staticcheck and `stylua --check nvim/`
clean; `go test -race ./...` every package ok; `go test ./...` twice: 221 passed, 0 failed,
0 skipped each time.

- Known intermittent failures: none open. The terminal tests (DEF-025) passed 20 of 20
  repeated runs after cc20c81; they failed 2 of 6 (`TestTerminalRestart`) and 1 of 10
  (`TestTerminalSessions`) on the baseline.
- Not run here: luacheck (not installed locally; CI runs it), the Linux and macOS CI jobs.
- Skipped by design when a tool is missing: Java (no JDK on this machine), git-lfs tests where
  git-lfs is absent.

GUI checks on the Windows build: tab bars in all three regions, accent moving between them,
close marks; working set over three 20-file explorer cycles (DEF-002); a click on a file then
typing edits it (DEF-005); a change on disk under unsaved edits shows PyxForge's dialog, and
Reload from Disk shows the new text (DEF-012). Measurement workloads and numbers are in the
register.
