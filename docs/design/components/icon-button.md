# Icon button

Implementation: `internal/ui/kit/button.go`. References: shadcn/ui `button` (icon size) and
`tooltip`; HIG `hig-components-menus` (buttons, toolbars) and `hig-foundations` (accessibility).

## Anatomy

| Part | PyxForge |
|---|---|
| hit area | square, 40 px on the rail, 24 px for panel and toast actions, centred in its cell |
| icon | Lucide line icon, 20 px on the rail, 16 px elsewhere |
| state wash | rounded rect, radius 4 |
| selection marker | 2 px Text.Primary bar on the container's leading edge (rail items: the panel is visible). Neutral by decision A1 |
| focus ring | 2 px Accent.Focus stroke, radius 4 (`kit.StyleFocusRing`, shared by every PyxForge control) |
| accessible name | `Label` field; the same action is in the command palette under that name |

## States

| State | Treatment |
|---|---|
| default | Text.Secondary icon, no wash |
| hover | State.Hover wash, Text.Primary icon |
| pressed | State.Pressed wash |
| selected | State.Selected wash, Text.Primary icon, leading marker |
| focused | 2 px focus ring (keyboard focus only) |

## Keyboard

Tab focuses it; Space or Enter activates.

## Gaps

- No tooltip: icon-only rail items are not labelled on hover. Fyne 2.8 has no tooltip widget and
  canvas overlays capture input, so a tooltip has to live in the workbench's content layer.
  Recorded as a Phase 1 review finding.
