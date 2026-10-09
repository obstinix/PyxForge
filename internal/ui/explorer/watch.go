package explorer

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2/widget"
	"github.com/fsnotify/fsnotify"
)

// watcher follows the folders the tree has listed and refreshes them when they change on disk
// (a build writing build/, Git switching branches), a moment after the changes settle.
type watcher struct {
	fs       *fsnotify.Watcher
	dispatch func(func())

	mu    sync.Mutex
	dirty map[string]bool // folder IDs to list again
	timer *time.Timer
}

const settle = 150 * time.Millisecond

// Watch starts following changes on disk. dispatch runs the refresh on the UI thread. Without
// Watch the tree changes only on Reload.
func (e *Explorer) Watch(dispatch func(func())) error {
	fw, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	w := &watcher{fs: fw, dispatch: dispatch, dirty: map[string]bool{}}
	e.watch = w
	for id := range e.children {
		e.follow(id)
	}
	go func() {
		for {
			select {
			case ev, ok := <-fw.Events:
				if !ok {
					return
				}
				e.changed(w, ev.Name)
			case err, ok := <-fw.Errors:
				if !ok {
					return
				}
				if e.OnErr != nil && !errors.Is(err, fsnotify.ErrEventOverflow) {
					dispatch(func() { e.OnErr(fmt.Errorf("watching files: %w", err)) })
				}
			}
		}
	}()
	return nil
}

// Close stops watching.
func (e *Explorer) Close() {
	if e.watch != nil {
		_ = e.watch.fs.Close()
		e.watch = nil
	}
}

// follow watches a listed folder.
func (e *Explorer) follow(id string) {
	if e.watch != nil {
		_ = e.watch.fs.Add(e.abs(id))
	}
}

func (e *Explorer) abs(id string) string { return filepath.Join(e.Root, filepath.FromSlash(id)) }

// changed marks the folder holding path for listing again. It runs on the watcher's goroutine.
func (e *Explorer) changed(w *watcher, path string) {
	rel, err := filepath.Rel(e.Root, filepath.Dir(path))
	if err != nil || strings.HasPrefix(rel, "..") {
		return
	}
	id := filepath.ToSlash(rel)
	if id == "." {
		id = ""
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	w.dirty[id] = true
	if w.timer == nil {
		w.timer = time.AfterFunc(settle, func() { w.dispatch(func() { e.refreshDirty(w) }) })
	}
}

// refreshDirty lists changed folders again, on the UI thread.
func (e *Explorer) refreshDirty(w *watcher) {
	if e.watch != w {
		return // closed since
	}
	w.mu.Lock()
	ids := w.dirty
	w.dirty, w.timer = map[string]bool{}, nil
	w.mu.Unlock()
	for id := range ids {
		e.forget(id)
	}
	e.tree.Refresh()
}

// forget drops a folder's cached listing; the tree lists it again when it draws.
func (e *Explorer) forget(id string) {
	delete(e.children, id)
}

// Expanded lists the open folders, for saving the layout.
func (e *Explorer) Expanded() []string {
	var out []string
	for id, dir := range e.isDir {
		if dir && id != "" && e.tree.IsBranchOpen(id) {
			out = append(out, id)
		}
	}
	slices.Sort(out)
	return out
}

// Expand opens folders, parents first, skipping any that no longer exist.
func (e *Explorer) Expand(ids []string) {
	ids = slices.Clone(ids)
	slices.SortFunc(ids, func(a, b string) int { return strings.Count(a, "/") - strings.Count(b, "/") })
	for _, id := range ids {
		if fi, err := os.Stat(e.abs(id)); err != nil || !fi.IsDir() {
			continue
		}
		e.childUIDs(parentID(id)) // learn that id is a folder
		e.tree.OpenBranch(id)
	}
}

func parentID(id string) string {
	if i := strings.LastIndex(id, "/"); i >= 0 {
		return id[:i]
	}
	return ""
}

// Reveal opens the folders above path, scrolls to it and highlights it, without opening it.
func (e *Explorer) Reveal(path string) bool {
	rel, err := filepath.Rel(e.Root, path)
	if err != nil || strings.HasPrefix(rel, "..") {
		return false
	}
	id := filepath.ToSlash(rel)
	var parts []string
	for p := parentID(id); p != ""; p = parentID(p) {
		parts = append(parts, p)
	}
	slices.Reverse(parts)
	e.childUIDs("")
	for _, p := range parts {
		e.childUIDs(p)
		e.tree.OpenBranch(p)
	}
	if !slices.Contains(e.childUIDs(parentID(id)), widget.TreeNodeID(id)) {
		return false
	}
	e.revealing = true
	e.tree.Select(id)
	e.revealing = false
	e.tree.ScrollTo(id)
	return true
}

// Create makes a file, or a folder with dir, at rel (slash-separated, relative to Root),
// creating missing parent folders. It refuses names that leave Root or already exist, and
// returns the absolute path.
func (e *Explorer) Create(rel string, dir bool) (string, error) {
	rel = strings.Trim(filepath.ToSlash(strings.TrimSpace(rel)), "/")
	if rel == "" {
		return "", errors.New("enter a name")
	}
	p := filepath.Join(e.Root, filepath.FromSlash(rel))
	if r, err := filepath.Rel(e.Root, p); err != nil || strings.HasPrefix(r, "..") || filepath.IsAbs(rel) {
		return "", fmt.Errorf("%s is outside the workspace", rel)
	}
	if _, err := os.Lstat(p); err == nil {
		return "", fmt.Errorf("%s already exists", rel)
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return "", err
	}
	var err error
	if dir {
		err = os.Mkdir(p, 0o755)
	} else {
		var f *os.File
		if f, err = os.OpenFile(p, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644); err == nil {
			err = f.Close()
		}
	}
	if err != nil {
		return "", err
	}
	// List the changed folders again now rather than waiting for the watcher.
	for id := parentID(rel); ; id = parentID(id) {
		e.forget(id)
		if id == "" {
			break
		}
	}
	e.tree.Refresh()
	return p, nil
}
