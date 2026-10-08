# Dialog

Implementation: Fyne `dialog.NewCustom` and `dialog.ShowConfirm` (`internal/ui/shell/dialogs.go`).
Anatomy reference: shadcn/ui `dialog` / `alert-dialog`; HIG `hig-components-dialogs` (alerts).

## Anatomy

| Part | PyxForge |
|---|---|
| overlay | Fyne modal scrim in `Glass.Scrim`; backdrop blur radius = `Glass.Blur` (0, D3) |
| content | Fyne dialog body on `Glass.Fill`, radius 8, one soft shadow; Fyne draws no specular edge or rim |
| title | Fyne dialog title (Geist SemiBold) |
| description and body | PyxForge content: Syne name, facts in a form layout with mono values |
| actions | "Close" (custom) or "Yes"/"No" (confirm) |

## Used for

- About: real build facts (version, Go, Fyne, license, workspace).
- Reset Appearance: confirmation before discarding the user's choice (HIG: confirm actions that
  are hard to recover from).

## Keyboard

Fyne dialogs take focus and respond to Enter on the default button. Escape to dismiss is to be
verified in the native app during the keyboard pass.

## Gaps

- No rim or specular edge on Fyne's own dialog body; a PyxForge dialog wrapper on `kit.Glass` would
  add them if the review asks for it.
