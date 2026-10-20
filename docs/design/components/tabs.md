# Tabs

Implementation: `kit.Tabs` (`internal/ui/kit/tabs.go`): `kit.NewDocTabs` for the editor,
`kit.NewTabs` for the bottom dock and the inspector, with the methods of Fyne's DocTabs and
AppTabs. `regionTabs` and `activate` (`internal/ui/shell/regions.go`) tell each bar whether its
region is active. Until Phase 1 the bars were Fyne's own, each in a `container.ThemeOverride`
that was refreshed whenever the active region changed; in Fyne 2.8.1 every such refresh makes it
parse and cache the whole font set again, about 13 MB that is never freed (defect PXF-DEF-002).
Anatomy reference: shadcn/ui `tabs` (Radix) via the shadcn MCP.

## Anatomy

| shadcn part | PyxForge |
|---|---|
| `TabsList` | tab row, 32 px grid row with a hairline below |
| `TabsTrigger` | tab label in Geist 13 semibold; editor tabs add a close mark (Lucide `x`), shown on the selected tab and under the pointer; a middle click also closes |
| `TabsContent` | the tab's content, filling the panel |
| overflow | the row scrolls sideways and keeps the selected tab in view (the inspector at 296 px shows five of its six tabs); Fyne's "more" menu is gone |

## States

| State | Treatment |
|---|---|
| Inactive | Text.Primary label, Geist semibold like the active one |
| Hover | hover wash (`State.Hover`); the close mark gets its own wash under the pointer |
| Active, in the active region | Accent.Primary label and 2 px underline. One tab bar at a time (decision A1) |
| Active, elsewhere | Text.Primary label and 2 px Text.Primary underline |
| Disabled | not used |

## Keyboard

Radix tabs move with ←/→ and activate on focus. The tabs are not keyboard-focusable (neither
were Fyne's), so PyxForge reaches them through commands instead:

| Command | Chord |
|---|---|
| Next Tab / Previous Tab (in the active region, wrapping) | Ctrl+Shift+PageDown / Ctrl+Shift+PageUp |
| Panel: Show Terminal, Build, Problems, QEMU, GDB, Log | palette |
| Inspector: Show Registers, Flags, Hex, Disasm, Memory | palette |
| Focus Explorer, Editor, Panel, Inspector | palette |

A region becomes active when a tab in it is tapped (the selected one too) or selected, a
control in it takes keyboard focus, the user clicks its empty space, or a command opens it.

## Sizing

Tab bar 32 px (DESIGN_SYSTEM.md §8); labels 13 px; close button inside the tab.
