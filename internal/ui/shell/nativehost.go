package shell

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"

	"github.com/obstinix/PyxForge/internal/lsp"
	"github.com/obstinix/PyxForge/internal/textbuf"
	"github.com/obstinix/PyxForge/internal/toolchain"
	"github.com/obstinix/PyxForge/internal/ui/codeedit"
	"github.com/obstinix/PyxForge/internal/ui/notifications"
	"github.com/obstinix/PyxForge/internal/ui/theme"
)

// The native editor (decision D16): each tab holds one codeedit.Editor over its own document,
// and a language server per language and project root is started the first time a file of
// that language opens. It replaces the embedded Neovim (editorHost) unless the editor engine
// preference says "neovim" (the default is native since D16 step 3); the shell's entry points (open, save, find, go to line, Problems,
// breakpoints, Run Current File, quit) branch to it through activeDoc.

// nativeDoc is one open document.
type nativeDoc struct {
	ed       *codeedit.Editor
	path     string // "" for a new, untitled file
	untitled string
	item     *container.TabItem
	lang     string // LSP language identifier, "" when no server takes the file
	uri      string
	client   *lsp.Client
	version  int
	lspDiags []Diagnostic // the language server's, lines and columns 0-based
	mtime    time.Time    // the file's modification time when loaded or saved
	stopLine int          // 1-based line a debugger stopped at, 0 for none
	asking   bool         // the changed-on-disk question is showing (askConflict)
	askedFor time.Time    // the file's modification time last asked about
}

func (d *nativeDoc) name() string {
	if d.path == "" {
		return d.untitled
	}
	return filepath.Base(d.path)
}

// nativeHost owns the native editor's tabs and language servers.
type nativeHost struct {
	s        *Shell
	docs     map[*container.TabItem]*nativeDoc
	clients  map[string]*lsp.Client // by server name and root
	waiting  map[string][]*nativeDoc
	missing  map[string]bool
	untitled int
	mu       sync.Mutex // guards clients, running and stopped for stop from another goroutine
	// running is every server started and not yet stopped, whether or not its start has been
	// reported on the UI thread; starting counts the starts under way. stop ends them all.
	running  map[*lsp.Client]bool
	starting sync.WaitGroup
	stopped  bool
}

// startNative makes the native host the first time it is needed; false when editing is off.
func (s *Shell) startNative() bool {
	if s.nat != nil {
		return true
	}
	if !s.editorEnabled {
		return false
	}
	s.nat = &nativeHost{s: s, docs: map[*container.TabItem]*nativeDoc{}, clients: map[string]*lsp.Client{},
		waiting: map[string][]*nativeDoc{}, missing: map[string]bool{}, running: map[*lsp.Client]bool{}}
	s.logf("Native editor ready (%s)", s.root)
	return true
}

// activeDoc is the native document in the selected tab, or nil.
func (s *Shell) activeDoc() *nativeDoc {
	if s.nat == nil {
		return nil
	}
	return s.nat.docs[s.editors.Selected()]
}

// docFor is the open document of path, or nil.
func (h *nativeHost) docFor(path string) *nativeDoc {
	want := lsp.Canonical(path) // a server names files in their long, resolved spelling
	for _, d := range h.docs {
		if d.path != "" && (sameFile(d.path, path) || sameFile(lsp.Canonical(d.path), want)) {
			return d
		}
	}
	return nil
}

// open edits a file in a tab of its own, or selects its tab.
func (h *nativeHost) open(path string, focus bool) *nativeDoc {
	s := h.s
	if d := h.docFor(path); d != nil {
		s.selectTab(d.item, focus)
		return d
	}
	buf, err := textbuf.Load(path)
	if err != nil {
		s.Notify(notifications.Error, "Cannot open file", err.Error())
		return nil
	}
	var mtime time.Time
	if st, err := os.Stat(path); err == nil {
		mtime = st.ModTime()
	}
	d := h.add(buf, path, mtime)
	s.addRecent(prefRecentFiles, path)
	s.logf("Tab %s: opened in PyxForge", d.name())
	s.selectTab(d.item, focus)
	h.attach(d)
	return d
}

// openBinary shows a binary file as read-only text, each byte one character, with no language
// server: its strings can be read, and nothing can be written back.
func (h *nativeHost) openBinary(path string) {
	if d := h.docFor(path); d != nil {
		h.s.selectTab(d.item, true)
		return
	}
	buf, err := textbuf.LoadBinary(path)
	if err != nil {
		h.s.Notify(notifications.Error, "Cannot open as text", err.Error())
		return
	}
	d := h.add(buf, path, time.Time{})
	h.title(d)
	h.s.logf("Tab %s: opened as text, read-only", d.name())
	h.s.selectTab(d.item, true)
}

// newUntitled opens an empty document that Save asks a name for.
func (h *nativeHost) newUntitled() {
	h.untitled++
	d := h.add(textbuf.New(""), "", time.Time{})
	d.untitled = fmt.Sprintf("Untitled-%d", h.untitled)
	h.title(d)
	h.s.selectTab(d.item, true)
}

func (h *nativeHost) add(buf *textbuf.Buffer, path string, mtime time.Time) *nativeDoc {
	s := h.s
	ed := codeedit.New(buf, path)
	ed.SetTextSize(s.textSize(prefEditorTextSize))
	ed.ShellTakes = s.takesFromEditor
	ed.OnShortcut = s.runChord
	ed.Dispatch = s.dispatch
	d := &nativeDoc{ed: ed, path: path, mtime: mtime}
	d.item = container.NewTabItem(filepath.Base(path), ed)
	ed.OnEdit = func() { h.edited(d) }
	ed.OnCursor = func(codeedit.Pos) { h.syncStatus() }
	ed.OnGutterTap = func(line int) {
		if d.path != "" {
			h.toggleBreakpointAt(d, line+1)
		}
	}
	buf.OnChange = func(c textbuf.Change) { h.shiftBreakpoints(d, c) }
	// The editor's own keys (KEYMAP.md): plain function keys and Ctrl+K, which no shell chord
	// uses, so they act only while the editor has focus.
	ed.OnKey = func(k fyne.KeyName, m fyne.KeyModifier) bool {
		switch {
		case m == 0 && k == fyne.KeyF9:
			s.toggleBreakpoint()
		case m == 0 && k == fyne.KeyF12:
			h.definition(d)
		case m == 0 && k == fyne.KeyF2:
			s.nativeRename(d)
		case m == fyne.KeyModifierShortcutDefault && k == fyne.KeyK:
			h.hover(d)
		default:
			return false
		}
		return true
	}
	h.docs[d.item] = d
	s.selectQuietly = true
	s.editors.Append(d.item)
	s.selectQuietly = false
	s.syncEditor()
	h.marks(d)
	return d
}

// edited runs after every change to a document's text.
func (h *nativeHost) edited(d *nativeDoc) {
	h.title(d)
	h.syncStatus()
	if d.client != nil && d.uri != "" {
		d.version++
		_ = d.client.DidChange(d.uri, d.version, d.ed.Buf.Text())
	}
}

// title marks unsaved documents with a dot, as the Neovim tabs do.
func (h *nativeHost) title(d *nativeDoc) {
	t := d.name()
	switch {
	case d.ed.Buf.ReadOnly:
		t += " (read-only)"
	case d.ed.Buf.Modified():
		t += " ●"
	}
	if d.item.Text != t {
		d.item.Text = t
		h.s.editors.Refresh()
	}
}

// selected runs when a document's tab becomes the selected one.
func (h *nativeHost) selected(it *container.TabItem, focus bool) {
	d := h.docs[it]
	if d == nil {
		return
	}
	h.checkDisk(d)
	h.syncStatus()
	if focus {
		h.s.win.Canvas().Focus(d.ed)
	}
}

// checkDisk reloads a document whose file changed on disk while it had no unsaved changes, and
// asks what to do when it has some: once per change on disk (Save asks again).
func (h *nativeHost) checkDisk(d *nativeDoc) {
	if d.path == "" || d.ed.Buf.ReadOnly {
		return
	}
	st, err := os.Stat(d.path)
	if err != nil || !st.ModTime().After(d.mtime) {
		return
	}
	if d.ed.Buf.Modified() {
		if !st.ModTime().Equal(d.askedFor) {
			d.askedFor = st.ModTime()
			h.s.askConflict(d) // d.mtime stays: saving asks again until the user chooses
		}
		return
	}
	h.reload(d)
}

// reload replaces a document's text with its file's, dropping unsaved changes.
func (h *nativeHost) reload(d *nativeDoc) {
	buf, err := textbuf.Load(d.path)
	if err != nil {
		h.s.Notify(notifications.Error, "Reload failed", err.Error())
		return
	}
	st, err := os.Stat(d.path)
	if err != nil {
		return
	}
	cur := d.ed.Cur
	d.ed.Buf.Replace(textbuf.Range{End: d.ed.Buf.End()}, buf.Text())
	d.ed.Buf.Seal()
	d.ed.Buf.MarkSaved() // the text is the file's again
	d.ed.SetCursor(cur, false)
	d.mtime = st.ModTime()
	h.edited(d)
	h.s.logf("%s reloaded: it changed on disk", d.name())
}

// close closes a tab, asking first when the document has unsaved changes.
func (h *nativeHost) close(d *nativeDoc) {
	if !d.ed.Buf.Modified() {
		h.remove(d)
		return
	}
	save := func() {
		if h.save(d) {
			h.remove(d)
		}
	}
	if d.path == "" {
		save = func() { h.s.nativeSaveAs(d) }
	}
	h.s.confirmUnsaved(fmt.Sprintf("Save changes to %s?", d.name()), save, func() { h.remove(d) })
}

func (h *nativeHost) remove(d *nativeDoc) {
	if d.client != nil && d.uri != "" {
		_ = d.client.DidClose(d.uri)
	}
	delete(h.docs, d.item)
	h.s.editors.Remove(d.item)
	h.s.editors.OnClosed(d.item)
	h.s.refreshProblems()
	h.syncStatus()
}

// save writes a document to its file; false, after telling the user why, when it could not.
func (h *nativeHost) save(d *nativeDoc) bool {
	if d.path == "" {
		h.s.nativeSaveAs(d)
		return false
	}
	if d.ed.Buf.ReadOnly {
		h.s.Notify(notifications.Info, d.name()+" is read-only", "It is a binary file shown as text; nothing is written back.")
		return false
	}
	if st, err := os.Stat(d.path); err == nil && st.ModTime().After(d.mtime) {
		h.s.askConflict(d) // changed on disk since it was loaded or saved
		return false
	}
	return h.write(d)
}

// write saves a document over its file, whatever the file holds now.
func (h *nativeHost) write(d *nativeDoc) bool {
	if err := d.ed.Buf.Save(d.path); err != nil {
		h.s.Notify(notifications.Error, "Save failed", err.Error())
		return false
	}
	if st, err := os.Stat(d.path); err == nil {
		d.mtime = st.ModTime()
	}
	if d.client != nil {
		_ = d.client.DidSave(d.uri)
	}
	h.title(d)
	h.syncStatus()
	if h.s.gitp.loaded {
		h.s.gitp.refresh()
	}
	return true
}

// saveAs writes a document to a new path, which becomes its file.
func (h *nativeHost) saveAs(d *nativeDoc, path string) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		h.s.Notify(notifications.Error, "Save As failed", err.Error())
		return
	}
	if err := d.ed.Buf.Save(path); err != nil {
		h.s.Notify(notifications.Error, "Save As failed", err.Error())
		return
	}
	if d.client != nil && d.uri != "" {
		_ = d.client.DidClose(d.uri)
		d.client, d.uri = nil, ""
	}
	d.path, d.untitled = path, ""
	if st, err := os.Stat(path); err == nil {
		d.mtime = st.ModTime()
	}
	h.title(d)
	h.s.addRecent(prefRecentFiles, path)
	h.s.explorer.SetActive(h.s.activeFile())
	h.attach(d)
}

// modified lists the documents with unsaved changes.
func (h *nativeHost) modified() []*nativeDoc {
	var out []*nativeDoc
	for _, d := range h.docs {
		if d.ed.Buf.Modified() {
			out = append(out, d)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].name() < out[j].name() })
	return out
}

// saveAll saves every modified document; false when one could not be (an untitled one is
// given a name first).
func (h *nativeHost) saveAll() bool {
	ok := true
	for _, d := range h.modified() {
		ok = h.save(d) && ok
	}
	return ok
}

// noFileOpen is the editor's status with the native editor and no document selected.
const noFileOpen = "No file open"

// syncStatus shows the current document, its cursor, its encoding and line endings, its language
// and its language server in the status bar.
func (h *nativeHost) syncStatus() {
	d := h.s.activeDoc()
	if d == nil {
		h.s.statusEditor.SetText(noFileOpen)
		return
	}
	text := d.name()
	if d.ed.Buf.Modified() {
		text += " ●"
	}
	text += fmt.Sprintf(" · Ln %d, Col %d", d.ed.Cur.Line+1, d.ed.Cur.Col+1)
	if d.ed.Buf.ReadOnly {
		text += " · Read-only"
	} else {
		text += " · " + d.ed.Buf.EncodingName() + " · " + d.ed.Buf.LineEnding()
	}
	if lang := d.ed.Language(); lang != "" {
		text += " · " + lang
	}
	if d.client != nil {
		text += " · " + d.client.Name
	}
	if h.s.statusEditor.text.Text != text {
		h.s.statusEditor.SetText(text)
	}
}

// --- Language servers

// attach starts (or reuses) the language server for a document's language and opens the
// document in it. A server that is not installed is reported once and not retried.
func (h *nativeHost) attach(d *nativeDoc) {
	srv, lang := lsp.ForFile(d.path)
	if srv == nil {
		return
	}
	d.lang, d.uri = lang, lsp.URI(d.path)
	root := srv.Root(d.path, h.s.root)
	key := srv.Name + "|" + strings.ToLower(root)
	if c := h.clients[key]; c != nil && c.Err() == nil {
		h.opened(d, c)
		return
	}
	if _, starting := h.waiting[key]; starting {
		h.waiting[key] = append(h.waiting[key], d)
		return
	}
	cmd := srv.Command(toolchain.LookPath)
	if cmd == nil {
		if !h.missing[srv.Name] {
			h.missing[srv.Name] = true
			h.s.logf("%s is not installed: no completion or diagnostics for this file type. pyxforge doctor lists language servers.", srv.Name)
			h.s.Notify(notifications.Info, srv.Name+" not installed", "Editing works; language features need the server.")
		}
		return
	}
	h.mu.Lock()
	if h.stopped {
		h.mu.Unlock()
		return
	}
	h.starting.Add(1)
	h.mu.Unlock()
	h.waiting[key] = []*nativeDoc{d}
	settings := srv.Settings
	name := srv.Name
	go func() {
		defer h.starting.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		var c *lsp.Client
		c, err := lsp.Start(ctx, root, cmd, func(uri string, diags []lsp.Diagnostic) {
			h.s.dispatch(func() { h.diagnostics(c, uri, diags) })
		})
		if err == nil {
			h.mu.Lock()
			stopped := h.stopped
			if !stopped {
				h.running[c] = true
			}
			h.mu.Unlock()
			if stopped { // the editor stopped while this server started
				sctx, scancel := context.WithTimeout(context.Background(), 2*time.Second)
				c.Shutdown(sctx)
				scancel()
				return
			}
			c.Settings = settings
			_ = c.Configure()
		}
		h.s.dispatch(func() {
			docs := h.waiting[key]
			delete(h.waiting, key)
			if err != nil {
				h.s.logf("%s did not start: %v", name, err)
				return
			}
			h.mu.Lock()
			h.clients[key] = c
			h.mu.Unlock()
			if h.s.closed.Load() {
				go c.Kill()
				return
			}
			h.s.logf("%s attached (%s)", name, root)
			for _, d := range docs {
				if h.docs[d.item] == d {
					h.opened(d, c)
				}
			}
			h.syncStatus()
		})
	}()
}

// opened tells a running server about a document and wires completion to it.
func (h *nativeHost) opened(d *nativeDoc, c *lsp.Client) {
	d.client = c
	d.version = 1
	_ = c.DidOpen(d.uri, d.lang, d.version, d.ed.Buf.Text())
	d.ed.Complete = func(at codeedit.Pos) { h.complete(d, at) }
}

// complete asks the server for completions at the cursor and shows those matching the word
// typed so far.
func (h *nativeHost) complete(d *nativeDoc, at codeedit.Pos) {
	c, uri := d.client, d.uri
	if c == nil {
		return
	}
	prefix := strings.ToLower(d.ed.IdentifierBefore())
	pos := lsp.Position{Line: at.Line, Character: c.Character(d.ed.Buf.Line(at.Line), at.Col)}
	go func() {
		ctx, cancel := lsp.Context()
		defer cancel()
		items, err := c.Completion(ctx, uri, pos)
		if err != nil {
			return
		}
		var out []codeedit.Completion
		for _, it := range items {
			label := strings.TrimSpace(it.Label)
			insert := it.InsertText
			if it.TextEdit != nil {
				insert = it.TextEdit.NewText
			}
			if insert == "" {
				insert = label
			}
			key := it.FilterText
			if key == "" {
				key = label
			}
			if prefix != "" && !strings.HasPrefix(strings.ToLower(key), prefix) {
				continue
			}
			out = append(out, codeedit.Completion{Label: label, Detail: it.Detail, Insert: insert})
			if len(out) == 200 {
				break
			}
		}
		d.ed.ShowCompletions(out)
	}()
}

// diagnostics records what a server reported for one document.
func (h *nativeHost) diagnostics(c *lsp.Client, uri string, diags []lsp.Diagnostic) {
	path := lsp.Path(uri)
	for _, d := range h.docs {
		if d.client != c || d.uri != uri && !sameFile(lsp.Canonical(d.path), path) {
			continue
		}
		d.lspDiags = d.lspDiags[:0]
		for _, x := range diags {
			line := d.ed.Buf.Line(x.Range.Start.Line)
			d.lspDiags = append(d.lspDiags, Diagnostic{Line: x.Range.Start.Line,
				Col: c.Column(line, x.Range.Start.Character), Severity: max(x.Severity, 1), Message: x.Message, Source: c.Name})
		}
		h.showDiagnostics(d, diags)
		h.s.refreshProblems()
	}
}

// showDiagnostics draws a document's problems: the language server's and the last build's.
func (h *nativeHost) showDiagnostics(d *nativeDoc, fromServer []lsp.Diagnostic) {
	var out []codeedit.Diagnostic
	if fromServer != nil && d.client != nil {
		for _, x := range fromServer {
			sl, el := d.ed.Buf.Line(x.Range.Start.Line), d.ed.Buf.Line(x.Range.End.Line)
			out = append(out, codeedit.Diagnostic{Severity: max(x.Severity, 1), Message: x.Message, Range: codeedit.Range{
				Start: codeedit.Pos{Line: x.Range.Start.Line, Col: d.client.Column(sl, x.Range.Start.Character)},
				End:   codeedit.Pos{Line: x.Range.End.Line, Col: d.client.Column(el, x.Range.End.Character)}}})
		}
	} else {
		for _, x := range d.lspDiags {
			p := codeedit.Pos{Line: x.Line, Col: x.Col}
			out = append(out, codeedit.Diagnostic{Severity: x.Severity, Message: x.Message, Range: codeedit.Range{Start: p, End: p}})
		}
	}
	for _, p := range h.s.buildp.diags {
		if p.located() && !p.noLine && d.path != "" && sameFile(p.path, d.path) {
			at := codeedit.Pos{Line: p.d.Line, Col: p.d.Col}
			out = append(out, codeedit.Diagnostic{Severity: p.d.Severity, Message: p.d.Message, Range: codeedit.Range{Start: at, End: at}})
		}
	}
	d.ed.SetDiagnostics(out)
}

// hover shows the server's description of the symbol at the cursor.
func (h *nativeHost) hover(d *nativeDoc) {
	if d.client == nil {
		h.s.Notify(notifications.Info, "No language server", "Hover needs a language server for this file type.")
		return
	}
	c, uri := d.client, d.uri
	pos := lsp.Position{Line: d.ed.Cur.Line, Character: c.Character(d.ed.Buf.Line(d.ed.Cur.Line), d.ed.Cur.Col)}
	go func() {
		ctx, cancel := lsp.Context()
		defer cancel()
		text, err := c.Hover(ctx, uri, pos)
		if err != nil || text == "" {
			text = "No information here."
		}
		d.ed.ShowInfo(text)
	}()
}

// definition goes to where the symbol at the cursor is defined, in its file.
func (h *nativeHost) definition(d *nativeDoc) {
	if d.client == nil {
		h.s.Notify(notifications.Info, "No language server", "Go to Definition needs a language server for this file type.")
		return
	}
	c, uri := d.client, d.uri
	pos := lsp.Position{Line: d.ed.Cur.Line, Character: c.Character(d.ed.Buf.Line(d.ed.Cur.Line), d.ed.Cur.Col)}
	go func() {
		ctx, cancel := lsp.Context()
		defer cancel()
		locs, err := c.Definition(ctx, uri, pos)
		h.s.dispatch(func() {
			if err != nil || len(locs) == 0 {
				h.s.Notify(notifications.Info, "No definition found", "The language server knows no definition for the symbol at the cursor.")
				return
			}
			h.goTo(lsp.Path(locs[0].URI), locs[0].Range.Start, c)
		})
	}()
}

// goTo opens path and puts the cursor at a server position.
func (h *nativeHost) goTo(path string, p lsp.Position, c *lsp.Client) {
	d := h.open(path, true)
	if d == nil {
		return
	}
	col := p.Character
	if c != nil {
		col = c.Column(d.ed.Buf.Line(p.Line), p.Character)
	}
	d.ed.Reveal(codeedit.Pos{Line: p.Line, Col: col})
}

// rename renames the symbol at the cursor everywhere the server says it is used, in the
// documents that are open and in files on disk.
func (h *nativeHost) rename(d *nativeDoc, newName string) {
	if d.client == nil || strings.TrimSpace(newName) == "" {
		return
	}
	c, uri := d.client, d.uri
	pos := lsp.Position{Line: d.ed.Cur.Line, Character: c.Character(d.ed.Buf.Line(d.ed.Cur.Line), d.ed.Cur.Col)}
	go func() {
		ctx, cancel := lsp.Context()
		defer cancel()
		w, err := c.Rename(ctx, uri, pos, newName)
		h.s.dispatch(func() {
			if err != nil {
				h.s.Notify(notifications.Error, "Not renamed", err.Error())
				return
			}
			n, files := h.applyEdits(w.Files(), c)
			h.s.Notify(notifications.Success, "Renamed", fmt.Sprintf("%d changes in %d files.", n, files))
		})
	}()
}

// format replaces the document with the server's formatting of it.
func (h *nativeHost) format(d *nativeDoc) {
	if d.client == nil {
		h.s.Notify(notifications.Info, "No language server", "Format Document needs a language server for this file type.")
		return
	}
	c, uri := d.client, d.uri
	spaces := d.ed.Indent != "\t"
	go func() {
		ctx, cancel := lsp.Context()
		defer cancel()
		edits, err := c.Format(ctx, uri, max(len(d.ed.Indent), 4), spaces)
		h.s.dispatch(func() {
			if err != nil {
				h.s.Notify(notifications.Error, "Not formatted", err.Error())
				return
			}
			h.applyEdits(map[string][]lsp.TextEdit{uri: edits}, c)
		})
	}()
}

// applyEdits applies a server's edits: to the open document of each file, as one undo step,
// or to the file on disk. It returns how many edits it made, and in how many files.
func (h *nativeHost) applyEdits(byURI map[string][]lsp.TextEdit, c *lsp.Client) (n, files int) {
	for uri, edits := range byURI {
		path := lsp.Path(uri)
		d := h.docFor(path)
		if d == nil {
			buf, err := textbuf.Load(path)
			if err != nil {
				continue
			}
			applyTo(buf, edits, c)
			if err := buf.Save(path); err != nil {
				h.s.Notify(notifications.Error, "Not saved: "+filepath.Base(path), err.Error())
				continue
			}
		} else {
			d.ed.Buf.Seal()
			applyTo(d.ed.Buf, edits, c)
			d.ed.Buf.Seal()
			d.ed.Fix()
			h.edited(d)
			d.ed.Refresh()
		}
		n += len(edits)
		files++
	}
	return n, files
}

// applyTo applies edits, last first, so earlier positions stay valid.
func applyTo(b *textbuf.Buffer, edits []lsp.TextEdit, c *lsp.Client) {
	sorted := slices.Clone(edits)
	sort.SliceStable(sorted, func(i, j int) bool {
		a, b := sorted[i].Range.Start, sorted[j].Range.Start
		return a.Line > b.Line || a.Line == b.Line && a.Character > b.Character
	})
	pos := func(p lsp.Position) textbuf.Pos {
		return textbuf.Pos{Line: p.Line, Col: c.Column(b.Line(p.Line), p.Character)}
	}
	for _, e := range sorted {
		b.Replace(textbuf.Range{Start: pos(e.Range.Start), End: pos(e.Range.End)}, e.NewText)
	}
}

// --- Breakpoints and the stopped line

func (h *nativeHost) toggleBreakpointAt(d *nativeDoc, line int) {
	s := h.s
	list := slices.Clone(s.breakpoints)
	i := slices.IndexFunc(list, func(b Breakpoint) bool { return sameFile(b.File, d.path) })
	if i < 0 {
		list = append(list, Breakpoint{File: d.path, Lines: []int{line}})
	} else {
		lines := slices.Clone(list[i].Lines)
		if j := slices.Index(lines, line); j >= 0 {
			lines = slices.Delete(lines, j, j+1)
		} else {
			lines = append(lines, line)
			slices.Sort(lines)
		}
		if len(lines) == 0 {
			list = slices.Delete(list, i, i+1)
		} else {
			list[i] = Breakpoint{File: list[i].File, Lines: lines}
		}
	}
	s.setBreakpoints(list)
	h.marks(d)
}

// shiftBreakpoints keeps breakpoints on their lines when lines are added or removed above.
func (h *nativeHost) shiftBreakpoints(d *nativeDoc, c textbuf.Change) {
	if d.path == "" {
		return
	}
	delta := strings.Count(c.Text, "\n") - (c.Range.End.Line - c.Range.Start.Line)
	if delta == 0 {
		return
	}
	changed := false
	list := slices.Clone(h.s.breakpoints)
	for i, b := range list {
		if !sameFile(b.File, d.path) {
			continue
		}
		var lines []int
		for _, l := range b.Lines {
			switch {
			case l-1 > c.Range.Start.Line && l-1 <= c.Range.End.Line && delta < 0:
				// the line was removed: the breakpoint goes with it
			case l-1 > c.Range.Start.Line || l-1 == c.Range.Start.Line && c.Range.Start.Col == 0 && delta > 0:
				lines = append(lines, l+delta)
			default:
				lines = append(lines, l)
			}
		}
		list[i] = Breakpoint{File: b.File, Lines: lines}
		changed = true
	}
	if changed {
		h.s.setBreakpoints(slices.DeleteFunc(list, func(b Breakpoint) bool { return len(b.Lines) == 0 }))
		h.marks(d)
	}
}

// marks draws a document's breakpoints and stopped line in its gutter.
func (h *nativeHost) marks(d *nativeDoc) {
	t := theme.Current()
	for l := 0; l < d.ed.Buf.LineCount(); l++ {
		d.ed.SetMark(l, nil)
	}
	red := t.Status.Error
	for _, b := range h.s.breakpoints {
		if d.path != "" && sameFile(b.File, d.path) {
			for _, l := range b.Lines {
				d.ed.SetMark(l-1, &red)
			}
		}
	}
	if d.stopLine > 0 {
		c := t.Accent.Primary
		d.ed.SetMark(d.stopLine-1, &c)
	}
}

// showStop marks the line a debugger stopped at and scrolls to it.
func (h *nativeHost) showStop(file string, line int) {
	d := h.open(file, false)
	if d == nil {
		return
	}
	for _, o := range h.docs {
		if o != d && o.stopLine > 0 {
			o.stopLine = 0
			h.marks(o)
		}
	}
	d.stopLine = line
	h.marks(d)
	d.ed.Reveal(codeedit.Pos{Line: line - 1})
}

func (h *nativeHost) clearStop() {
	for _, d := range h.docs {
		if d.stopLine > 0 {
			d.stopLine = 0
			h.marks(d)
		}
	}
}

// --- Find

// find moves to the next (back: previous) match of text in a document and returns where it is
// among all matches; with here a match at the cursor counts.
func (h *nativeHost) find(d *nativeDoc, text string, o FindOptions, back, here bool) FindResult {
	if text == "" {
		d.ed.SetCursor(d.ed.Cur, false)
		d.ed.Refresh()
		return FindResult{}
	}
	all := matches(d.ed.Model, text, o)
	if len(all) == 0 {
		return FindResult{}
	}
	from := d.ed.Cur
	if d.ed.HasSelection() {
		from = d.ed.Selection().Start
	}
	pick := -1
	switch {
	case back:
		for i := len(all) - 1; i >= 0; i-- {
			if all[i].Start.Less(from) {
				pick = i
				break
			}
		}
		if pick < 0 {
			pick = len(all) - 1
		}
	default:
		for i, r := range all {
			if here && !r.Start.Less(from) || !here && from.Less(r.Start) {
				pick = i
				break
			}
		}
		if pick < 0 {
			pick = 0
		}
	}
	d.ed.Select(all[pick])
	d.ed.Reveal(all[pick].Start)
	d.ed.Select(all[pick])
	d.ed.Refresh()
	return FindResult{Current: pick + 1, Total: len(all)}
}

// matches lists every match of text in a model, in order.
func matches(m *codeedit.Model, text string, o FindOptions) []codeedit.Range {
	var out []codeedit.Range
	q := []rune(text)
	if !o.MatchCase {
		q = []rune(strings.ToLower(text))
	}
	for l := 0; l < m.Buf.LineCount(); l++ {
		line := m.Buf.Line(l)
		cmp := line
		if !o.MatchCase {
			cmp = []rune(strings.ToLower(string(line)))
		}
		for i := 0; i+len(q) <= len(cmp); i++ {
			if string(cmp[i:i+len(q)]) != string(q) {
				continue
			}
			if o.WholeWord && (i > 0 && isWordRune(line[i-1]) || i+len(q) < len(line) && isWordRune(line[i+len(q)])) {
				continue
			}
			out = append(out, codeedit.Range{Start: codeedit.Pos{Line: l, Col: i}, End: codeedit.Pos{Line: l, Col: i + len(q)}})
			i += len(q) - 1
		}
	}
	return out
}

func isWordRune(r rune) bool { return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r) }

// replace replaces the selected match and moves to the next.
func (h *nativeHost) replace(d *nativeDoc, text, with string, o FindOptions) FindResult {
	all := matches(d.ed.Model, text, o)
	if d.ed.HasSelection() && slices.Contains(all, d.ed.Selection()) {
		d.ed.Paste(with)
		h.edited(d)
	}
	return h.find(d, text, o, false, true)
}

// replaceAll replaces every match, as one undo step.
func (h *nativeHost) replaceAll(d *nativeDoc, text, with string, o FindOptions) int {
	all := matches(d.ed.Model, text, o)
	d.ed.Buf.Seal()
	for i := len(all) - 1; i >= 0; i-- {
		d.ed.Buf.Replace(all[i], with)
	}
	d.ed.Buf.Seal()
	d.ed.Fix()
	if len(all) > 0 {
		h.edited(d)
	}
	d.ed.Refresh()
	return len(all)
}

// --- Shutdown

// stop ends every language server, waiting a short while for each to exit by itself.
func (h *nativeHost) stop() {
	h.mu.Lock()
	h.stopped = true
	h.mu.Unlock()
	// A server still starting shuts itself down once started; wait a while for that, not for
	// one that hangs in its start.
	started := make(chan struct{})
	go func() { h.starting.Wait(); close(started) }()
	select {
	case <-started:
	case <-time.After(5 * time.Second):
	}
	h.mu.Lock()
	clients := make([]*lsp.Client, 0, len(h.running))
	for c := range h.running {
		clients = append(clients, c)
	}
	h.running = map[*lsp.Client]bool{}
	h.clients = map[string]*lsp.Client{}
	h.mu.Unlock()
	var wg sync.WaitGroup
	for _, c := range clients {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			c.Shutdown(ctx)
		}()
	}
	wg.Wait()
}
