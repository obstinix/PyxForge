# Tabs

Implementation: Fyne `container.DocTabs` (editor) and `container.AppTabs` (bottom dock,
inspector), each wrapped by `regionTabs` in a theme override that follows the active region
(`internal/ui/shell/regions.go`). Anatomy reference: shadcn/ui
`tabs` (Radix) via the shadcn MCP.

## Anatomy

| shadcn part | PyxForge |
|---|---|
| `TabsList` | Fyne tab bar, 32 px grid row with a hairline below |
| `TabsTrigger` | tab label in Geist 13; editor tabs add a close button (Lucide `x`) |
| `TabsContent` | the tab's content, filling the panel |
| overflow | Fyne's "more" button with the Lucide `ellipsis` (inspector at 296 px shows four of five tabs) |

## States

| State | Treatment |
|---|---|
| Inactive | Text.Primary label at Fyne's inactive weight |
| Hover | Fyne hover wash (`State.Hover`) |
| Active, in the active region | Accent.Primary label and 2 px underline (Fyne colours both with Primary). One tab bar at a time (decision A1) |
| Active, elsewhere | Text.Primary label and 2 px Text.Primary underline |
| Disabled | not used |

## Keyboard

Radix tabs move with ←/→ and activate on focus. Fyne 2.8 tabs are not keyboard-focusable, so
PyxForge reaches them through commands instead:

| Command | Chord |
|---|---|
| Next Tab / Previous Tab (in the active region, wrapping) | Ctrl+Shift+PageDown / Ctrl+Shift+PageUp |
| Panel: Show Terminal, Build, Problems, QEMU, GDB, Log | palette |
| Inspector: Show Registers, Flags, Hex, Disasm, Memory | palette |
| Focus Explorer, Editor, Panel, Inspector | palette |

A region becomes active when a tab in it is selected, a control in it takes keyboard focus, the
user clicks its empty space, or a command opens it.

## Sizing

Tab bar 32 px (DESIGN_SYSTEM.md §8); labels 13 px; close button inside the tab.
