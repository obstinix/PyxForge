// Package explorer shows the workspace as a lazily loaded file tree that follows changes on
// disk, and creates files and folders.
package explorer

import (
	"errors"
	"fmt"
	"image/color"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"github.com/obstinix/PyxForge/internal/crash"
	"github.com/obstinix/PyxForge/internal/ui/icons"
	"github.com/obstinix/PyxForge/internal/ui/kit"
	"github.com/obstinix/PyxForge/internal/ui/theme"
)

// DefaultHidden are the folders never listed unless the user changes the set (Settings > Files &
// Explorer): version-control internals and build caches.
var DefaultHidden = []string{".git", "node_modules", "target", ".impeccable"}

// hiddenSet is the set in force. Workers read it, so it is replaced whole, never changed.
var hiddenSet atomic.Pointer[map[string]bool]

func init() { SetHidden(DefaultHidden) }

// SetHidden replaces the names of the folders never listed, in the tree and in Go to File.
func SetHidden(names []string) {
	m := map[string]bool{}
	for _, n := range names {
		if n = strings.TrimSpace(n); n != "" {
			m[n] = true
		}
	}
	hiddenSet.Store(&m)
}

func hidden(name string) bool { return (*hiddenSet.Load())[name] }

// DefaultShowLimit is how many entries of a folder are listed before a "Show all" row.
const DefaultShowLimit = 500

// syncWait is how long a listing may take before the tree shows "Loading…" and goes on: a
// local folder lists well within it and shows at once; a slow drive or network share does not
// hold the window up (owner review F1).
const syncWait = 30 * time.Millisecond

// The tree's own rows that are not files: a folder being listed, and the end of a long one.
const (
	loadingRow = "\x00loading"
	moreRow    = "\x00more"
)

func synthetic(id string) bool { return strings.Contains(id, "\x00") }

func rowIn(parent, row string) string {
	if parent == "" {
		return row
	}
	return parent + "/" + row
}

// Explorer is the file tree. Node IDs are slash-separated paths relative to Root; "" is Root.
type Explorer struct {
	Root string
	// OnOpen is called with an absolute path when a file is opened with a click or Enter: the
	// editor should take keyboard focus.
	OnOpen func(path string)
	// OnPreview is called when a file is opened with Space, which keeps keyboard focus in the
	// tree for further browsing. Nil means OnOpen.
	OnPreview func(path string)
	OnErr     func(err error) // called when a directory cannot be read
	// OnFilesChanged is called on the UI thread with the files the watcher saw change on disk,
	// in folders the tree has listed.
	OnFilesChanged func(paths []string)
	OnFocus        func(bool) // called when the tree gains or loses keyboard focus
	// OnContext is called when a row is clicked with the secondary button: its absolute path,
	// whether it is a folder, and where on the canvas the click was, for a context menu.
	OnContext func(path string, dir bool, at fyne.Position)
	// OnRename and OnTrash are called for F2 and Delete on the highlighted row (or, with none,
	// the active file's): its absolute path and whether it is a folder.
	OnRename, OnTrash func(path string, dir bool)
	// OnDrag is called while a row is dragged, with its absolute path, whether it is a folder,
	// and the pointer's absolute position; OnDragEnd when it is let go, with where.
	OnDrag, OnDragEnd func(path string, dir bool, at fyne.Position)
	// OnUndo is called for Ctrl+Z while the tree has focus: undo the last move.
	OnUndo func()
	// Dispatch runs a listing's result on the UI thread; nil means fyne.Do.
	Dispatch func(func())
	// ShowLimit is how many entries of a folder are listed before a "Show all" row; 0 means
	// DefaultShowLimit.
	ShowLimit int

	tree     *tree
	children map[string][]string
	isDir    map[string]bool
	watch    *watcher
	active   string          // the row of the file shown in the editor, drawn selected
	selected string          // the row the pointer last selected, drawn selected too
	preview  bool            // the row being activated was activated with Space
	peek     *string         // when set, activating a row stores its id here and does nothing else
	loading  map[string]bool // folders being listed on a worker
	stale    map[string]bool // folders whose listing is shown until a new one arrives
	showAll  map[string]bool // folders listed past ShowLimit
	gen      int             // counts Reloads: a listing from before one is dropped
	rows     map[*row]bool   // the row widgets the tree made, for FolderAt
	target   string          // the folder a drag would drop into, outlined; "" for none
	dragAt   fyne.Position   // where the dragged row's pointer is
}

// New returns an explorer rooted at root.
func New(root string) *Explorer {
	e := &Explorer{Root: root, children: map[string][]string{}, isDir: map[string]bool{"": true},
		loading: map[string]bool{}, stale: map[string]bool{}, showAll: map[string]bool{}, rows: map[*row]bool{}, target: "\x00"}
	e.tree = &tree{e: e, onFocus: func(on bool) {
		if e.OnFocus != nil {
			e.OnFocus(on)
		}
	}}
	e.tree.ChildUIDs, e.tree.IsBranch = e.childUIDs, e.branch
	e.tree.CreateNode, e.tree.UpdateNode = e.create, e.update
	e.tree.ExtendBaseWidget(e.tree)
	e.tree.HideSeparators = true
	// Fyne's tree reports nothing when the selected row is tapped again, so no row stays
	// selected: every tap opens. The file in the editor is drawn selected instead (SetActive).
	e.tree.OnSelected = func(id widget.TreeNodeID) {
		e.tree.UnselectAll()
		if e.peek != nil {
			*e.peek = id
			return
		}
		e.activate(id)
	}
	return e
}

// activate opens a row: a folder opens or closes, a file opens (Space previews it).
func (e *Explorer) activate(id widget.TreeNodeID) {
	if e.syntheticRow(id) {
		return
	}
	if e.isDir[id] {
		e.tree.ToggleBranch(id)
		return
	}
	p := filepath.Join(e.Root, filepath.FromSlash(id))
	switch {
	case e.preview && e.OnPreview != nil:
		e.OnPreview(p)
	case e.OnOpen != nil:
		e.OnOpen(p)
	}
}

// click is a single click on a row: the tree takes keyboard focus, the row is selected, and a
// folder opens or closes.
func (e *Explorer) click(id widget.TreeNodeID, dir bool) {
	if e.syntheticRow(id) {
		return
	}
	if c := fyne.CurrentApp().Driver().CanvasForObject(e.tree); c != nil {
		c.Focus(e.tree)
	}
	e.selected = id
	if dir {
		e.tree.ToggleBranch(id)
	}
	e.tree.Refresh()
}

// syntheticRow handles a click on a row that is not a file: "Show all" lists the rest of its
// folder; "Loading…" does nothing. It reports whether id was such a row.
func (e *Explorer) syntheticRow(id string) bool {
	if !synthetic(id) {
		return false
	}
	if strings.HasSuffix(id, moreRow) {
		e.showAll[parentID(id)] = true
		e.tree.Refresh()
	}
	return true
}

// FolderAt is the folder a drop at the absolute position at would go into: the folder under
// the pointer, a file's folder, or Root below the last row. ok is false outside the tree.
func (e *Explorer) FolderAt(at fyne.Position) (string, bool) {
	d := fyne.CurrentApp().Driver()
	tp, ts := d.AbsolutePositionForObject(e.tree), e.tree.Size()
	if !e.tree.Visible() || at.X < tp.X || at.Y < tp.Y || at.X > tp.X+ts.Width || at.Y > tp.Y+ts.Height {
		return "", false
	}
	for r := range e.rows {
		if !r.Visible() || r.id == "" || synthetic(r.id) {
			continue
		}
		p, s := d.AbsolutePositionForObject(r), r.Size()
		if at.Y >= p.Y && at.Y < p.Y+s.Height && at.X >= tp.X && at.X <= tp.X+ts.Width && s.Height > 0 {
			id := r.id
			if !r.dir {
				id = parentID(id)
			}
			return e.abs(id), true
		}
	}
	return e.Root, true
}

// SetDropTarget outlines the folder a drag would drop into ("" is Root; "\x00" none).
func (e *Explorer) SetDropTarget(path string) {
	id := "\x00"
	if path != "" {
		if rel, err := filepath.Rel(e.Root, path); err == nil && !strings.HasPrefix(rel, "..") {
			id = filepath.ToSlash(rel)
			if id == "." {
				id = ""
			}
		}
	}
	if id != e.target {
		e.target = id
		e.tree.Refresh()
	}
}

// Selected is the row last selected with the pointer, relative to Root, or "".
func (e *Explorer) Selected() string { return e.selected }

// Widget is the tree to place in a panel.
func (e *Explorer) Widget() fyne.CanvasObject { return e.tree }

// Focus gives the tree keyboard focus on canvas c.
func (e *Explorer) Focus(c fyne.Canvas) { c.Focus(e.tree) }

// tree is Fyne's tree with a focus callback, so the shell can show where keyboard focus is
// (Fyne marks the keyboard-highlighted row with the hover wash only), and with Enter and
// Space opening the highlighted file: Enter for editing, Space to preview it.
type tree struct {
	widget.Tree
	e       *Explorer
	onFocus func(bool)
}

func (t *tree) TypedKey(ev *fyne.KeyEvent) {
	switch ev.Name {
	case fyne.KeyReturn, fyne.KeyEnter:
		t.Tree.TypedKey(&fyne.KeyEvent{Name: fyne.KeySpace}) // Fyne's tree activates on Space
	case fyne.KeySpace:
		t.e.preview = true
		t.Tree.TypedKey(ev)
		t.e.preview = false
	case fyne.KeyF2, fyne.KeyDelete:
		f := t.e.OnRename
		if ev.Name == fyne.KeyDelete {
			f = t.e.OnTrash
		}
		if id := t.highlighted(); id != "" && f != nil && t.e.Root != "" {
			f(filepath.Join(t.e.Root, filepath.FromSlash(id)), t.e.isDir[id])
		}
	default:
		t.Tree.TypedKey(ev)
	}
}

// highlighted is the row the keyboard is on, or else the active file's row, or "". Fyne keeps
// the keyboard's row to itself, so it is read by activating it with Space while e.peek is set.
func (t *tree) highlighted() string {
	var id string
	t.e.peek = &id
	t.Tree.TypedKey(&fyne.KeyEvent{Name: fyne.KeySpace})
	t.e.peek = nil
	if id == "" {
		id = t.e.active
	}
	return id
}

// SetActive draws the row of path, the file shown in the editor, selected; "" or a path
// outside the workspace clears it.
func (e *Explorer) SetActive(path string) {
	id := ""
	if path != "" {
		if rel, err := filepath.Rel(e.Root, path); err == nil && !strings.HasPrefix(rel, "..") && rel != "." {
			id = filepath.ToSlash(rel)
		}
	}
	if id == e.active {
		return
	}
	e.active = id
	e.tree.Refresh()
}

// Active returns the row drawn selected, relative to Root, or "".
func (e *Explorer) Active() string { return e.active }

// TypedShortcut takes Ctrl+Z, which undoes the last move by drag (OnUndo).
func (t *tree) TypedShortcut(sc fyne.Shortcut) {
	if _, ok := sc.(*fyne.ShortcutUndo); ok && t.e.OnUndo != nil {
		t.e.OnUndo()
	}
}

func (t *tree) FocusGained() {
	t.Tree.FocusGained()
	t.onFocus(true)
}

func (t *tree) FocusLost() {
	t.Tree.FocusLost()
	t.onFocus(false)
}

// Files lists every regular file under Root (relative, slash-separated) for quick open, with
// the same hiding rules as the tree, so every file the tree shows can be found by name. It
// stops after limit entries and reports whether it did.
func (e *Explorer) Files(limit int) (files []string, truncated bool) { return Files(e.Root, limit) }

// Files lists the files under root as Explorer.Files does. It touches no explorer, so it can
// run on a worker.
func Files(root string, limit int) (files []string, truncated bool) {
	if root == "" {
		return nil, false
	}
	_ = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // unreadable entries are skipped, not fatal
		}
		if d.IsDir() && p != root && hidden(d.Name()) {
			return filepath.SkipDir
		}
		if !d.IsDir() {
			if len(files) == limit {
				truncated = true
				return filepath.SkipAll
			}
			rel, _ := filepath.Rel(root, p)
			files = append(files, filepath.ToSlash(rel))
		}
		return nil
	})
	return files, truncated
}

// Reload drops cached listings and redraws the tree.
func (e *Explorer) Reload() {
	e.gen++
	e.children = map[string][]string{}
	e.isDir = map[string]bool{"": true}
	e.loading, e.stale = map[string]bool{}, map[string]bool{}
	e.tree.Refresh()
}

// childUIDs is what the tree shows under a folder. A folder not yet listed is listed on a
// worker: if that ends within syncWait the rows show at once, else a "Loading…" row stands in
// until it does. A folder listed before and changed on disk keeps its rows until the new
// listing arrives.
func (e *Explorer) childUIDs(id widget.TreeNodeID) []widget.TreeNodeID {
	if e.Root == "" || synthetic(id) {
		return nil
	}
	if kids, ok := e.children[id]; ok && !e.stale[id] {
		return e.capped(id, kids)
	}
	if !e.loading[id] {
		e.load(id, syncWait)
	}
	if kids, ok := e.children[id]; ok {
		return e.capped(id, kids)
	}
	return []string{rowIn(id, loadingRow)}
}

// listNow lists a folder and waits for it, for Reveal and Expand, which need its rows.
func (e *Explorer) listNow(id string) []string {
	if _, ok := e.children[id]; !ok || e.stale[id] {
		e.load(id, 10*time.Second)
	}
	return e.children[id]
}

// capped is a folder's rows, ending in a "Show all" row past ShowLimit.
func (e *Explorer) capped(id string, kids []string) []string {
	limit := e.limit()
	if len(kids) <= limit || e.showAll[id] {
		return kids
	}
	return append(kids[:limit:limit], rowIn(id, moreRow))
}

func (e *Explorer) limit() int {
	if e.ShowLimit > 0 {
		return e.ShowLimit
	}
	return DefaultShowLimit
}

// entry is one listed name; listing is a folder's entries, read on a worker.
type entry struct {
	name string
	dir  bool
}

type listing struct {
	entries []entry
	err     error
}

var errListingFailed = errors.New("listing the folder failed")

// listDir is list; tests replace it with a slow one.
var listDir = list

// list reads a folder, folders first, by name without case, skipping hidden folders.
func list(dir string) listing {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return listing{err: err}
	}
	out := make([]entry, 0, len(ents))
	for _, en := range ents {
		if en.IsDir() && hidden(en.Name()) {
			continue
		}
		out = append(out, entry{en.Name(), en.IsDir()})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].dir != out[j].dir {
			return out[i].dir
		}
		return strings.ToLower(out[i].name) < strings.ToLower(out[j].name)
	})
	return listing{entries: out}
}

// load lists a folder on a worker and waits up to wait for it; a later result is applied on
// the UI thread.
func (e *Explorer) load(id string, wait time.Duration) {
	e.loading[id] = true
	done := make(chan listing, 1)
	dir := e.abs(id)
	crash.Go("Explorer listing", func() {
		l := listing{err: errListingFailed} // what is sent if list panics
		defer func() { done <- l }()
		l = listDir(dir)
	})
	select {
	case l := <-done:
		e.apply(id, e.gen, l)
	case <-time.After(wait):
		gen := e.gen
		go func() {
			l := <-done
			e.dispatch(func() {
				e.apply(id, gen, l)
				e.tree.Refresh()
			})
		}()
	}
}

func (e *Explorer) dispatch(f func()) {
	if e.Dispatch != nil {
		e.Dispatch(f)
		return
	}
	fyne.Do(f)
}

// apply stores a folder's listing, on the UI thread.
func (e *Explorer) apply(id string, gen int, l listing) {
	if gen != e.gen {
		return // listed before a Reload
	}
	delete(e.loading, id)
	delete(e.stale, id)
	if l.err != nil {
		if e.OnErr != nil {
			e.OnErr(fmt.Errorf("%s: %w", filepath.Base(e.abs(id)), l.err))
		}
		e.children[id] = nil
		return
	}
	kids := make([]string, len(l.entries))
	for i, en := range l.entries {
		kids[i] = rowIn(id, en.name)
		e.isDir[kids[i]] = en.dir
	}
	e.children[id] = kids
	e.follow(id)
}

func (e *Explorer) branch(id widget.TreeNodeID) bool { return e.isDir[id] }

func (e *Explorer) create(bool) fyne.CanvasObject {
	icon := kit.NewIcon(icons.File, kit.Secondary)
	name := kit.NewText("", kit.Body, kit.Primary)
	wash := canvas.NewRectangle(color.Transparent)
	wash.CornerRadius = theme.RadiusControl
	r := &row{e: e, box: container.NewStack(wash, container.NewHBox(icon, name))}
	r.ExtendBaseWidget(r)
	e.rows[r] = true
	return r
}

// row is one line of the tree. It takes the primary click itself: Fyne's GL driver gives a
// click to the topmost object that takes any mouse event (this row, for its context menu), and
// drops it unless that object is Tappable, so a row that only took secondary clicks swallowed
// every left click (owner review F2). One click selects (and opens or closes a folder); a
// double click opens a file.
type row struct {
	widget.BaseWidget
	e   *Explorer
	box *fyne.Container
	id  widget.TreeNodeID
	dir bool
}

func (r *row) CreateRenderer() fyne.WidgetRenderer { return widget.NewSimpleRenderer(r.box) }

// Tapped selects the row: a folder opens or closes, a file is marked selected.
func (r *row) Tapped(*fyne.PointEvent) { r.e.click(r.id, r.dir) }

// DoubleTapped opens a file (a folder opens or closes, as a single click does).
func (r *row) DoubleTapped(*fyne.PointEvent) {
	if r.dir {
		r.e.click(r.id, true)
		return
	}
	r.e.activate(r.id)
}

// Dragged drags the row: the shell draws it under the pointer and says where it would go.
func (r *row) Dragged(ev *fyne.DragEvent) {
	if synthetic(r.id) || r.id == "" || r.e.OnDrag == nil {
		return
	}
	r.e.dragAt = ev.AbsolutePosition
	r.e.OnDrag(r.e.abs(r.id), r.dir, ev.AbsolutePosition)
}

// DragEnd drops the row where the pointer is.
func (r *row) DragEnd() {
	if synthetic(r.id) || r.id == "" || r.e.OnDragEnd == nil {
		return
	}
	r.e.OnDragEnd(r.e.abs(r.id), r.dir, r.e.dragAt)
}

func (r *row) TappedSecondary(ev *fyne.PointEvent) {
	if r.e.OnContext != nil && r.e.Root != "" {
		r.e.OnContext(filepath.Join(r.e.Root, filepath.FromSlash(r.id)), r.dir, ev.AbsolutePosition)
	}
}

func (e *Explorer) update(id widget.TreeNodeID, branch bool, o fyne.CanvasObject) {
	r := o.(*row)
	r.id, r.dir = id, branch
	row := r.box
	wash := row.Objects[0].(*canvas.Rectangle)
	line := row.Objects[1].(*fyne.Container)
	icon := line.Objects[0].(*kit.Icon)
	name := line.Objects[1].(*kit.Text)
	switch {
	case strings.HasSuffix(id, loadingRow):
		icon.Hide()
		name.Role = kit.Tertiary
		name.SetText("Loading…")
	case strings.HasSuffix(id, moreRow):
		icon.Hide()
		name.Role = kit.Accent
		name.SetText(fmt.Sprintf("Show all (%d)", len(e.children[parentID(id)])))
	default:
		icon.Show()
		icon.Name = fileIcon(id, branch)
		icon.Refresh()
		name.Role = kit.Primary
		name.SetText(filepath.Base(filepath.FromSlash(id)))
	}
	if id != "" && id == e.target {
		wash.StrokeColor, wash.StrokeWidth = theme.Current().Accent.Primary, 1
	} else if wash.StrokeWidth != 0 {
		wash.StrokeWidth = 0
	}
	var fill color.Color = color.Transparent
	switch {
	case id != "" && id == e.active:
		fill = theme.Current().State.Selected
	case id != "" && id == e.selected:
		fill = theme.Current().State.Hover
	}
	if wash.FillColor != fill {
		wash.FillColor = fill
		wash.Refresh()
	}
}

func fileIcon(id string, dir bool) icons.Name {
	if dir {
		return icons.Folder
	}
	switch strings.ToLower(filepath.Ext(id)) {
	case ".asm", ".s", ".c", ".h", ".cpp", ".hpp", ".rs", ".go", ".lua", ".ld", ".sh", ".py":
		return icons.FileCode
	case ".bin", ".img", ".elf", ".o", ".iso":
		return icons.Binary
	case ".md", ".txt", ".toml", ".json", ".yml", ".yaml":
		return icons.FileText
	}
	return icons.File
}
