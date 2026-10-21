# Keymap policy

Decision K2 (`DECISIONS.md`, 2026-10-20) replaces K1 ("Neovim owns every key"): **PyxForge takes
the keys a conventional editor uses, and Neovim gets every other key.** K1 suited Vim users only;
the owner's Phase 1 brief makes PyxForge a general-purpose editor first, with Neovim as its
engine.

## The rule

1. **Ctrl+Shift chords** (Cmd+Shift on macOS) are PyxForge's wherever keyboard focus is.
2. **The conventional editor keys** Ctrl+N, Ctrl+O, Ctrl+S, Ctrl+W, Ctrl+F, Ctrl+H and Ctrl+P
   (Cmd on macOS) are PyxForge's, except:
   - in the **Terminal** panel, where the shell program gets them: Ctrl+W deletes a word, Ctrl+R
     and Ctrl+P walk the history, Ctrl+C interrupts;
   - in the **editor** after "Toggle Neovim Keys in the Editor" (palette, saved per user), which
     hands these plain-Ctrl keys back to Neovim; the Ctrl+Shift chords keep working.
   Only the keys bound below are taken; an unbound one (Ctrl+B, Ctrl+J…) still reaches Neovim.
3. **Ctrl+Tab** and **Ctrl+Shift+Tab** switch tabs wherever focus is (Ctrl on every platform:
   Cmd+Tab switches applications on macOS).
4. PyxForge never binds **Alt** or **Ctrl+Alt**. On Windows, AltGr arrives as Ctrl+Alt, and code
   needs AltGr for `{ } [ ] \ |` on many layouts (Hungarian AltGr+B is `{`).
5. Every command is in the command palette (Ctrl+Shift+P), so nothing depends on a key.
6. Keybindings are registered once, in `internal/ui/shell/commands.go`, and the editor and
   terminal ask `takesFromEditor` / `takesFromTerminal` (`internal/ui/shell/keymap.go`) which keys
   are PyxForge's, so a key runs one command once. `TestKeyboardPolicy` and
   `TestEditorShortcutRunsOnce` enforce this; anything that lists the bindings reads the registry.

What Vim users give up while the conventional keys are on: Ctrl+N and Ctrl+P (completion and
line moves), Ctrl+O (jumplist; `''` and `:jumps` remain), Ctrl+W (window commands; `:wincmd`
remains), Ctrl+F (page down; PageDown remains) and Ctrl+H (backspace). Ctrl+S was already
PyxForge's save.

## Bindings

On macOS read Cmd for Ctrl, except in Ctrl+Tab. `TestKeymapDocumentsEveryChord` fails if a
binding the shell registers is missing here.

| Keys | Command |
|---|---|
| Ctrl+N | New File |
| Ctrl+O | Open File… |
| Ctrl+P | Go to File |
| Ctrl+S | Save |
| Ctrl+Shift+S | Save As… |
| Ctrl+F | Find |
| Ctrl+H | Find and Replace |
| Ctrl+W | Close Editor Tab |
| Ctrl+Shift+W | Close All Editor Tabs |
| Ctrl+Tab | Next Tab |
| Ctrl+Shift+Tab | Previous Tab |
| Ctrl+Shift+P | Show All Commands |
| Ctrl+Shift+E | Toggle Explorer |
| Ctrl+Shift+J | Toggle Panel |
| Ctrl+Shift+I | Toggle Inspector |
| Ctrl+Shift+B | Build |
| Ctrl+Shift+M | Show Problems |
| Ctrl+Shift+F8 | Next Problem |
| Ctrl+Shift+F7 | Previous Problem |
| Ctrl+Shift+F9 | Run Current File |
| Ctrl+Shift+R | Run in QEMU |
| Ctrl+Shift+D | Debug in QEMU |
| Ctrl+Shift+F2 | Stop QEMU |
| Ctrl+Shift+F5 | Continue |
| Ctrl+Shift+F10 | Step Over Instruction |
| Ctrl+Shift+F11 | Step Instruction |
| Ctrl+Shift+` | New Terminal |
| Ctrl+Shift+] | Next Terminal |
| Ctrl+Shift+[ | Previous Terminal |
| Ctrl+Shift+G | Show Git |
| Ctrl+Shift+, | Open Settings |

Ctrl+Shift+PageDown and Ctrl+Shift+PageUp also move to the next and previous tab. New File opens
an untitled file; saving it asks where. Find and Find and Replace open a bar under the file:
literal text, Match case and Whole word, Enter for the next match, Esc to close; the matches are
Neovim's search, so n and N go on from them. Next Tab and Previous Tab move through the tabs of the active region (editor,
panel or inspector). Open Settings puts keyboard focus on the first theme card. The debugger
chords follow the F5/F10/F11 convention of other debuggers, with Ctrl+Shift added so Neovim keeps
the plain function keys. Git branches and stashes (Switch Branch…, Create Branch…, Stash
Changes…, Pop Stash…), Switch Terminal…, Add Breakpoint… and the other actions are in the palette.

Inside Neovim, Ctrl+S is also a mapping (`nvim/lua/pyxforge/keymaps.lua`) for when Neovim keys
are chosen; it saves through `pyxforge.save`, like PyxForge's own Save, so a file that changed
on disk is never written over without asking. F9 is a mapping too: it sets or clears a
breakpoint on the cursor's line (Run > Toggle Breakpoint), for Debug Current File and Debug in
QEMU. Being Neovim's, it reaches no other panel, so a program in the Terminal keeps F9.

The Terminal panel is a Neovim terminal: every key goes to the shell except the Ctrl+Shift
chords and Ctrl+Tab, and Esc Esc leaves terminal mode for scrolling and copying.
