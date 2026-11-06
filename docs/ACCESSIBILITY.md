# Accessibility

What PyxForge 3.0 offers people who don't use a mouse or who use a screen reader, what it
doesn't yet, and the alternatives. Checked on Windows 11 with PyxForge at 3.0.0-rc.3 (#49).

## Keyboard

Everything PyxForge does can be done from the keyboard.

- **Every command is in the command palette** (Ctrl+Shift+P): type part of its name and press
  Enter, or move with the arrow keys first. `TestEveryCommandIsReachableFromTheKeyboard` checks
  that typing each of the 50-plus commands' names finds it in the first ten results.
- **The main actions have chords**, all Ctrl+Shift, listed in the
  [keymap](architecture/KEYMAP.md): Build (Ctrl+Shift+B), Run Current File (Ctrl+Shift+F9), the
  explorer, panel and inspector toggles, Go to File (Ctrl+P), and more. The editor keeps the
  conventional keys (Ctrl+S, Ctrl+F, Ctrl+W); every other key goes to Neovim.
- **Dialogs**: Tab moves between fields and buttons; Enter confirms; Esc cancels, whichever
  control has focus (`TestEscClosesADialogFromAFocusedButton`), except inside Fyne's own Open and
  Save file pickers, where Esc needs the focus off their buttons.
- **Settings**: every theme and accent card, and every option, takes focus with Tab and is
  chosen with Space or Enter.
- **The editor is Neovim**, so its whole keyboard language is there.

## Seeing the screen

- Five themes, two light and three dark, each with two accents; every text and accent colour is
  checked against WCAG contrast ratios (`TestContrast`).
- Settings > Motion draws the start-up screen still, and so does Windows' own "Animation effects"
  setting.
- The interface follows the system's scaling.

## Screen readers

**The PyxForge window is not yet usable with a screen reader.** Fyne, its toolkit, added
accessibility in 2.8 behind a build option: a flat list of named buttons and texts, given to
Windows UI Automation (which Narrator and NVDA read) and to macOS. PyxForge's own controls give
their names and roles to it (its icon buttons, tabs, explorer rows, settings cards and texts,
`TestAccessibleNames`). But Fyne lists only controls in plain containers, and most of PyxForge's
interface (panels, tabs, the explorer, the editor) is drawn inside its own widgets, which Fyne
doesn't look into. A build with Fyne's accessibility option, checked with Windows UI
Automation, shows the window's title and the menu bar's seven menus and nothing else. Release
builds don't turn the option on until it can show the rest.

What works with a screen reader today:

- **The command line.** `pyxforge` does the main work without the window, in a terminal a
  screen reader reads: `pyxforge build`, `pyxforge run` (boots the project in QEMU and prints its
  serial output), `pyxforge inspect` (a binary's sections, symbols and disassembly),
  `pyxforge info`, `pyxforge doctor` (which tools are found and how to install the rest),
  `pyxforge toolchains install`, `pyxforge clean`. `pyxforge help` lists them all.
- **Neovim in a terminal.** The editor PyxForge embeds is Neovim; run in a terminal, it works
  with screen readers that read the console, with PyxForge's projects as they are.
- **Git** from the terminal, on the same repositories the Git tab shows.

## Reporting a problem

Open an issue on [GitHub](https://github.com/obstinix/PyxForge/issues) with the label
`area: ui`, saying what you tried, what you use (screen reader, magnifier, keyboard only) and
what happened.
