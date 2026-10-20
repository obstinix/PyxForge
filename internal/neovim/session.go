// Package neovim runs Neovim as PyxForge's editor engine: an embedded child process (`nvim
// --embed`) driven over msgpack-RPC, drawn by PyxForge through the linegrid UI protocol. Vim
// behaviour, buffers, syntax and LSP all stay in Neovim; this package only moves keys in and
// screen updates out (decision D4).
package neovim

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/neovim/go-client/nvim"
	"github.com/obstinix/PyxForge/internal/proc"
	"github.com/obstinix/PyxForge/internal/toolchain"
)

// Options configure a session.
type Options struct {
	Path   string   // the nvim executable; empty means search PATH
	Dir    string   // working folder
	Args   []string // extra command-line arguments, after PyxForge's own
	Env    []string // extra environment, added to the current one
	Width  int      // initial grid size in cells
	Height int

	// Config is PyxForge's init.lua (nvim.Install). With it, Neovim runs under
	// NVIM_APPNAME=AppName with that configuration; without it, Neovim starts --clean.
	Config  string
	AppName string

	// OnClipboardSet receives text Neovim copies to the system clipboard ("+ and "*
	// registers); OnClipboardGet supplies it on paste. Both may be called from the RPC
	// goroutine. Clipboard integration needs Config and Neovim 0.10+.
	OnClipboardSet func(text string)
	OnClipboardGet func() string

	// OnFlush runs on the RPC goroutine after a redraw batch ends in a flush: the grid is
	// consistent and can be drawn. It must not block.
	OnFlush func()
	// OnEvent runs on the RPC goroutine for buffer events PyxForge subscribes to.
	OnEvent func(Event)
	// OnExit runs once when the Neovim process ends, with the reason if it failed.
	OnExit func(error)
}

// Event is a buffer change Neovim reports through the autocommands Start installs.
type Event struct {
	Kind        string // "BufEnter", "BufModifiedSet", "BufWritePost", "BufDelete", "BufFilePost" (renamed), "DiagnosticChanged", "LspAttach", "LspMissing", "LspMessage", "TermClose" or "FileChanged"
	Buffer      int
	Name        string // the buffer's file name; for the Lsp events, the server's name
	Modified    bool
	BufType     string       // Neovim's 'buftype': "" for a file, "terminal", "help", "nofile"…
	Listed      bool         // 'buflisted': the buffer is one the user opened
	Diagnostics []Diagnostic // for DiagnosticChanged
	Status      int          // for TermClose, the job's exit status; for LspMessage, the level (1 error … 4 log)
	Message     string       // for LspMessage; for FileChanged, "conflict" (unsaved changes here) or "deleted"
}

// IsFile reports whether the event is about a listed buffer holding a named file.
func (e Event) IsFile() bool { return e.BufType == "" && e.Listed && e.Name != "" }

// Diagnostic is one vim.diagnostic entry, 0-based like Neovim's.
type Diagnostic struct {
	Line, Col int
	Severity  int // 1 error, 2 warning, 3 info, 4 hint
	Message   string
	Source    string
}

// Session is one running Neovim.
type Session struct {
	v      *nvim.Nvim
	grid   Grid
	opts   Options
	inputs chan string
	done   chan struct{}
	once   sync.Once
	exitMu sync.Mutex
	exited bool

	clipboard bool // Neovim uses PyxForge as its clipboard provider

	inputAt atomic.Int64 // when the oldest input not yet answered by a flush was queued
	sent    atomic.Int64
	latency Latency
}

// Latency measures the time from queuing keys to the flush that shows their effect.
type Latency struct {
	mu    sync.Mutex
	Count int
	Total time.Duration
	Max   time.Duration
}

// Snapshot returns the counters.
func (l *Latency) Snapshot() (count int, avg, worst time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.Count > 0 {
		avg = l.Total / time.Duration(l.Count)
	}
	return l.Count, avg, l.Max
}

// Latency returns the input-to-flush counters.
func (s *Session) Latency() *Latency { return &s.latency }

// ErrNotInstalled is returned when no nvim executable can be found.
var ErrNotInstalled = errors.New("nvim not found on PATH: install Neovim to edit files")

// eventsLua installs the autocommands that report buffer state to PyxForge. It runs once,
// with the RPC channel ID as its argument.
const eventsLua = `
local chan = ...
local group = vim.api.nvim_create_augroup("pyxforge", { clear = true })
local function send(ev, diags)
  local buf = ev.buf
  if not vim.api.nvim_buf_is_valid(buf) then return end
  vim.rpcnotify(chan, "pyxforge_event", ev.event, buf, vim.api.nvim_buf_get_name(buf),
    vim.bo[buf].modified, vim.bo[buf].buftype, vim.bo[buf].buflisted, diags or {})
end
vim.api.nvim_create_autocmd({ "BufEnter", "BufModifiedSet", "BufWritePost", "BufDelete", "BufFilePost" }, {
  group = group, callback = function(ev) send(ev) end,
})
vim.api.nvim_create_autocmd("DiagnosticChanged", {
  group = group,
  callback = function(ev)
    local out = {}
    for _, d in ipairs(vim.diagnostic.get(ev.buf)) do
      out[#out + 1] = { d.lnum, d.col, d.severity, d.message, d.source or "" }
    end
    send(ev, out)
  end,
})
vim.api.nvim_create_autocmd("TermClose", {
  group = group,
  callback = function(ev)
    local status = vim.v.event and vim.v.event.status or -1
    vim.rpcnotify(chan, "pyxforge_term", ev.buf, status)
  end,
})

-- A file changed on disk while open. A buffer without unsaved changes is reloaded ('autoread'
-- does most of these before this runs). A buffer with unsaved changes, or whose file was
-- deleted, is left as it is and PyxForge asks the user what to do: Neovim's own question would
-- appear inside the editor grid, under the wrong tab and out of reach of the mouse.
local function changed(buf, reason)
  vim.rpcnotify(chan, "pyxforge_filechanged", buf, vim.api.nvim_buf_get_name(buf), reason, vim.bo[buf].modified)
end
vim.api.nvim_create_autocmd("FileChangedShell", {
  group = group,
  callback = function(ev)
    local reason = vim.v.fcs_reason
    if reason == "deleted" then
      vim.v.fcs_choice = ""
      changed(ev.buf, "deleted")
    elseif vim.bo[ev.buf].modified or reason == "conflict" then
      vim.v.fcs_choice = ""
      vim.b[ev.buf].pyxforge_conflict = true
      changed(ev.buf, "conflict")
    elseif reason == "mode" or reason == "time" then
      vim.v.fcs_choice = "" -- the text is the same
    else
      vim.v.fcs_choice = "reload"
    end
  end,
})
vim.api.nvim_create_autocmd({ "BufWritePost", "BufReadPost" }, {
  group = group,
  callback = function(ev) vim.b[ev.buf].pyxforge_conflict = nil end,
})

-- Every save PyxForge makes, and Ctrl+S, goes through pyxforge.save: a buffer whose file
-- changed on disk under unsaved changes is not written (a plain :write would stop to ask
-- inside the grid, and over RPC would wait for an answer); PyxForge is asked again instead.
_G.pyxforge = _G.pyxforge or {}
function pyxforge.save(buf)
  buf = (buf == nil or buf == 0) and vim.api.nvim_get_current_buf() or buf
  if vim.b[buf].pyxforge_conflict then
    changed(buf, "conflict")
    return false
  end
  vim.api.nvim_buf_call(buf, function() vim.cmd.write() end)
  return true
end
-- save_all writes every listed, modified file buffer and returns the names of those it left
-- because their file changed on disk.
function pyxforge.save_all()
  local left = {}
  for _, b in ipairs(vim.api.nvim_list_bufs()) do
    if vim.bo[b].buflisted and vim.bo[b].modified and vim.bo[b].buftype == "" then
      if not pyxforge.save(b) then left[#left + 1] = vim.api.nvim_buf_get_name(b) end
    end
  end
  return left
end
`

// Start launches Neovim, attaches PyxForge as its UI and installs the event autocommands.
// Neovim starts without the user's configuration (--clean): PyxForge's own configuration is
// loaded by later phases with an explicit -u, never from the user's config folder.
func Start(ctx context.Context, o Options) (*Session, error) {
	path := o.Path
	if path == "" {
		p, err := toolchain.LookPath("nvim") // a path set in Doctor, the bundled copy, then PATH
		if err != nil {
			return nil, ErrNotInstalled
		}
		path = p
	}
	if o.Width <= 0 || o.Height <= 0 {
		o.Width, o.Height = 80, 24
	}
	args := []string{"--embed", "--clean", "-n"}
	env := append(os.Environ(), o.Env...)
	env = withLogFile(env)
	if o.Config != "" {
		args = []string{"--embed", "-u", o.Config, "-n"}
		app := o.AppName
		if app == "" {
			app = "pyxforge"
		}
		env = append(env, "NVIM_APPNAME="+app)
	}
	args = append(args, o.Args...)
	v, err := nvim.NewChildProcess(
		nvim.ChildProcessCommand(path),
		nvim.ChildProcessArgs(args...),
		nvim.ChildProcessDir(o.Dir),
		nvim.ChildProcessEnv(env),
		nvim.ChildProcessContext(ctx),
		nvim.ChildProcessServe(false),
		nvim.ChildProcessLogf(func(string, ...any) {}),
	)
	if err != nil {
		return nil, fmt.Errorf("start Neovim: %w", err)
	}
	s := &Session{v: v, opts: o, inputs: make(chan string, 256), done: make(chan struct{})}
	if err := v.RegisterHandler("redraw", func(updates ...[]any) {
		batch := make([]any, len(updates))
		for i, u := range updates {
			batch[i] = u
		}
		if !s.grid.Apply(batch) {
			return
		}
		if t := s.inputAt.Swap(0); t != 0 {
			d := time.Since(time.Unix(0, t))
			s.latency.mu.Lock()
			s.latency.Count++
			s.latency.Total += d
			s.latency.Max = max(s.latency.Max, d)
			s.latency.mu.Unlock()
		}
		if o.OnFlush != nil {
			o.OnFlush()
		}
	}); err != nil {
		v.Close()
		return nil, err
	}
	if err := v.RegisterHandler("pyxforge_event", func(kind string, buf int, name string, modified bool, buftype string, listed bool, diags []any) {
		if o.OnEvent != nil {
			o.OnEvent(Event{Kind: kind, Buffer: buf, Name: name, Modified: modified, BufType: buftype, Listed: listed,
				Diagnostics: parseDiagnostics(diags)})
		}
	}); err != nil {
		v.Close()
		return nil, err
	}
	if err := v.RegisterHandler("pyxforge_lsp", func(kind string, buf int, server string, level int, text string) {
		if o.OnEvent == nil {
			return
		}
		k := map[string]string{"attach": "LspAttach", "missing": "LspMissing", "message": "LspMessage"}[kind]
		if k != "" {
			o.OnEvent(Event{Kind: k, Buffer: buf, Name: server, Status: level, Message: text})
		}
	}); err != nil {
		v.Close()
		return nil, err
	}
	if err := v.RegisterHandler("pyxforge_term", func(buf, status int) {
		if o.OnEvent != nil {
			o.OnEvent(Event{Kind: "TermClose", Buffer: buf, Status: status})
		}
	}); err != nil {
		v.Close()
		return nil, err
	}
	if err := v.RegisterHandler("pyxforge_filechanged", func(buf int, name, reason string, modified bool) {
		if o.OnEvent != nil {
			o.OnEvent(Event{Kind: "FileChanged", Buffer: buf, Name: name, Message: reason, Modified: modified, Listed: true})
		}
	}); err != nil {
		v.Close()
		return nil, err
	}
	if err := v.RegisterHandler("pyxforge_clipboard_set", func(lines []string, regtype string) {
		if o.OnClipboardSet != nil {
			// A linewise register is whole lines: end it with exactly one newline (Neovim may
			// or may not pass the trailing empty line).
			text := strings.Join(lines, "\n")
			if regtype == "V" && !strings.HasSuffix(text, "\n") {
				text += "\n"
			}
			o.OnClipboardSet(text)
		}
	}); err != nil {
		v.Close()
		return nil, err
	}
	if err := v.RegisterHandler("pyxforge_clipboard_get", func() ([]any, error) {
		text := ""
		if o.OnClipboardGet != nil {
			text = o.OnClipboardGet()
		}
		regtype := "v"
		if strings.HasSuffix(text, "\n") {
			regtype, text = "V", strings.TrimSuffix(text, "\n")
		}
		return []any{strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n"), regtype}, nil
	}); err != nil {
		v.Close()
		return nil, err
	}
	go func() {
		err := v.Serve()
		s.exitMu.Lock()
		s.exited = true
		s.exitMu.Unlock()
		s.stop()
		if o.OnExit != nil {
			o.OnExit(err)
		}
	}()
	go s.sendInputs()

	if err := v.AttachUI(o.Width, o.Height, map[string]any{"rgb": true, "ext_linegrid": true}); err != nil {
		s.Close()
		return nil, fmt.Errorf("attach to Neovim: %w", err)
	}
	chanID := v.ChannelID()
	if err := v.ExecLua(eventsLua, nil, chanID); err != nil {
		s.Close()
		return nil, fmt.Errorf("install PyxForge autocommands: %w", err)
	}
	if o.Config != "" {
		if err := v.ExecLua(`return require("pyxforge.clipboard").setup(...)`, &s.clipboard, chanID); err != nil {
			s.Close()
			return nil, fmt.Errorf("set up the clipboard: %w", err)
		}
	}
	return s, nil
}

// withLogFile points Neovim's log at PyxForge's cache folder unless the environment already
// names one. Otherwise Neovim may fall back to writing .nvimlog in its working folder, which
// is the user's project.
func withLogFile(env []string) []string {
	for _, kv := range env {
		if strings.HasPrefix(kv, "NVIM_LOG_FILE=") {
			return env
		}
	}
	dir, err := os.UserCacheDir()
	if err != nil {
		dir = os.TempDir()
	}
	dir = filepath.Join(dir, "PyxForge")
	if os.MkdirAll(dir, 0o755) != nil {
		return env
	}
	return append(env, "NVIM_LOG_FILE="+filepath.Join(dir, "nvim.log"))
}

func parseDiagnostics(list []any) []Diagnostic {
	out := make([]Diagnostic, 0, len(list))
	for _, item := range list {
		d, _ := item.([]any)
		if len(d) < 5 {
			continue
		}
		msg, _ := d[3].(string)
		src, _ := d[4].(string)
		out = append(out, Diagnostic{Line: toInt(d[0]), Col: toInt(d[1]), Severity: toInt(d[2]), Message: msg, Source: src})
	}
	return out
}

// sendInputs forwards keys in order, off the UI thread.
func (s *Session) sendInputs() {
	for {
		select {
		case keys := <-s.inputs:
			_, _ = s.v.Input(keys)
		case <-s.done:
			return
		}
	}
}

// Input queues keys in Neovim's key notation (`:help key-notation`). It never blocks the
// caller; keys are dropped only if 256 inputs are already waiting.
func (s *Session) Input(keys string) {
	// Stamp before queuing, so the flush that answers these keys cannot run first.
	s.inputAt.CompareAndSwap(0, time.Now().UnixNano())
	select {
	case s.inputs <- keys:
		s.sent.Add(1)
	default:
	}
}

// Inputs counts the key inputs queued so far.
func (s *Session) Inputs() int64 { return s.sent.Load() }

// Grid returns the current screen.
func (s *Session) Grid() Snapshot { return s.grid.Snapshot() }

// Resize asks Neovim to redraw at a new size in cells.
func (s *Session) Resize(cols, rows int) error {
	if cols < 1 || rows < 1 {
		return nil
	}
	return s.v.TryResizeUI(cols, rows)
}

// Mouse sends a mouse event: button "left", "right", "middle" or "wheel"; action "press",
// "drag", "release", or "up"/"down" for the wheel; modifier like "C" or "".
func (s *Session) Mouse(button, action, modifier string, row, col int) error {
	return s.v.InputMouse(button, action, modifier, 0, row, col)
}

// Open edits a file in the current window.
func (s *Session) Open(path string) error {
	return s.v.ExecLua(`vim.cmd.edit(vim.fn.fnameescape(...))`, nil, path)
}

// Save writes the current buffer. It reports false, and writes nothing, when the file changed
// on disk while the buffer had unsaved changes; a FileChanged event follows.
func (s *Session) Save() (bool, error) {
	var saved bool
	err := s.v.ExecLua(`return pyxforge.save(0)`, &saved)
	return saved, err
}

// SaveAll writes every modified file buffer and returns the names of those it did not write
// because their file changed on disk; a FileChanged event follows for each.
func (s *Session) SaveAll() ([]string, error) {
	var left []string
	err := s.v.ExecLua(`return pyxforge.save_all()`, &left)
	return left, err
}

// SaveAs writes a buffer to path and makes path its file, as :saveas does. With overwrite an
// existing file at path is replaced; without it, that is an error (E13), and nothing is
// written. A BufFilePost event reports the new name.
func (s *Session) SaveAs(buf int, path string, overwrite bool) error {
	return s.v.ExecLua(`local buf, path, overwrite = ...
vim.api.nvim_buf_call(buf, function()
  vim.cmd((overwrite and "saveas! " or "saveas ") .. vim.fn.fnameescape(path))
end)`, nil, buf, path, overwrite)
}

// ErrChangedOnDisk means a buffer was not written because its file changed on disk while it
// had unsaved changes: Reload or Overwrite settles it.
var ErrChangedOnDisk = errors.New("the file changed on disk while it had unsaved changes here")

// Reload replaces a buffer's text with its file's, dropping unsaved changes.
func (s *Session) Reload(buf int) error {
	return s.v.ExecLua(`local buf = ...
vim.api.nvim_buf_call(buf, function() vim.cmd("edit!") end)`, nil, buf)
}

// Overwrite writes a buffer over its file even though the file changed on disk.
func (s *Session) Overwrite(buf int) error {
	return s.v.ExecLua(`local buf = ...
vim.api.nvim_buf_call(buf, function() vim.cmd("write!") end)`, nil, buf)
}

// Conflicted reports whether a buffer's file changed on disk while it had unsaved changes and
// neither Reload nor Overwrite (nor a later read or write) has settled it.
func (s *Session) Conflicted(buf int) (bool, error) {
	var on bool
	err := s.v.ExecLua(`local buf = ...
return vim.api.nvim_buf_is_valid(buf) and vim.b[buf].pyxforge_conflict == true`, &on, buf)
	return on, err
}

// CheckFiles makes Neovim look for files changed on disk now, as it does on entering a buffer.
func (s *Session) CheckFiles() error { return s.v.Command("silent! checktime") }

// Modified reports whether the current buffer has unsaved changes.
func (s *Session) Modified() (bool, error) {
	var m bool
	err := s.v.ExecLua(`return vim.bo.modified`, &m)
	return m, err
}

// SwitchTo makes a buffer current in the current window. It travels through the input queue
// as a <Cmd> key, so keys typed after it reach the new buffer: a separate RPC call could be
// overtaken by them.
func (s *Session) SwitchTo(buf int) {
	s.Input(fmt.Sprintf("<Cmd>buffer %d<CR>", buf))
}

// CloseBuffer deletes a buffer: with save its changes are written first, with discard they
// are dropped; with neither, a modified buffer is refused.
func (s *Session) CloseBuffer(buf int, save, discard bool) error {
	var closed bool
	err := s.v.ExecLua(`local buf, save, discard = ...
if save and not pyxforge.save(buf) then return false end
vim.api.nvim_buf_delete(buf, { force = discard })
return true`, &closed, buf, save, discard)
	if err == nil && !closed {
		return ErrChangedOnDisk
	}
	return err
}

// ModifiedFiles lists the names of listed file buffers with unsaved changes.
func (s *Session) ModifiedFiles() ([]string, error) {
	var names []string
	err := s.v.ExecLua(`local out = {}
for _, b in ipairs(vim.api.nvim_list_bufs()) do
  if vim.bo[b].buflisted and vim.bo[b].modified and vim.bo[b].buftype == "" then
    out[#out + 1] = vim.api.nvim_buf_get_name(b)
  end
end
return out`, &names)
	return names, err
}

// GoTo opens a file and puts the cursor on a 0-based line and column.
func (s *Session) GoTo(path string, line, col int) error {
	return s.v.ExecLua(`local path, line, col = ...
vim.cmd.edit(vim.fn.fnameescape(path))
pcall(vim.api.nvim_win_set_cursor, 0, { line + 1, col })`, nil, path, line, col)
}

// SetColorscheme replaces Neovim's highlights with groups (name → nvim_set_hl attributes)
// and sets 'background' ("dark" or "light").
func (s *Session) SetColorscheme(name, background string, groups map[string]map[string]any) error {
	return s.v.ExecLua(`local name, background, groups = ...
vim.cmd.highlight("clear")
vim.o.background = background
vim.g.colors_name = name
for group, attrs in pairs(groups) do
  vim.api.nvim_set_hl(0, group, attrs)
end`, nil, name, background, groups)
}

// SetTerminalColors sets the 16 ANSI colours ("#rrggbb") that terminals opened afterwards use.
func (s *Session) SetTerminalColors(colors [16]string) error {
	return s.v.ExecLua(`for i, c in ipairs(...) do vim.g["terminal_color_" .. (i - 1)] = c end`, nil, colors[:])
}

// Terminal starts the user's shell ('shell') in a terminal in the current window, in
// terminal mode, and returns its buffer. A terminal buffer the window showed before is
// closed, ending its job if it still runs.
func (s *Session) Terminal() (int, error) {
	var buf int
	err := s.v.ExecLua(`local old = vim.api.nvim_get_current_buf()
vim.cmd.terminal()
if old ~= vim.api.nvim_get_current_buf() and vim.api.nvim_buf_is_valid(old) then
  pcall(vim.api.nvim_buf_delete, old, { force = true })
end
return vim.api.nvim_get_current_buf()`, &buf)
	if err != nil {
		return 0, err
	}
	// Enter terminal mode in a request of its own: when the old terminal was in terminal mode,
	// Neovim leaves that mode after the call that closed it, cancelling a startinsert made
	// in the same call (seen on 0.9).
	err = s.v.ExecLua(`if vim.api.nvim_get_mode().mode ~= "t" then vim.cmd.startinsert() end`, nil)
	return buf, err
}

// NewTerminal starts another shell in a new terminal buffer shown in the current window, in
// terminal mode, and returns the buffer. Other terminals keep running in hidden buffers.
func (s *Session) NewTerminal() (int, error) {
	var buf int
	if err := s.v.ExecLua(`vim.cmd.terminal()
return vim.api.nvim_get_current_buf()`, &buf); err != nil {
		return 0, err
	}
	err := s.v.ExecLua(`if vim.api.nvim_get_mode().mode ~= "t" then vim.cmd.startinsert() end`, nil)
	return buf, err
}

// KeepTranscript replaces the terminal buffer of a shell that has exited with a read-only
// copy of everything it showed, scrollback included, in the windows that showed it, and
// returns the copy's buffer. In an exited terminal any key in terminal mode closes the buffer
// and its output with it; the copy can be scrolled, searched and copied from safely. The
// copy has the text without its colours.
func (s *Session) KeepTranscript(buf int) (int, error) {
	var out int
	err := s.v.ExecLua(`local buf = ...
if not vim.api.nvim_buf_is_valid(buf) then return 0 end
local lines = vim.api.nvim_buf_get_lines(buf, 0, -1, false)
while #lines > 0 and lines[#lines]:match("^%s*$") do lines[#lines] = nil end
local out = vim.api.nvim_create_buf(false, true)
vim.api.nvim_buf_set_lines(out, 0, -1, false, lines)
vim.bo[out].modifiable = false
vim.bo[out].bufhidden = "hide"
vim.b[out].pyxforge_transcript = true
for _, w in ipairs(vim.fn.win_findbuf(buf)) do
  vim.api.nvim_win_set_buf(w, out)
  pcall(vim.api.nvim_win_set_cursor, w, { math.max(#lines, 1), 0 })
end
pcall(vim.api.nvim_buf_delete, buf, { force = true })
return out`, &out, buf)
	return out, err
}

// ShowTerminal shows a terminal buffer in the current window, in terminal mode while its job
// runs. (In an exited terminal, terminal mode would close the buffer at the next key.)
func (s *Session) ShowTerminal(buf int) error {
	var running bool
	if err := s.v.ExecLua(`local buf = ...
vim.api.nvim_set_current_buf(buf)
local job = vim.b[buf].terminal_job_id
return job ~= nil and vim.fn.jobwait({ job }, 0)[1] == -1`, &running, buf); err != nil {
		return err
	}
	// A request of its own, as in Terminal: leaving the previous terminal's mode would cancel it.
	if running {
		return s.v.ExecLua(`if vim.api.nvim_get_mode().mode ~= "t" then vim.cmd.startinsert() end`, nil)
	}
	return s.v.Command("stopinsert")
}

// Diff opens path beside a read-only copy of other (its committed text, say), named name,
// and turns on Neovim's diff mode for the pair. A previous Diff's copy is closed first.
func (s *Session) Diff(path, name, other string) error {
	return s.v.ExecLua(`local path, name, text = ...
for _, w in ipairs(vim.api.nvim_list_wins()) do
  if vim.b[vim.api.nvim_win_get_buf(w)].pyxforge_diff then pcall(vim.api.nvim_win_close, w, true) end
end
vim.cmd("silent! diffoff!")
vim.cmd.edit(vim.fn.fnameescape(path))
local ft = vim.bo.filetype
vim.cmd("leftabove vnew")
local buf = vim.api.nvim_get_current_buf()
vim.bo[buf].buftype = "nofile"
vim.bo[buf].bufhidden = "wipe"
vim.bo[buf].swapfile = false
vim.b[buf].pyxforge_diff = true
local lines = vim.split(text, "\n", { plain = true })
if lines[#lines] == "" then table.remove(lines) end
vim.api.nvim_buf_set_lines(buf, 0, -1, false, lines)
vim.bo[buf].modifiable = false
vim.bo[buf].filetype = ft
pcall(vim.api.nvim_buf_set_name, buf, name)
vim.cmd("diffthis")
vim.cmd("wincmd p")
vim.cmd("diffthis")`, nil, path, name, other)
}

// BuildDiagnostic is a build message for the editor, 0-based like Neovim's.
type BuildDiagnostic struct {
	File      string
	Line, Col int
	Severity  int // 1 error, 2 warning, 3 info, 4 hint
	Message   string
	Source    string
}

// SetBuildDiagnostics replaces the build diagnostics the editor shows, in open buffers and in
// files opened later (PyxForge's configuration, nvim/lua/pyxforge/build.lua). Without that
// configuration it does nothing.
func (s *Session) SetBuildDiagnostics(items []BuildDiagnostic) error {
	if s.opts.Config == "" {
		return nil
	}
	list := make([]map[string]any, 0, len(items))
	for _, d := range items {
		list = append(list, map[string]any{"file": d.File, "lnum": d.Line, "col": d.Col, "severity": d.Severity,
			"message": d.Message, "source": d.Source})
	}
	return s.v.ExecLua(`require("pyxforge.build").set(...)`, nil, list)
}

// Command runs an Ex command.
func (s *Session) Command(cmd string) error { return s.v.Command(cmd) }

// ExecLua runs Lua in Neovim; result may be nil.
func (s *Session) ExecLua(code string, result any, args ...any) error {
	return s.v.ExecLua(code, result, args...)
}

// Clipboard reports whether Neovim's "+ and "* registers use the system clipboard through
// PyxForge (Neovim 0.10 or newer with PyxForge's configuration).
func (s *Session) Clipboard() bool { return s.clipboard }

// Exited reports whether the Neovim process has ended.
func (s *Session) Exited() bool {
	s.exitMu.Lock()
	defer s.exitMu.Unlock()
	return s.exited
}

func (s *Session) stop() { s.once.Do(func() { close(s.done) }) }

// Close ends Neovim and waits for the process to exit: closing its RPC channel makes an
// embedded Neovim quit, and the client kills it if it has not exited after ten seconds.
//
// It also ends what Neovim started, which Neovim leaves behind as it exits: the shells of its
// terminals (stopped first, as a closing console can give a shell up to five seconds), what
// those shells started (a prompt's git, a build), console hosts, language servers. Anything
// of that tree still running a moment after Neovim is terminated, so nothing keeps running in,
// or holding open, the session's folder.
func (s *Session) Close() error {
	nvimPID, children := s.children()
	s.stop()
	err := s.v.Close()
	waitGone(children, childGrace)
	if nvimPID > 0 {
		// Processes keep their parent's ID after it exits, so the tree, including what started
		// after the first look, can still be found from Neovim's.
		left := proc.Descendants(nvimPID)
		for _, pid := range children {
			if proc.Alive(pid) && !slices.Contains(left, pid) {
				left = append(left, pid)
			}
		}
		for _, pid := range left {
			_ = proc.Terminate(pid)
		}
		waitGone(left, childGrace)
	}
	return err
}

// childGrace is how long Close gives Neovim's child processes to end, before and after
// terminating them.
const childGrace = time.Second

func waitGone(pids []int, d time.Duration) {
	deadline := time.Now().Add(d)
	for _, pid := range pids {
		for proc.Alive(pid) && time.Now().Before(deadline) {
			time.Sleep(10 * time.Millisecond)
		}
	}
}

// children stops the jobs of Neovim's terminals and returns Neovim's process ID and the
// processes it started, or nothing when Neovim does not answer within a second (it may be
// busy, or gone).
func (s *Session) children() (int, []int) {
	if s.Exited() {
		return 0, nil
	}
	type answer struct {
		PID  int
		Jobs []int // the terminal shells, which Descendants may not list (macOS)
	}
	got := make(chan answer, 1)
	go func() {
		var a answer
		_ = s.v.ExecLua(`local jobs = {}
for _, b in ipairs(vim.api.nvim_list_bufs()) do
  local job = vim.b[b].terminal_job_id
  if job then
    local ok, pid = pcall(vim.fn.jobpid, job)
    if ok and pid > 0 then jobs[#jobs + 1] = pid end
    pcall(vim.fn.jobstop, job)
  end
end
return { PID = vim.fn.getpid(), Jobs = jobs }`, &a)
		got <- a
	}()
	select {
	case a := <-got:
		if a.PID > 0 {
			pids := proc.Descendants(a.PID)
			for _, j := range a.Jobs {
				if !slices.Contains(pids, j) {
					pids = append(pids, j)
				}
			}
			return a.PID, pids
		}
	case <-time.After(time.Second):
	}
	return 0, nil
}

// WaitFlush waits until the grid has flushed past version, or the timeout passes. It exists
// for tests and the feasibility harness.
func (s *Session) WaitFlush(after uint64, timeout time.Duration) (Snapshot, bool) {
	deadline := time.Now().Add(timeout)
	for {
		g := s.Grid()
		if g.Version > after {
			return g, true
		}
		if time.Now().After(deadline) {
			return g, false
		}
		time.Sleep(2 * time.Millisecond)
	}
}
