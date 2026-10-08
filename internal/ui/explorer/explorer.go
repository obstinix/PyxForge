// Package explorer shows the workspace as a lazily loaded, read-only file tree.
package explorer

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"github.com/obstinix/PyxForge/internal/ui/icons"
	"github.com/obstinix/PyxForge/internal/ui/kit"
)

// Hidden directories are never listed: version-control internals and build caches.
var hidden = map[string]bool{".git": true, "node_modules": true, "target": true, ".impeccable": true}

// Explorer is the file tree. Node IDs are slash-separated paths relative to Root; "" is Root.
type Explorer struct {
	Root    string
	OnOpen  func(path string) // called with an absolute path when a file is activated
	OnErr   func(err error)   // called when a directory cannot be read
	OnFocus func(bool)        // called when the tree gains or loses keyboard focus

	tree     *tree
	children map[string][]string
	isDir    map[string]bool
}

// New returns an explorer rooted at root.
func New(root string) *Explorer {
	e := &Explorer{Root: root, children: map[string][]string{}, isDir: map[string]bool{"": true}}
	e.tree = &tree{onFocus: func(on bool) {
		if e.OnFocus != nil {
			e.OnFocus(on)
		}
	}}
	e.tree.ChildUIDs, e.tree.IsBranch = e.childUIDs, e.branch
	e.tree.CreateNode, e.tree.UpdateNode = e.create, e.update
	e.tree.ExtendBaseWidget(e.tree)
	e.tree.HideSeparators = true
	e.tree.OnSelected = func(id widget.TreeNodeID) {
		if e.isDir[id] {
			e.tree.ToggleBranch(id)
			e.tree.UnselectAll()
			return
		}
		if e.OnOpen != nil {
			e.OnOpen(filepath.Join(e.Root, filepath.FromSlash(id)))
		}
	}
	return e
}

// Widget is the tree to place in a panel.
func (e *Explorer) Widget() fyne.CanvasObject { return e.tree }

// Focus gives the tree keyboard focus on canvas c.
func (e *Explorer) Focus(c fyne.Canvas) { c.Focus(e.tree) }

// tree is Fyne's tree with a focus callback, so the shell can show where keyboard focus is:
// Fyne marks the keyboard-highlighted row with the hover wash only.
type tree struct {
	widget.Tree
	onFocus func(bool)
}

func (t *tree) FocusGained() {
	t.Tree.FocusGained()
	t.onFocus(true)
}

func (t *tree) FocusLost() {
	t.Tree.FocusLost()
	t.onFocus(false)
}

// Files lists every regular file under Root (relative, slash-separated) for quick open,
// skipping hidden directories. It stops after limit entries.
func (e *Explorer) Files(limit int) []string {
	var out []string
	_ = filepath.WalkDir(e.Root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // unreadable entries are skipped, not fatal
		}
		if d.IsDir() && p != e.Root && (hidden[d.Name()] || strings.HasPrefix(d.Name(), ".")) {
			return filepath.SkipDir
		}
		if !d.IsDir() {
			rel, _ := filepath.Rel(e.Root, p)
			out = append(out, filepath.ToSlash(rel))
			if len(out) >= limit {
				return filepath.SkipAll
			}
		}
		return nil
	})
	return out
}

// Reload drops cached listings and redraws the tree.
func (e *Explorer) Reload() {
	e.children = map[string][]string{}
	e.isDir = map[string]bool{"": true}
	e.tree.Refresh()
}

func (e *Explorer) childUIDs(id widget.TreeNodeID) []widget.TreeNodeID {
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
	return kids
}

func (e *Explorer) branch(id widget.TreeNodeID) bool { return e.isDir[id] }

func (e *Explorer) create(bool) fyne.CanvasObject {
	icon := kit.NewIcon(icons.File, kit.Secondary)
	name := kit.NewText("", kit.Body, kit.Primary)
	return container.NewHBox(icon, name)
}

func (e *Explorer) update(id widget.TreeNodeID, branch bool, o fyne.CanvasObject) {
	row := o.(*fyne.Container)
	icon := row.Objects[0].(*kit.Icon)
	name := row.Objects[1].(*kit.Text)
	icon.Name = fileIcon(id, branch)
	icon.Refresh()
	name.SetText(filepath.Base(filepath.FromSlash(id)))
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
