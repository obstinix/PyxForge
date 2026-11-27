// Package explorer shows the workspace as a lazily loaded file tree that follows changes on
// disk, and creates files and folders.
package explorer

import (
	"image/color"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"github.com/obstinix/PyxForge/internal/ui/icons"
	"github.com/obstinix/PyxForge/internal/ui/kit"
	"github.com/obstinix/PyxForge/internal/ui/theme"
)

// Hidden directories are never listed: version-control internals and build caches.
var hidden = map[string]bool{".git": true, "node_modules": true, "target": true, ".impeccable": true}

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

	tree     *tree
	children map[string][]string
	isDir    map[string]bool
	watch    *watcher
	active   string  // the row of the file shown in the editor, drawn selected
	selected string  // the row the pointer last selected, drawn selected too
	preview  bool    // the row being activated was activated with Space
	peek     *string // when set, activating a row stores its id here and does nothing else
}

// New returns an explorer rooted at root.
func New(root string) *Explorer {
	e := &Explorer{Root: root, children: map[string][]string{}, isDir: map[string]bool{"": true}}
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
	if c := fyne.CurrentApp().Driver().CanvasForObject(e.tree); c != nil {
		c.Focus(e.tree)
	}
	e.selected = id
	if dir {
		e.tree.ToggleBranch(id)
	}
	e.tree.Refresh()
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
		if d.IsDir() && p != root && hidden[d.Name()] {
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
	e.children = map[string][]string{}
	e.isDir = map[string]bool{"": true}
	e.tree.Refresh()
}

func (e *Explorer) childUIDs(id widget.TreeNodeID) []widget.TreeNodeID {
	if e.Root == "" {
		return nil
	}
	if kids, ok := e.children[id]; ok {
		return kids
	}
	entries, err := os.ReadDir(filepath.Join(e.Root, filepath.FromSlash(id)))
	if err != nil {
		if e.OnErr != nil {
			e.OnErr(err)
		}
		return nil
	}
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].IsDir() != entries[j].IsDir() {
			return entries[i].IsDir()
		}
		return strings.ToLower(entries[i].Name()) < strings.ToLower(entries[j].Name())
	})
	var kids []string
	for _, en := range entries {
		if en.IsDir() && hidden[en.Name()] {
			continue
		}
		kid := en.Name()
		if id != "" {
			kid = id + "/" + kid
		}
		e.isDir[kid] = en.IsDir()
		kids = append(kids, kid)
	}
	e.children[id] = kids
	e.follow(id)
	return kids
}

func (e *Explorer) branch(id widget.TreeNodeID) bool { return e.isDir[id] }

func (e *Explorer) create(bool) fyne.CanvasObject {
	icon := kit.NewIcon(icons.File, kit.Secondary)
	name := kit.NewText("", kit.Body, kit.Primary)
	wash := canvas.NewRectangle(color.Transparent)
	wash.CornerRadius = theme.RadiusControl
	r := &row{e: e, box: container.NewStack(wash, container.NewHBox(icon, name))}
	r.ExtendBaseWidget(r)
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
	icon.Name = fileIcon(id, branch)
	icon.Refresh()
	name.SetText(filepath.Base(filepath.FromSlash(id)))
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
