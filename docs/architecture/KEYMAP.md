# Keymap policy

Decision K2 (`ARCHITECTURE_DECISIONS.md`, 2026-10-20) replaced K1 ("Neovim owns every key"): **PyxForge takes
the keys a conventional editor uses, and the editor gets every other key.** K2 was written when
the editor was an embedded Neovim; since D16 the editor is PyxForge's own
(`internal/ui/codeedit`), and the rule is unchanged.

## The rule

1. **Ctrl+Shift chords** (Cmd+Shift on macOS) are PyxForge's wherever keyboard focus is.
2. **The conventional editor keys** Ctrl+N, Ctrl+O, Ctrl+S, Ctrl+W, Ctrl+F, Ctrl+H and Ctrl+P
   (Cmd on macOS) are PyxForge's, except:
   - in the **Terminal** panel, where the shell program gets them: Ctrl+W deletes a word, Ctrl+R
     and Ctrl+P walk the history, Ctrl+C interrupts.
   Only the keys bound below are taken; an unbound one (Ctrl+B, Ctrl+J…) still reaches the editor.
3. **Ctrl+Tab** and **Ctrl+Shift+Tab** switch tabs wherever focus is (Ctrl on every platform:
   Cmd+Tab switches applications on macOS).
4. PyxForge never binds **Alt** or **Ctrl+Alt**. On Windows, AltGr arrives as Ctrl+Alt, and code
   needs AltGr for `{ } [ ] \ |` on many layouts (Hungarian AltGr+B is `{`).
5. Every command is in the command palette (Ctrl+Shift+P), so nothing depends on a key.
6. Keybindings are registered once, in `internal/ui/shell/commands.go`, and the editor and
   terminal ask `takesFromEditor` / `takesFromTerminal` (`internal/ui/shell/keymap.go`) which keys
   are PyxForge's, so a key runs one command once. `TestKeyboardPolicy` and
   `TestEditorShortcutRunsOnce` enforce this; anything that lists the bindings reads the registry.


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
| Ctrl+Shift+F | Find in Terminal Output |
| Ctrl+Shift+Up | Previous Command in Terminal |
| Ctrl+Shift+Down | Next Command in Terminal |
| Ctrl+Shift+G | Show Git |
| Ctrl+Shift+O | Go to Symbol in File |
| Ctrl+Shift+, | Open Settings |

Ctrl+Shift+PageDown and Ctrl+Shift+PageUp also move to the next and previous tab. New File opens
an untitled file; saving it asks where. Find and Find and Replace open a bar under the file:
literal text, Match case and Whole word, Enter for the next match, Esc to close. Next Tab and Previous Tab move through the tabs of the active region (editor,
panel or inspector). Open Settings opens Settings with focus in its search field (see Settings below). The debugger
chords follow the F5/F10/F11 convention of other debuggers, with Ctrl+Shift added so the editor
and the terminal keep the plain function keys. Git branches and stashes (Switch Branch…, Create Branch…, Stash
Changes…, Pop Stash…), Switch Terminal…, Add Breakpoint… and the other actions are in the palette.

Right-click an editor tab for Close, Close Others, Close to the Right, Close Saved, Close All,
Pin or Unpin, Copy Path, Copy Relative Path and Reveal. Pinned tabs stay at the front with a pin
in place of the close mark (tap it to unpin); Close Others, Close to the Right, Close Saved and
Close All (Ctrl+Shift+W) leave them open, a middle click does not close them, and Close or
Ctrl+W still does. Drag a tab to move it; the palette also has Move Editor Tab Left and Right,
Pin or Unpin Editor Tab and the bulk closes. Pins and the tab order come back with the workspace.

The editor splits into up to three groups side by side (D20): Split Right in a tab's menu or the
palette moves the tab into the group on its right, adding one; a file is in one group at a time.
The palette also has Move Editor to Left Group, Focus Next and Previous Editor Group and Join All
Editor Groups. Commands and keys act on the active group, whose selected tab is drawn in the
accent. A group closes with its last tab, and the groups come back with the workspace.

In the Git tab, selecting a changed file opens its side-by-side diff in an editor tab;
the palette's Show Changes in Current File does the same for the active file. The diff's bar has
Previous Change, Next Change, Refresh and Open File.

In the editor, F9 sets or clears a breakpoint on the cursor's line, and so does a click in the
gutter (the sign and line-number columns) on that line.

In the explorer, Enter opens the highlighted file and Space previews it; F2 renames it and
Delete moves it to the Recycle Bin (the Trash on Linux) after asking. With no highlighted row,
F2 and Delete act on the active file. Right-click a row for the same actions and New File, New
Folder, Copy Path, Copy Relative Path and Reveal in the file manager.

The editor (decision D16) has keys of its own, which act
only while it has focus because no shell chord uses them: F9 sets or clears a breakpoint, F12
goes to the definition of the symbol at the cursor, Shift+F12 lists its references in the
palette, F2 renames it, Ctrl+K shows the language
server's description of it, Ctrl+Space asks for completions (they also appear while typing),
Ctrl+/ comments or uncomments the selected lines, and Tab and Shift+Tab indent and outdent
them. Ctrl+Z and Ctrl+Y (or Ctrl+Shift+Z) undo and redo, Ctrl+Left and Ctrl+Right move by words,
and Ctrl+Home and Ctrl+End go to the start and end of the file. A click on a line number sets or
clears a breakpoint there; a click in the narrow column between the numbers and the text folds
the block that starts on that line, as the palette's Fold or Unfold Block does at the cursor.

The Terminal panel (D17) gives every key to the program except the Ctrl+Shift chords and
Ctrl+Tab. Ctrl+C copies when text is selected and interrupts otherwise; Ctrl+Shift+C and
Ctrl+Shift+V copy and paste, as does Shift+Insert for paste; on Windows Ctrl+V pastes too.
The wheel and Shift+PageUp/PageDown scroll the history, typing returns to the end, a drag or
a double click selects, and a right click copies the selection or pastes. A program that asks
for the mouse (htop, vim with `mouse=a`) gets it; Shift with the mouse still selects.

Command blocks (D24). The identity prompt asked for Ctrl+Up and Ctrl+Down to jump between
prompts; K2 gives the shell every plain-Ctrl key (readline and PSReadLine use Ctrl+arrows), so
they are Ctrl+Shift+Up and Ctrl+Shift+Down. Ctrl+Shift+F opens Find in Terminal Output over the
panes: Enter and Shift+Enter step through the matches and Esc closes it. Ctrl+click opens a
`path:line` link in the editor. Hovering a block shows its actions (Copy Command, Copy Output,
Rerun, Collapse, Send to Problems); from the keyboard they are palette commands (Copy Terminal
Command, Copy Terminal Command's Output, Rerun Terminal Command, Collapse or Expand Terminal
Command, Send Terminal Command to Problems) acting on the command at the top of the view after
Ctrl+Shift+Up or Down, otherwise the last one that ended. The session
rail's right-click menu renames, restarts, splits and closes sessions, and Split Terminal
Right, Close Terminal Split and Close Exited Terminals are in the palette. A QEMU serial or
GDB session is read only until its Send input switch (Tab reaches it, Space turns it on).

## Graph view (D23)

Open Graph View and Show Local Graph are in the palette, and the rail's graph button opens the
Graph. With keyboard focus on the graph, the arrows pan, + and - zoom, 0 fits the graph to the
view, Tab and Shift+Tab go through the nodes in PageRank order (the most linked first), Enter
opens the node's file and Esc lets go of the node. With the mouse, the wheel zooms around the
pointer, a drag on the background pans, a drag on a node moves and pins it, a double click
unpins it, a click opens the file and Ctrl+click opens it in another editor group. The controls
panel's switches, lists and sliders are kit controls: Tab reaches each.

## Forge Notes (D25)

New Note…, New Note from Selection…, Open Today's Dev Log, Insert Link to Current Location,
Open Markdown Preview and Show Backlinks are in the palette (the Notes commands). In a preview,
a link is a button: Tab reaches it, Enter or a click follows it. A link to a name several files
have opens the palette to choose one. The Backlinks tab lists each linking note as a button.

## Settings (D22)

Ctrl+Shift+, opens Settings over the workspace with keyboard focus in its search field (K2 gives
PyxForge no plain Ctrl+, chord); the status bar's theme item opens it at Appearance with focus on
the first theme card. Typing searches every page. Tab moves from the search field to the page
list, where Up and Down move and Enter chooses a page, and on into the page's controls. Esc closes
Settings whatever has focus, except that in a search field with text it clears the text first; the
close button and a click outside Settings close it too.

## Controls (kit, identity prompt Phase 1)

Every PyxForge control takes keyboard focus with Tab and Shift+Tab and shows the 2 px focus
ring while it has it.

| Control | Keys |
|---|---|
| Button (`kit.Button`) | Space, Enter or Return presses it; in a dialog, Esc closes the dialog |
| Switch (`kit.Switch`) | Space, Enter or Return turns it on or off |
| Dropdown (`kit.Dropdown`) | Space, Enter or Return opens the list; Up and Down change the choice without opening it |
| Slider (`kit.Slider`) | Left/Down and Right/Up move one step, with Shift ten steps; Home and End go to the ends |
| Navigation list (`kit.NavList`) | Up and Down move the cursor past section headings; Enter, Return or Space selects |
| Search field (`kit.SearchField`) | Esc clears the text; Esc on an empty field leaves it (the owner decides what that does) |

