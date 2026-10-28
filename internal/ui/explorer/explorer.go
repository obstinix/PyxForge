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

	tree     *tree
	children map[string][]string
	isDir    map[string]bool
	watch    *watcher
	active   string // the row of the file shown in the editor, drawn selected
	preview  bool   // the row being activated was activated with Space
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
	return e
}

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
	default:
		t.Tree.TypedKey(ev)
	}
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
func (e *Explorer) Files(limit int) (files []string, truncated bool) {
	if e.Root == "" {
		return nil, false
	}
	_ = filepath.WalkDir(e.Root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // unreadable entries are skipped, not fatal
		}
		if d.IsDir() && p != e.Root && hidden[d.Name()] {
			return filepath.SkipDir
		}
		if !d.IsDir() {
			if len(files) == limit {
				truncated = true
				return filepath.SkipAll
			}
			rel, _ := filepath.Rel(e.Root, p)
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
	return container.NewStack(wash, container.NewHBox(icon, name))
}

func (e *Explorer) update(id widget.TreeNodeID, branch bool, o fyne.CanvasObject) {
	row := o.(*fyne.Container)
	wash := row.Objects[0].(*canvas.Rectangle)
	line := row.Objects[1].(*fyne.Container)
	icon := line.Objects[0].(*kit.Icon)
	name := line.Objects[1].(*kit.Text)
	icon.Name = fileIcon(id, branch)
	icon.Refresh()
	name.SetText(filepath.Base(filepath.FromSlash(id)))
	var fill color.Color = color.Transparent
	if id != "" && id == e.active {
		fill = theme.Current().State.Selected
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
