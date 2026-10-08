# Keymap policy

Decision K1 (`DECISIONS.md`, 2026-10-08): **Neovim owns every key while the editor has focus.**
PyxForge's users are Vim users. A shell shortcut on Ctrl+W, Ctrl+B, Ctrl+J or Ctrl+P would break
the editor's window commands, paging, motion and completion.

## The rule

1. The shell binds **only Ctrl+Shift chords** (Cmd+Shift on macOS), named
   `shell.ShellModifier`. `TestShellChordsLeaveKeysToNeovim` fails if any binding uses another
   modifier set or is bound twice.
2. The shell never binds plain Ctrl, Alt, or Ctrl+Alt. On Windows, AltGr arrives as Ctrl+Alt,
   and kernel C needs AltGr for `{ } [ ] \ |` on many layouts (Hungarian AltGr+B is `{`).
3. Every command is in the command palette (Ctrl+Shift+P), so nothing depends on a chord.
4. Keybindings are registered once, in `internal/ui/shell/commands.go`. Anything that lists them,
   such as the empty editor state, reads the registry; nothing types a binding by hand.

## Bindings

| Chord | Command |
|---|---|
| Ctrl+Shift+P | Show All Commands |
| Ctrl+Shift+O | Go to File |
| Ctrl+Shift+E | Toggle Explorer |
| Ctrl+Shift+J | Toggle Panel |
| Ctrl+Shift+I | Toggle Inspector |
| Ctrl+Shift+W | Close Editor Tab |
| Ctrl+Shift+, | Open Settings |

## Why Ctrl+Shift

A terminal cannot send Ctrl+Shift+letter differently from Ctrl+letter, so Neovim configurations
written for terminals almost never map them. A GUI Neovim does receive them, as `<C-S-…>`. A user
who maps one of the chords above in `init.lua` loses it to the shell; they can still run the
shell command from the palette or with `:Pyx`.

## Phase 3 contract (the Neovim editor widget)

- The editor widget implements `fyne.Shortcutable` and `desktop.Keyable`, so Fyne delivers every
  key and shortcut to it while it has focus.
- In `TypedShortcut`, it calls `shell.IsShellChord`. A shell chord is handed back to the window's
  shortcut handler; everything else, copy and paste included, goes to Neovim as input.
- Neovim also gets `:Pyx <command-id>` (for example `:Pyx view.panel`), sent over RPC to the same
  registry, so a user can bind shell commands with their own Neovim mappings.

## Text entries

While a text entry has focus (the command palette's query, for example), Fyne sends shortcuts to
the entry, not the window. Shell chords therefore do nothing until the entry closes. Escape
closes the palette.
