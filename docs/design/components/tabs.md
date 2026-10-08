# Tabs

Implementation: Fyne `container.DocTabs` (editor) and `container.AppTabs` (bottom dock,
inspector), wrapped in `quiet()` (`internal/ui/shell/panels.go`). Anatomy reference: shadcn/ui
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
| Active | Text.Primary label and a 2 px Text.Primary underline. The accent is kept off tabs: with three tab bars on screen, accented labels put six accent marks in view (DESIGN_SYSTEM.md §1) |
| Disabled | not used |

## Keyboard

Radix tabs move with ←/→ and activate on focus. Fyne 2.8 tabs are not keyboard-focusable: tabs
are reached with the mouse or through commands. **Gap**, tracked for the keyboard-navigation pass.

## Sizing

Tab bar 32 px (DESIGN_SYSTEM.md §8); labels 13 px; close button inside the tab.
