# Phase 7 results: stability (owner review F1, F2, F4, F5, F6)

2026-11-28, `main` from `7c9d921` to the Phase 7 gate commit. The causes found before any
change are in [`phase-07-preflight.md`](phase-07-preflight.md). Timings are from Windows 11 on
an Intel Core Ultra 9 275HX unless a line says otherwise.

## What changed, by finding

| Finding | Root cause | Fix | Proof |
|---|---|---|---|
| **F2** Explorer rows ignore clicks | The row took right-clicks only, so Fyne's GL driver gave it every click and dropped the left ones | Rows take the click: one selects (and opens or closes a folder), a double click or Enter opens | `TestRowsTakeClicks` aims through the driver's own hit test; it fails without the fix |
| **F5** The Graph freezes the app | Each node was filled through a rasteriser the size of the window: 7 min a frame on a 3,102-node graph | PyxForge's own anti-aliased rasteriser touching only covered pixels; frames drawn on a worker; folders collapse above a node cap (2,000); layout stops after 20 s | `TestDrawBudget`, `TestDrawNeverStalls`, `TestCollapseAboveCap` |
| **F4** Large files crash the app | Text kept as runes (4 bytes a character), coloured on the UI thread in one pass; gutter marks redrew the editor once per line | Lines are slices of the file's bytes; colouring on a worker above 256 KB, none above 2 MB; long lines cut at 10,000 characters; read-only above 50 MB; a paged hex view for binaries and anything above 512 MB; marks set in one redraw | `TestLargeFiles`, `TestLinesShareTheFile`, `TestLongLineIsCut`, `TestGigabyteReadsOnePage` |
| **F1** Freeze on launch, "opens a whole drive" | Not reproduced. Walks ran on the UI thread (Go to File, Explorer folders); nothing guarded a drive root or the home folder | Both walks on workers; a drive root, the file system's root or the home folder is asked about first, Cancel by default; Reopen Last Workspace is a setting, off; crash logs; a stall watchdog | `TestQuickOpenListsOffTheUIThread`, `TestSlowFolderShowsLoading`, `TestBroadFolderAsksAndEnterCancels`, `TestGoContainsAPanic` |
| **F6** Start screen | Thin Welcome tab; recent rows overlapped the checkbox when the list changed | Pins, Remove, Locate… for a folder that is gone, folder drop, a footer; the page lays out again when its list changes | `TestWelcomePinsAndLocates`, `TestDropOpensFiles`; `welcome-*` renders |

Explorer additions (7.2): folders over 500 entries end in "Show all (N)"; the hidden folders are a
setting; rows can be dragged onto a folder (move, open tabs follow, Ctrl+Z undoes), onto the
editor (open) or onto the terminal (the quoted path is typed). Rename (F2), Move to Recycle Bin
(Delete) and the context menu already existed.

## Numbers

| Measure | Before | After |
|---|---|---|
| Graph frame, Aegis (3,102 nodes), glow on hubs | 7 min 1.5 s | 15.8 ms |
| Graph frame, 10,000-file sparse graph | not measured | 14.8 ms |
| 100 MB log: open and first frame | 3.0 s on the UI thread, heap +575 MB | 112 ms, heap +132 MB |
| 2 MB JSON on one line: open and first frame | 0.47 s | 80 ms |
| 1 GB disk image | would be read whole | 8 ms (hex view, one screen read) |
| Go to File on Aegis (13,542 files) | 36 ms walk on the UI thread before the palette showed | palette at once; walk on a worker |
| Start-up, window shown (warm; the start-up screen's 950 ms minimum included) | not measured | 1.32–1.36 s; first start after a build 1.84 s |
| Window stalls over 200 ms in the first 10 s, Aegis and this repository | not measured | none |

The 2 MB file's first open took 4.9 s in the test until the file had been opened once: Windows
Defender scans a file just written. That time is not PyxForge's; the test opens each file once
before timing it.

## Gate 7

| Check | Result |
|---|---|
| `forbidcheck`, `gofmt`, `go vet` | Pass |
| `go test -race ./...`, Windows 11 | Pass (see the report for the run on the gate commit) |
| `go test -race ./...`, Ubuntu 24.04 under WSL, with bash, zsh and fish | Pass (same) |
| Renders in every theme and accent | `welcome-*`, `hexview-*`, `longline-*`, `settings-files-*`, `broad-folder-*`: 40 renders, looked at |
| Launch under 1.5 s | 1.32–1.36 s warm |
| No stall over 200 ms | None in the first 10 s on either workspace (the watchdog) |
| Explorer in the packaged build | Not tried by hand here: the screen tool cannot attach to a development build. The fix is in the code path the packaged build runs, and the hit-test test mirrors the GL driver |

## Known issues

- A click whose pointer moves more than Fyne's drag threshold before release becomes a drag,
  not a selection; it drops where it started, which does nothing.
- The hex view has no Go to Offset field yet; Home, End, the arrows, Page keys, the wheel and
  the scroll strip move it.
- Text files between 50 and 512 MB are read whole (about their size in memory), read-only; they
  are not paged from disk.
- The editor's cut marker for a long line can sit past the right edge; the status bar says the
  cursor's line is cut.
