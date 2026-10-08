# Notification (toast)

Implementation: `internal/ui/notifications/notifications.go`. Anatomy reference: shadcn/ui
`sonner` via the shadcn MCP (`sonner-types` example).

## Anatomy

| Sonner part | PyxForge |
|---|---|
| toast container (position) | content layer, stacked upward from the bottom-right, above the status bar; not a canvas overlay, so the workspace stays clickable around it |
| icon by type | Lucide `info`, `circle-check`, `triangle-alert`, `circle-x` in the status colour |
| title | Geist SemiBold 13, Text.Primary |
| description | Geist 13, Text.Secondary |
| close button | 24 px icon button (`x`), keyboard-focusable |
| action button | not implemented |
| promise / loading toast | not implemented |

## Types

default/info, success, warning, error, matching Sonner. Colour is never the only signal: each
type has its own icon (HIG accessibility: convey information with more than colour).

## Behaviour

| Rule | Value |
|---|---|
| width | 360 px |
| visible at once | 4; the oldest is dropped |
| lifetime | 6 s; errors 10 s |
| material | glass (DESIGN_SYSTEM.md §9) |
| log | every notification is also written to the Log tab |

## Gaps

- Sonner pauses dismissal while hovered; PyxForge does not yet.
- No action button (for example "Retry"), needed once builds and QEMU exist.
- Time-limited content is an accessibility concern (HIG: minimise time-boxed elements): the Log
  tab keeps every message, so nothing is lost when a toast expires.
