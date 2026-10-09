package shell

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"github.com/obstinix/PyxForge/internal/neovim"
	"github.com/obstinix/PyxForge/internal/ui/editor"
	"github.com/obstinix/PyxForge/internal/ui/notifications"
	"github.com/obstinix/PyxForge/internal/ui/theme"
	"github.com/obstinix/PyxForge/nvim"
)

// editorHost connects the editor tabs to one embedded Neovim for the workspace. Neovim owns
// the buffers; each tab shows one listed file buffer, and the single editor view moves into
// the selected tab. Events from Neovim reach the host on the UI thread through s.dispatch.
type editorHost struct {
	s       *Shell
	sess    *neovim.Session
	view    *editor.View
	err     error // why Neovim could not start, once it has failed
	bufs    map[int]*bufTab
	byTab   map[*container.TabItem]*bufTab
	current int
	diags   map[int]bufDiags
	servers map[int][]string // language servers attached, by buffer

	qmu      sync.Mutex
	queue    []func() // Neovim calls that must keep their order (opening files)
	draining bool
}

// do runs f off the UI thread after every call queued before it, so files open in the order
// they were asked for.
func (h *editorHost) do(f func()) {
	h.qmu.Lock()
	h.queue = append(h.queue, f)
	start := !h.draining
	h.draining = true
	h.qmu.Unlock()
	if start {
		go h.drain()
	}
}

func (h *editorHost) drain() {
	for {
		h.qmu.Lock()
		if len(h.queue) == 0 {
			h.draining = false
			h.qmu.Unlock()
			return
		}
		f := h.queue[0]
		h.queue = h.queue[1:]
		h.qmu.Unlock()
		f()
	}
}

type bufTab struct {
	buf      int
	path     string
	modified bool
	item     *container.TabItem
	holder   *fyne.Container
}

type bufDiags struct {
	path  string
	items []neovim.Diagnostic
}

// startEditor starts Neovim the first time a file is opened. It returns false, after telling
// the user why, when Neovim is not available; the shell then shows the file's placeholder.
func (s *Shell) startEditor() bool {
	if !s.editorEnabled {
		return false
	}
	h := s.ed
	if h != nil {
		return h.sess != nil
	}
	h = &editorHost{s: s, view: editor.NewView(), bufs: map[int]*bufTab{}, byTab: map[*container.TabItem]*bufTab{},
		diags: map[int]bufDiags{}, servers: map[int][]string{}}
	s.ed = h
	h.view.IsShellChord = IsShellChord
	h.view.OnShortcut = s.runChord
	h.view.Dispatch = s.dispatch
	o := s.nvimOptions()
	o.Width, o.Height = 100, 30
	o.OnFlush = func() {
		h.view.FlushHook()
		s.dispatch(h.syncStatus)
	}
	o.OnEvent = func(e neovim.Event) { s.dispatch(func() { h.onEvent(e) }) }
	o.OnExit = func(err error) {
		s.dispatch(func() {
			if h.sess != nil {
				s.logf("Neovim exited%s", errSuffix(err))
			}
		})
	}
	sess, err := neovim.Start(context.Background(), o)
	if err != nil {
		h.err = err
		s.Notify(notifications.Error, "Editor unavailable", err.Error())
		s.logf("Neovim could not start: %v", err)
		return false
	}
	h.sess = sess
	h.view.Attach(sess)
	s.syncEditorTheme()
	// An OS light/dark switch changes the System theme without SetSelection.
	s.app.Settings().AddListener(func(fyne.Settings) { s.dispatch(s.syncEditorTheme) })
	s.logf("Neovim attached (%s)", s.root)
	return true
}

// nvimOptions are the settings every embedded Neovim shares: the executable, the workspace
// folder, PyxForge's configuration and the system clipboard.
func (s *Shell) nvimOptions() neovim.Options {
	cfg := ""
	base := s.nvimRuntime
	if base == "" {
		base, _ = nvim.DefaultBase()
	}
	if base != "" {
		init, err := nvim.Install(base)
		if err != nil {
			s.logf("PyxForge's Neovim configuration could not be installed (%v); Neovim starts without it", err)
		}
		cfg = init
	}
	return neovim.Options{
		Path: s.nvimPath, Dir: s.root, Config: cfg, AppName: nvim.AppName, Env: s.nvimEnv,
		OnClipboardSet: func(text string) { s.dispatch(func() { s.app.Clipboard().SetContent(text) }) },
		OnClipboardGet: func() string {
			got := make(chan string, 1)
			s.dispatch(func() { got <- s.app.Clipboard().Content() })
			select {
			case text := <-got:
				return text
			case <-time.After(2 * time.Second): // the UI thread is busy; paste nothing rather than hang
				return ""
			}
		},
	}
}

// syncEditorTheme gives Neovim, in the editor and the terminal, the current theme's colours.
func (s *Shell) syncEditorTheme() {
	s.term.syncTheme(false)
	if s.ed == nil || s.ed.sess == nil {
		return
	}
	bg, groups := editor.Colorscheme(theme.Current())
	sess := s.ed.sess
	go func() {
		if err := sess.SetColorscheme("pyxforge", bg, groups); err != nil {
			s.dispatch(func() { s.logf("Editor colours not applied: %v", err) })
		}
	}()
}

func errSuffix(err error) string {
	if err == nil {
		return ""
	}
	return ": " + err.Error()
}

// open edits a file in Neovim; its tab appears when Neovim reports the buffer.
func (h *editorHost) open(path string) {
	for _, bt := range h.bufs {
		if sameFile(bt.path, path) {
			h.s.editors.Select(bt.item)
			return
		}
	}
	sess := h.sess
	h.do(func() {
		if err := sess.Open(path); err != nil {
			h.s.dispatch(func() { h.s.Notify(notifications.Error, "Cannot open file", err.Error()) })
		}
	})
}

func sameFile(a, b string) bool {
	return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
}

func (h *editorHost) onEvent(e neovim.Event) {
	switch e.Kind {
	case "BufEnter":
		h.current = e.Buffer
		if !e.IsFile() {
			return
		}
		bt := h.bufs[e.Buffer]
		if bt == nil {
			bt = &bufTab{buf: e.Buffer, path: e.Name, holder: container.NewStack()}
			bt.item = container.NewTabItem(filepath.Base(e.Name), bt.holder)
			h.bufs[e.Buffer], h.byTab[bt.item] = bt, bt
			h.s.editors.Append(bt.item)
			h.s.syncEditor()
		}
		bt.modified = e.Modified
		h.title(bt)
		h.s.editors.Select(bt.item)
		h.show(bt)
	case "BufModifiedSet", "BufWritePost":
		if bt := h.bufs[e.Buffer]; bt != nil {
			bt.modified = e.Modified
			h.title(bt)
		}
		h.syncStatus()
		if e.Kind == "BufWritePost" && h.s.gitp.loaded {
			h.s.gitp.refresh()
		}
	case "BufDelete":
		if bt := h.bufs[e.Buffer]; bt != nil {
			delete(h.bufs, e.Buffer)
			delete(h.byTab, bt.item)
			h.s.editors.Remove(bt.item)
			h.s.syncEditor()
		}
		delete(h.diags, e.Buffer)
		h.s.refreshProblems()
	case "LspAttach":
		if !slices.Contains(h.servers[e.Buffer], e.Name) {
			h.servers[e.Buffer] = append(h.servers[e.Buffer], e.Name)
		}
		if bt := h.bufs[e.Buffer]; bt != nil {
			h.s.logf("%s attached to %s", e.Name, filepath.Base(bt.path))
		}
		h.syncStatus()
	case "LspMissing":
		h.s.logf("%s is not installed: no completion or diagnostics for this file type. pyxforge doctor lists language servers.", e.Name)
		h.s.Notify(notifications.Info, e.Name+" not installed", "Editing works; language features need the server.")
	case "LspMessage":
		h.s.logf("%s: %s", e.Name, e.Message)
		if e.Status == 1 {
			h.s.Notify(notifications.Error, e.Name, e.Message)
		}
	case "DiagnosticChanged":
		if len(e.Diagnostics) == 0 {
			delete(h.diags, e.Buffer)
		} else {
			h.diags[e.Buffer] = bufDiags{path: e.Name, items: e.Diagnostics}
		}
		h.s.refreshProblems()
	}
}

// title marks unsaved buffers with a dot, as most editors do.
func (h *editorHost) title(bt *bufTab) {
	t := filepath.Base(bt.path)
	if bt.modified {
		t += " ●"
	}
	if bt.item.Text != t {
		bt.item.Text = t
		h.s.editors.Refresh()
	}
}

// show moves the editor view into a tab.
func (h *editorHost) show(bt *bufTab) {
	for _, other := range h.bufs {
		if other != bt && len(other.holder.Objects) > 0 {
			other.holder.RemoveAll()
		}
	}
	if len(bt.holder.Objects) == 0 {
		bt.holder.Add(h.view)
	}
}

// selected handles the user choosing a tab: Neovim switches to that buffer.
func (h *editorHost) selected(it *container.TabItem) {
	bt := h.byTab[it]
	if bt == nil {
		return
	}
	h.show(bt)
	if bt.buf != h.current {
		h.current = bt.buf
		buf := bt.buf
		go func() { _ = h.sess.SwitchTo(buf) }()
	}
	if c := h.s.win.Canvas(); c != nil {
		c.Focus(h.view)
	}
}

// close closes a buffer's tab, asking first when it has unsaved changes.
func (h *editorHost) close(bt *bufTab) {
	if !bt.modified {
		go func() { _ = h.sess.CloseBuffer(bt.buf, false, false) }()
		return
	}
	h.s.confirmUnsaved(fmt.Sprintf("Save changes to %s?", filepath.Base(bt.path)),
		func() { go func() { _ = h.sess.CloseBuffer(bt.buf, true, false) }() },
		func() { go func() { _ = h.sess.CloseBuffer(bt.buf, false, true) }() })
}

// syncStatus shows Neovim's mode and the current file in the status bar.
func (h *editorHost) syncStatus() {
	mode := modeLabel(h.sess.Grid().Mode)
	text := "Neovim · " + mode
	if bt := h.bufs[h.current]; bt != nil {
		text = mode + " · " + filepath.Base(bt.path)
		if bt.modified {
			text += " ●"
		}
		if srv := h.servers[h.current]; len(srv) > 0 {
			text += " · " + strings.Join(srv, ", ")
		}
	}
	if h.s.statusEditor.text.Text != text {
		h.s.statusEditor.SetText(text)
	}
}

func modeLabel(m string) string {
	switch {
	case m == "", strings.HasPrefix(m, "normal"):
		return "NORMAL"
	case strings.HasPrefix(m, "insert"):
		return "INSERT"
	case strings.HasPrefix(m, "visual"):
		return "VISUAL"
	case strings.HasPrefix(m, "replace"):
		return "REPLACE"
	case strings.HasPrefix(m, "cmdline"):
		return "COMMAND"
	case m == "terminal":
		return "TERMINAL"
	case m == "operator":
		return "PENDING"
	}
	return strings.ToUpper(m)
}

// confirmUnsaved asks Save / Don't Save / Cancel.
func (s *Shell) confirmUnsaved(question string, save, discard func()) {
	var d dialog.Dialog
	saveBtn := widget.NewButton("Save", func() { d.Hide(); save() })
	saveBtn.Importance = widget.HighImportance
	discardBtn := widget.NewButton("Don't Save", func() { d.Hide(); discard() })
	cancelBtn := widget.NewButton("Cancel", func() { d.Hide() })
	content := container.NewVBox(widget.NewLabel("Your changes will be lost if you don't save them."),
		container.NewHBox(discardBtn, cancelBtn, saveBtn))
	d = dialog.NewCustomWithoutButtons(question, content, s.win)
	d.Show()
	s.lastDialog = d
}

// confirmQuit runs before the window closes: unsaved buffers are offered for saving, then
// Neovim is stopped so no process outlives the window.
func (s *Shell) confirmQuit() {
	quit := func() {
		s.Shutdown()
		s.win.Close()
	}
	h := s.ed
	if h == nil || h.sess == nil {
		quit()
		return
	}
	names, err := h.sess.ModifiedFiles()
	if err != nil || len(names) == 0 {
		quit()
		return
	}
	sort.Strings(names)
	for i, n := range names {
		names[i] = filepath.Base(n)
	}
	s.confirmUnsaved(fmt.Sprintf("Save changes to %s?", strings.Join(names, ", ")),
		func() {
			_ = h.sess.Command("wall")
			quit()
		},
		quit)
}

// Shutdown saves the workspace layout and ends every process the shell started. The window's
// close path runs it, and so does the app when it is told to stop by a signal (Ctrl+C in the
// terminal that started it, SIGTERM), which would otherwise leave QEMU running.
func (s *Shell) Shutdown() {
	s.closed.Store(true)
	s.saveState()
	s.StopAll()
}

// StopAll ends every process the shell started (QEMU, GDB, the terminal, Neovim) and stops
// watching files. The window closes through it; review renders call it too.
func (s *Shell) StopAll() {
	s.mach.shutdown()
	s.explorer.Close()
	s.stopEditor()
	s.term.stop()
}

// stopEditor ends Neovim.
func (s *Shell) stopEditor() {
	if s.ed != nil && s.ed.sess != nil {
		sess := s.ed.sess
		s.ed.sess = nil
		_ = sess.Close()
	}
}

// runChord runs the shell command bound to a shortcut, for keys the editor hands back.
func (s *Shell) runChord(sc fyne.Shortcut) bool {
	if run, ok := s.chordRuns[sc.ShortcutName()]; ok {
		run()
		return true
	}
	return false
}
