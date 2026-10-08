# Command palette

Implementation: `internal/ui/commandpalette/palette.go`; commands from `internal/command`.
Anatomy reference: shadcn/ui `command` (cmdk) via the shadcn MCP, `command-demo` example, read
2026-10-08. Reference only; nothing from the registry is in the repository.

## Anatomy

| shadcn part | PyxForge | Status |
|---|---|---|
| `Command` (root) | glass panel (`kit.Glass`), 640 px wide, 12 % from the top, centred | done |
| `CommandInput` | `widget.Entry` with a mode-specific placeholder ("Run a command", "Go to a file by name", "Select a theme") | done |
| `CommandList` | `widget.List`, at most 10 visible rows, scrolls beyond | done |
| `CommandEmpty` | "No matches" row in tertiary text | done |
| `CommandItem` | 36 px row: optional icon, title, detail (category or folder) in tertiary, keybinding right-aligned in mono caption | done |
| `CommandShortcut` | keybinding column, rendered from the registered shortcut (`Ctrl+Shift+P`) | done |
| `CommandGroup` + heading | the category is shown per row as detail text instead of group headings; fuzzy ranking mixes categories | deliberate |
| `CommandSeparator` | not used | — |
| disabled item | not used: only commands that work today are registered | deliberate |

## States

| State | Treatment |
|---|---|
| Closed | not in the canvas overlays |
| Open | glass panel, entry focused (Accent focus border from Fyne's entry) |
| Selected row | `Accent.Selection` wash (Fyne list selection) |
| Hover row | `State.Hover` wash (Fyne list hover); selection does not follow the pointer |
| No results | "No matches" row; panel keeps one row of height |

## Keyboard

| Key | Action |
|---|---|
| Ctrl+Shift+P | open over all commands |
| Ctrl+P | open over workspace files |
| typing | fuzzy filter (word starts and consecutive runs rank highest) |
| ↓ / ↑ | move selection, wrapping at both ends |
| Enter | run the selected item and close |
| Esc, click outside | close without running |

## Sizing

Row 36 px (SK:240); Fyne lists add 4 px between rows; panel height = entry + rows × 40 + 24.

## Gaps

- Screen-reader semantics: Fyne exposes no accessibility tree, so the list cannot announce
  results (platform limitation, recorded in `PHASE1_REVIEW.md`).
- Matched characters are not highlighted.
- No recent-commands ordering yet.
