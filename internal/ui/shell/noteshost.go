package shell

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"github.com/obstinix/PyxForge/internal/config"
	"github.com/obstinix/PyxForge/internal/lsp"
	"github.com/obstinix/PyxForge/internal/notes"
	"github.com/obstinix/PyxForge/internal/ui/codeedit"
	"github.com/obstinix/PyxForge/internal/ui/commandpalette"
	"github.com/obstinix/PyxForge/internal/ui/icons"
	"github.com/obstinix/PyxForge/internal/ui/kit"
	"github.com/obstinix/PyxForge/internal/ui/mdview"
	"github.com/obstinix/PyxForge/internal/ui/notifications"
	"github.com/obstinix/PyxForge/internal/ui/theme"
)

// notesHost is Forge Notes in the shell (D25, identity prompt Phase 6): the index of notes'
// links, the Backlinks tab of the inspector, Markdown previews, and the note commands.
type notesHost struct {
	s        *Shell
	idx      *notes.Index
	building bool
	waiting  []func()

	backTab  *container.TabItem
	backBody *fyne.Container
	backFor  string

	previews map[string]*notePreview // by file path
	lastNote string                  // the note last active in the editor
}

// notePreview is a Markdown preview tab.
type notePreview struct {
	path string
	view *mdview.View
	item *container.TabItem
}

func (s *Shell) notesHost() *notesHost {
	if s.forge == nil {
		s.forge = &notesHost{s: s, previews: map[string]*notePreview{}}
	}
	return s.forge
}

// notesDir is the project's notes folder, slash-separated from the root ([notes] dir).
func (s *Shell) notesDir() string {
	if cfg, err := config.Load(s.root); err == nil && cfg.Notes.Dir != "" {
		return cfg.Notes.Dir
	}
	return config.DefaultNotesDir
}

// withIndex runs f once the notes index is built, building it off the UI thread first.
func (h *notesHost) withIndex(f func()) {
	if h.s.root == "" {
		return
	}
	if h.idx != nil {
		f()
		return
	}
	h.waiting = append(h.waiting, f)
	if h.building {
		return
	}
	h.building = true
	root, dir := h.s.root, h.s.notesDir()
	go func() {
		idx, err := notes.Build(root, dir, nil)
		h.s.dispatch(func() {
			h.building = false
			if err != nil {
				h.waiting = nil
				h.s.logf("Notes: cannot index the project: %v", err)
				return
			}
			h.idx = idx
			for _, w := range h.waiting {
				w()
			}
			h.waiting = nil
		})
	}()
}

// filesChanged is the explorer's watcher: the index re-reads what changed, and the open
// previews of changed files are drawn again.
func (h *notesHost) filesChanged(paths []string) {
	if h == nil {
		return
	}
	if h.idx != nil && h.idx.Update(paths) {
		h.backFor = ""
		h.syncBacklinks()
	}
	for _, p := range paths {
		if pv := h.previews[p]; pv != nil && h.s.tabFor(p) == nil { // not open in the editor: read the disk
			if data, err := os.ReadFile(p); err == nil {
				pv.view.SetText(string(data))
			}
		}
	}
}

// rel is a path's slash-separated form inside the project, "" outside.
func (h *notesHost) rel(p string) string {
	r, ok := relWithin(h.s.root, p)
	if !ok {
		return ""
	}
	return r
}

// Backlinks: a tab of the inspector listing the notes that link to the active file, with the
// line of each link; for a note, its unlinked mentions and the links that lead nowhere.

func (s *Shell) buildBacklinks() *container.TabItem {
	h := s.notesHost()
	h.backBody = container.NewVBox()
	h.backTab = container.NewTabItem("Backlinks", container.NewVScroll(
		container.New(layout.NewCustomPaddedLayout(theme.Space2, theme.Space2, theme.Space2, theme.Space2), h.backBody)))
	return h.backTab
}

// syncBacklinks fills the Backlinks tab for the active file, when it is on view.
func (h *notesHost) syncBacklinks() {
	if h == nil || h.backTab == nil {
		return
	}
	s := h.s
	if s.inspectorTabs == nil || s.inspectorTabs.Selected() != h.backTab || !s.bench.inspectorOn {
		return
	}
	p := s.activeFile()
	rel := h.rel(p)
	if rel == h.backFor && h.backFor != "" {
		return
	}
	h.withIndex(func() {
		h.backFor = rel
		h.backBody.Objects = h.backlinkRows(rel)
		h.backBody.Refresh()
	})
}

func (h *notesHost) backlinkRows(rel string) []fyne.CanvasObject {
	note := func(text string) fyne.CanvasObject { return kit.NewParagraph(text, kit.Tertiary) }
	if rel == "" {
		return []fyne.CanvasObject{note("Open a file of the project to see which notes link to it.")}
	}
	var rows []fyne.CanvasObject
	section := func(title string) {
		rows = append(rows, kit.NewText(strings.ToUpper(title), kit.Label, kit.Tertiary))
	}
	row := func(where, context string, open func()) fyne.CanvasObject {
		b := kit.NewButton(where, open)
		b.Variant = kit.ButtonGhost
		return container.NewVBox(container.NewHBox(b), container.New(layout.NewCustomPaddedLayout(0, theme.Space2, theme.Space3, 0),
			kit.NewParagraph(context, kit.Secondary)))
	}
	back := h.idx.Backlinks(rel)
	section(fmt.Sprintf("Linked from %d", len(back)))
	if len(back) == 0 {
		rows = append(rows, note("No note links to "+filepath.Base(rel)+". A [["+filepath.Base(rel)+"]] in a note would."))
	}
	for _, b := range back {
		b := b
		rows = append(rows, row(fmt.Sprintf("%s:%d", b.Note, b.Line+1), b.Context, func() { h.openAt(b.Note, b.Line) }))
	}
	if !strings.HasSuffix(strings.ToLower(rel), ".md") {
		return rows
	}
	mentions := h.idx.UnlinkedMentions(rel)
	section(fmt.Sprintf("Unlinked mentions %d", len(mentions)))
	for _, m := range mentions {
		m := m
		rows = append(rows, row(fmt.Sprintf("%s (line %d)", m.Name, m.Line+1), m.Context, func() { h.openAt(m.Path, 0) }))
	}
	if issues := h.idx.Issues(rel); len(issues) > 0 {
		section(fmt.Sprintf("Links that lead nowhere %d", len(issues)))
		for _, b := range issues {
			b := b
			why := "nothing has that name"
			switch {
			case len(b.To.Ambiguous) > 0:
				why = "several files have that name: " + strings.Join(b.To.Ambiguous, ", ")
			case b.To.NoSymbol:
				why = "no definition of " + b.Link.Symbol + " in " + b.To.Path
			}
			rows = append(rows, row(fmt.Sprintf("[[%s]] (line %d)", b.Link.Raw, b.Line+1), why, func() { h.openAt(rel, b.Line) }))
		}
	}
	return rows
}

// openAt opens a project file (slash-separated) at a 0-based line.
func (h *notesHost) openAt(rel string, line int) {
	h.s.openAt(filepath.Join(h.s.root, filepath.FromSlash(rel)), line, 0)
}

// Links

// followLink opens what a [[link]] in note (a project path) leads to: a file at its line or
// symbol, a note; an ambiguous name lists the files to choose from; a missing one says so. A
// symbol goes through the language server when the file has one, else the definition found by
// text search, said to be approximate.
func (h *notesHost) followLink(note string, l notes.Link) {
	h.withIndex(func() {
		r := h.idx.Resolve(note, l)
		switch {
		case len(r.Ambiguous) > 0:
			var items []commandpalette.Item
			for _, p := range r.Ambiguous {
				p := p
				items = append(items, commandpalette.Item{Title: filepath.Base(p), Detail: p, Icon: icons.File, Run: func() {
					h.openAt(p, max(l.Line-1, 0))
				}})
			}
			h.s.palette.Show("Several files are called "+l.Target+": choose one", items)
		case r.Path == "":
			h.s.Notify(notifications.Info, "Nothing called "+l.Target, "No file or note in the project has that name.")
		case l.Symbol != "":
			h.openSymbol(r, l.Symbol)
		default:
			h.openAt(r.Path, max(r.Line-1, 0))
		}
	})
}

// openSymbol opens a file at a symbol: the language server's answer when the file has one,
// else the text search's line.
func (h *notesHost) openSymbol(r notes.Resolved, symbol string) {
	s := h.s
	abs := filepath.Join(s.root, filepath.FromSlash(r.Path))
	fallback := func() {
		if r.NoSymbol {
			s.Notify(notifications.Info, "No definition of "+symbol, "Neither a language server nor a search of "+filepath.Base(r.Path)+" finds it.")
			h.openAt(r.Path, 0)
			return
		}
		h.openAt(r.Path, max(r.Line-1, 0))
		s.logf("Notes: %s#%s found by text search (approximate)", r.Path, symbol)
	}
	if !s.startNative() {
		fallback()
		return
	}
	d := s.nat.open(abs, true)
	if d == nil || d.client == nil || d.uri == "" {
		fallback()
		return
	}
	c, uri := d.client, d.uri
	go func() {
		ctx, cancel := lsp.Context()
		defer cancel()
		syms, err := c.DocumentSymbols(ctx, uri)
		s.dispatch(func() {
			if err == nil {
				for _, sym := range syms {
					if sym.Name == symbol {
						if dd := s.nat.open(abs, true); dd != nil {
							dd.ed.Reveal(codeedit.Pos{Line: sym.Selection.Start.Line, Col: sym.Selection.Start.Character})
						}
						return
					}
				}
			}
			fallback()
		})
	}()
}

// Preview

// openPreview shows a Markdown file's preview in its own tab, in the editor group to the
// right of the file's (D20), drawn again as the file is edited.
func (s *Shell) openPreview() {
	p := s.activeFile()
	if l := strings.ToLower(p); !strings.HasSuffix(l, ".md") && !strings.HasSuffix(l, ".markdown") {
		s.Notify(notifications.Info, "Not a Markdown file", "Markdown Preview shows a note or another .md file: open one first.")
		return
	}
	h := s.notesHost()
	pv := h.previews[p]
	if pv != nil && s.hasTab(pv.item) {
		s.selectTab(pv.item, false)
		return
	}
	pv = &notePreview{path: p, view: mdview.New()}
	note := h.rel(p)
	pv.view.OnLink = func(l mdview.Link) { h.previewLink(note, p, l) }
	pv.item = container.NewTabItemWithIcon("Preview: "+filepath.Base(p), icons.Get(icons.FileText, theme.Current().Text.Secondary), pv.view)
	h.previews[p] = pv
	h.refreshPreview(p)
	from := slices.Index(s.groups, s.editors)
	s.selectQuietly = true
	s.editors.Append(pv.item)
	s.selectQuietly = false
	s.syncEditor()
	s.selectTab(pv.item, false)
	s.moveToGroup(pv.item, from+1) // beside the note
}

// refreshPreview draws a preview from the editor's text, or the disk's.
func (h *notesHost) refreshPreview(p string) {
	pv := h.previews[p]
	if pv == nil {
		return
	}
	if d := h.s.docFor(p); d != nil {
		pv.view.SetText(d.ed.Buf.Text())
		return
	}
	if data, err := os.ReadFile(p); err == nil {
		pv.view.SetText(string(data))
	}
}

// previewLink follows a link clicked in a preview: a [[wikilink]] as a note's link, a
// Markdown link to a file of the project, or a web address in the system's browser.
func (h *notesHost) previewLink(note, file string, l mdview.Link) {
	if l.Wiki {
		h.followLink(note, notes.ParseLink(l.Target))
		return
	}
	if strings.Contains(l.Target, "://") {
		if u, err := parseURL(l.Target); err == nil {
			_ = fyne.CurrentApp().OpenURL(u)
		}
		return
	}
	target := l.Target
	if i := strings.IndexAny(target, "#?"); i >= 0 {
		target = target[:i]
	}
	abs := filepath.Clean(filepath.Join(filepath.Dir(file), filepath.FromSlash(target)))
	if _, ok := relWithin(h.s.root, abs); !ok {
		h.s.Notify(notifications.Info, "Outside the project", target+" is not a file of this project.")
		return
	}
	if _, err := os.Stat(abs); err != nil {
		h.s.Notify(notifications.Info, "No such file", target+" does not exist.")
		return
	}
	h.s.OpenFile(abs)
}

// Commands

// newNote asks for a name and makes notes/<name>.md, creating the notes folder with its first
// note. body, when set, is the note's text after its title.
func (s *Shell) newNote(body string) {
	if s.root == "" {
		s.Notify(notifications.Info, "No folder open", "Notes live in a project's notes folder: open a folder first.")
		return
	}
	entry := s.newDialogEntry()
	entry.SetPlaceHolder("boot-hang-investigation")
	d := escForm("New Note", "Create", "Cancel", []*widget.FormItem{widget.NewFormItem("Name", entry)}, func(ok bool) {
		if ok {
			s.createNote(entry.Text, body)
		}
	}, s.win)
	d.Resize(fyne.NewSize(380, d.MinSize().Height))
	s.submitForm(d, entry)
	s.win.Canvas().Focus(entry)
	s.lastDialog = d
}

// createNote writes a note and opens it; an existing note of that name is opened instead.
func (s *Shell) createNote(name, body string) string {
	name = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(name), ".md"))
	if name == "" || strings.ContainsAny(name, `\/:*?"<>|`) {
		s.Notify(notifications.Info, "Not a note name", "A note's name is a file name without slashes or : * ? \" < > |.")
		return ""
	}
	dir := filepath.Join(s.root, filepath.FromSlash(s.notesDir()))
	p := filepath.Join(dir, name+".md")
	if _, err := os.Stat(p); err == nil {
		s.OpenFile(p)
		return p
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		s.Notify(notifications.Error, "Cannot make the notes folder", err.Error())
		return ""
	}
	text := "# " + name + "\n\n" + body
	if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
		s.Notify(notifications.Error, "Cannot write the note", err.Error())
		return ""
	}
	s.forge.filesChanged([]string{p})
	s.OpenFile(p)
	return p
}

// noteFromSelection makes a note of the selected code: a fenced block, a [[file:line]] link to
// where it came from, and the date.
func (s *Shell) noteFromSelection() {
	d := s.activeDoc()
	if d == nil || d.path == "" || !d.ed.HasSelection() {
		s.Notify(notifications.Info, "Nothing selected", "Select code in a file, then New Note from Selection.")
		return
	}
	h := s.notesHost()
	sel := d.ed.Selection()
	text := d.ed.SelectedText()
	path, line := d.path, sel.Start.Line+1
	lang := strings.TrimPrefix(strings.ToLower(filepath.Ext(path)), ".")
	h.withIndex(func() {
		s.newNote(selectionNote(h.idx.LinkTo(h.rel(path), line), lang, text))
	})
}

// selectionNote is the text of a note made from selected code: where it came from, the date,
// and the code in a fenced block.
func selectionNote(link, lang, code string) string {
	return fmt.Sprintf("From %s on %s:\n\n```%s\n%s\n```\n", link, now().Format("2006-01-02"), lang, strings.TrimRight(code, "\n"))
}

// devLogTemplate is the template a dev log starts from, until the user edits their own copy in
// the notes folder (templates/dev-log.md); {{date}} becomes the day's date.
const devLogTemplate = "# Dev log {{date}}\n\n## Done\n\n- \n\n## Next\n\n- \n\n## Notes\n\n"

// openDevLog opens today's dev log, notes/log/YYYY-MM-DD.md, making it from the template
// (notes/templates/dev-log.md, written with the first log so it can be edited).
func (s *Shell) openDevLog() {
	if s.root == "" {
		s.Notify(notifications.Info, "No folder open", "The dev log lives in a project's notes folder: open a folder first.")
		return
	}
	dir := filepath.Join(s.root, filepath.FromSlash(s.notesDir()))
	day := now().Format("2006-01-02")
	p := filepath.Join(dir, "log", day+".md")
	if _, err := os.Stat(p); err != nil {
		tpl := filepath.Join(dir, "templates", "dev-log.md")
		data, err := os.ReadFile(tpl)
		if err != nil {
			data = []byte(devLogTemplate)
			if os.MkdirAll(filepath.Dir(tpl), 0o755) == nil {
				_ = os.WriteFile(tpl, data, 0o644)
			}
		}
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			s.Notify(notifications.Error, "Cannot make the log folder", err.Error())
			return
		}
		if err := os.WriteFile(p, []byte(strings.ReplaceAll(string(data), "{{date}}", day)), 0o644); err != nil {
			s.Notify(notifications.Error, "Cannot write the dev log", err.Error())
			return
		}
		s.forge.filesChanged([]string{p, tpl})
	}
	s.OpenFile(p)
}

// insertLink puts a [[file:line]] link to the cursor's place into the note last active, at its
// cursor; with no note open, it goes to the clipboard.
func (s *Shell) insertLink() {
	d := s.activeDoc()
	if d == nil || d.path == "" {
		s.Notify(notifications.Info, "No file", "Insert Link to Current Location links to a place in a file open in the editor.")
		return
	}
	h := s.notesHost()
	path, line := d.path, d.ed.Cur.Line+1
	h.withIndex(func() {
		link := h.idx.LinkTo(h.rel(path), line)
		if nd := s.docFor(h.lastNote); nd != nil && h.lastNote != path {
			nd.ed.Paste(link)
			nd.ed.Refresh()
			if nd.ed.OnEdit != nil {
				nd.ed.OnEdit()
			}
			s.Notify(notifications.Success, "Link inserted", link+" is in "+filepath.Base(h.lastNote)+".")
			return
		}
		fyne.CurrentApp().Clipboard().SetContent(link)
		s.Notify(notifications.Info, "Link copied", link+" is on the clipboard: no note is open to put it in.")
	})
}

// noteActivated records the note last active, for Insert Link to Current Location, and keeps
// the Backlinks tab on the active file.
func (h *notesHost) activeChanged() {
	if h == nil {
		return
	}
	if p := h.s.activeFile(); strings.HasSuffix(strings.ToLower(p), ".md") {
		h.lastNote = p
	}
	h.syncBacklinks()
}

// docFor is the editor document of a path, nil when it is not open.
func (s *Shell) docFor(p string) *nativeDoc {
	if s.nat == nil || p == "" {
		return nil
	}
	for _, d := range s.nat.docs {
		if d.path == p {
			return d
		}
	}
	return nil
}

// now is the clock note commands date with; tests replace it.
var now = time.Now

func parseURL(s string) (*url.URL, error) { return url.Parse(s) }

// ShowBacklinks opens the inspector at Backlinks; for the palette and review renders.
func (s *Shell) ShowBacklinks() {
	h := s.notesHost()
	if !s.bench.inspectorOn {
		s.ToggleInspector()
	}
	s.inspectorTabs.Select(h.backTab)
	h.backFor = ""
	h.syncBacklinks()
}

// BacklinksShown reports whether the Backlinks tab has its content; for review renders.
func (s *Shell) BacklinksShown() bool {
	h := s.notesHost()
	return h.idx != nil && h.backBody != nil && len(h.backBody.Objects) > 0
}
