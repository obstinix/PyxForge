// Package lsp is the language-server client of PyxForge's native editor: JSON-RPC 2.0 over a
// server's standard input and output (Language Server Protocol 3.17), with the requests the
// editor uses (completion, hover, definition, references, rename, formatting, code actions)
// and the diagnostics a server publishes. Servers are separate programs (clangd, gopls,
// pyright, rust-analyzer); PyxForge finds them, it does not bundle them.
package lsp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/obstinix/PyxForge/internal/proc"
)

// Position in a document as the server counts it: a line and a character offset in the
// position encoding agreed at initialisation (UTF-32 when the server offers it, else UTF-16).
type Position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

// Range in a document.
type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

// Location is a range in a file.
type Location struct {
	URI   string `json:"uri"`
	Range Range  `json:"range"`
}

// Diagnostic is a problem a server reports. Severity: 1 error, 2 warning, 3 information, 4 hint.
type Diagnostic struct {
	Range    Range  `json:"range"`
	Severity int    `json:"severity"`
	Message  string `json:"message"`
	Source   string `json:"source"`
}

// CompletionItem is one completion offer.
type CompletionItem struct {
	Label      string    `json:"label"`
	Detail     string    `json:"detail"`
	Kind       int       `json:"kind"`
	InsertText string    `json:"insertText"`
	FilterText string    `json:"filterText"`
	SortText   string    `json:"sortText"`
	TextEdit   *TextEdit `json:"textEdit"`
}

// TextEdit replaces a range with text.
type TextEdit struct {
	Range   Range  `json:"range"`
	NewText string `json:"newText"`
}

// WorkspaceEdit is a set of edits per file, as rename and code actions return them.
type WorkspaceEdit struct {
	Changes         map[string][]TextEdit `json:"changes"`
	DocumentChanges []struct {
		TextDocument struct {
			URI string `json:"uri"`
		} `json:"textDocument"`
		Edits []TextEdit `json:"edits"`
	} `json:"documentChanges"`
}

// Files returns the edits by file URI, from whichever form the server used.
func (w WorkspaceEdit) Files() map[string][]TextEdit {
	out := map[string][]TextEdit{}
	for u, e := range w.Changes {
		out[u] = append(out[u], e...)
	}
	for _, d := range w.DocumentChanges {
		out[d.TextDocument.URI] = append(out[d.TextDocument.URI], d.Edits...)
	}
	return out
}

// CodeAction is a fix or refactoring a server offers for a range.
type CodeAction struct {
	Title string         `json:"title"`
	Kind  string         `json:"kind"`
	Edit  *WorkspaceEdit `json:"edit"`
}

// Client is one running language server.
type Client struct {
	cmd  *exec.Cmd
	in   io.WriteCloser
	root string

	mu      sync.Mutex
	nextID  int
	pending map[int]chan response
	closed  bool
	err     error
	wmu     sync.Mutex

	// Encoding is "utf-32" or "utf-16", the position encoding agreed with the server.
	Encoding string
	// Name and Version are what the server says it is.
	Name, Version string
	caps          map[string]json.RawMessage

	// OnDiagnostics receives every textDocument/publishDiagnostics, from the client's reading
	// goroutine.
	OnDiagnostics func(uri string, diags []Diagnostic)
	// Done is closed when the server's process has ended.
	Done chan struct{}
}

type response struct {
	Result json.RawMessage `json:"result"`
	Error  *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// Start runs a server in root and initialises it. ctx bounds the start-up: a server that
// does not answer initialize in time is stopped and an error returned.
func Start(ctx context.Context, root string, argv []string, onDiag func(string, []Diagnostic)) (*Client, error) {
	if len(argv) == 0 {
		return nil, errors.New("lsp: no server command")
	}
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Dir = root
	proc.Bind(cmd) // ends with PyxForge (D10)
	proc.NoWindow(cmd)
	in, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	c := &Client{cmd: cmd, in: in, root: root, pending: map[int]chan response{}, OnDiagnostics: onDiag, Done: make(chan struct{})}
	go c.read(bufio.NewReader(out))
	go func() {
		err := cmd.Wait()
		c.fail(fmt.Errorf("lsp: %s ended: %v", filepath.Base(argv[0]), err))
		close(c.Done)
	}()
	if err := c.initialize(ctx); err != nil {
		c.Kill()
		return nil, err
	}
	return c, nil
}

func (c *Client) initialize(ctx context.Context) error {
	params := map[string]any{
		"processId": nil,
		"rootUri":   URI(c.root),
		"workspaceFolders": []map[string]string{
			{"uri": URI(c.root), "name": filepath.Base(c.root)},
		},
		"clientInfo": map[string]string{"name": "PyxForge"},
		"capabilities": map[string]any{
			"general": map[string]any{"positionEncodings": []string{"utf-32", "utf-16"}},
			"textDocument": map[string]any{
				"synchronization":    map[string]any{"didSave": true},
				"publishDiagnostics": map[string]any{"relatedInformation": false},
				"completion": map[string]any{
					"completionItem": map[string]any{"snippetSupport": false},
				},
				"hover":          map[string]any{"contentFormat": []string{"plaintext", "markdown"}},
				"definition":     map[string]any{},
				"references":     map[string]any{},
				"rename":         map[string]any{"prepareSupport": false},
				"formatting":     map[string]any{},
				"codeAction":     map[string]any{"codeActionLiteralSupport": map[string]any{"codeActionKind": map[string]any{"valueSet": []string{"quickfix", "refactor", "source"}}}},
				"documentSymbol": map[string]any{},
			},
			"workspace": map[string]any{"workspaceFolders": true, "applyEdit": false},
		},
	}
	var res struct {
		Capabilities map[string]json.RawMessage `json:"capabilities"`
		ServerInfo   struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		} `json:"serverInfo"`
	}
	if err := c.Call(ctx, "initialize", params, &res); err != nil {
		return fmt.Errorf("lsp: initialize: %w", err)
	}
	c.caps = res.Capabilities
	c.Name, c.Version = res.ServerInfo.Name, res.ServerInfo.Version
	c.Encoding = "utf-16"
	if raw, ok := res.Capabilities["positionEncoding"]; ok {
		var e string
		if json.Unmarshal(raw, &e) == nil && e == "utf-32" {
			c.Encoding = "utf-32"
		}
	}
	return c.Notify("initialized", map[string]any{})
}

// Supports reports whether the server announced a capability, such as "hoverProvider".
func (c *Client) Supports(capability string) bool {
	raw, ok := c.caps[capability]
	return ok && string(raw) != "false" && string(raw) != "null"
}

func (c *Client) fail(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return
	}
	c.closed, c.err = true, err
	for id, ch := range c.pending {
		close(ch)
		delete(c.pending, id)
	}
}

// Err is why the client stopped, or nil while it runs.
func (c *Client) Err() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.err
}

func (c *Client) write(msg map[string]any) error {
	msg["jsonrpc"] = "2.0"
	body, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	c.wmu.Lock()
	defer c.wmu.Unlock()
	if _, err := fmt.Fprintf(c.in, "Content-Length: %d\r\n\r\n", len(body)); err != nil {
		return err
	}
	_, err = c.in.Write(body)
	return err
}

// Notify sends a notification.
func (c *Client) Notify(method string, params any) error {
	if err := c.Err(); err != nil {
		return err
	}
	return c.write(map[string]any{"method": method, "params": params})
}

// Call sends a request and decodes its result into out (which may be nil).
func (c *Client) Call(ctx context.Context, method string, params any, out any) error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return c.err
	}
	c.nextID++
	id := c.nextID
	ch := make(chan response, 1)
	c.pending[id] = ch
	c.mu.Unlock()
	if err := c.write(map[string]any{"id": id, "method": method, "params": params}); err != nil {
		return err
	}
	select {
	case r, ok := <-ch:
		if !ok {
			return c.Err()
		}
		if r.Error != nil {
			return fmt.Errorf("%s: %s (%d)", method, r.Error.Message, r.Error.Code)
		}
		if out != nil && len(r.Result) > 0 && string(r.Result) != "null" {
			return json.Unmarshal(r.Result, out)
		}
		return nil
	case <-ctx.Done():
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		_ = c.Notify("$/cancelRequest", map[string]int{"id": id})
		return ctx.Err()
	}
}

func (c *Client) read(r *bufio.Reader) {
	for {
		n := -1
		for {
			line, err := r.ReadString('\n')
			if err != nil {
				return
			}
			line = strings.TrimSpace(line)
			if line == "" {
				break
			}
			if k, v, ok := strings.Cut(line, ":"); ok && strings.EqualFold(k, "Content-Length") {
				n, _ = strconv.Atoi(strings.TrimSpace(v))
			}
		}
		if n < 0 {
			continue
		}
		body := make([]byte, n)
		if _, err := io.ReadFull(r, body); err != nil {
			return
		}
		var msg struct {
			ID     *json.RawMessage `json:"id"`
			Method string           `json:"method"`
			Params json.RawMessage  `json:"params"`
			response
		}
		if json.Unmarshal(body, &msg) != nil {
			continue
		}
		switch {
		case msg.Method != "" && msg.ID != nil: // a request from the server
			c.answer(*msg.ID, msg.Method)
		case msg.Method == "textDocument/publishDiagnostics":
			var p struct {
				URI         string       `json:"uri"`
				Diagnostics []Diagnostic `json:"diagnostics"`
			}
			if json.Unmarshal(msg.Params, &p) == nil && c.OnDiagnostics != nil {
				c.OnDiagnostics(p.URI, p.Diagnostics)
			}
		case msg.ID != nil:
			var id int
			if json.Unmarshal(*msg.ID, &id) != nil {
				continue
			}
			c.mu.Lock()
			ch := c.pending[id]
			delete(c.pending, id)
			c.mu.Unlock()
			if ch != nil {
				ch <- msg.response
			}
		}
	}
}

// answer replies to the requests servers send clients: configuration gets empty settings,
// progress tokens and registrations are accepted, anything else is refused.
func (c *Client) answer(id json.RawMessage, method string) {
	reply := map[string]any{"id": id}
	switch method {
	case "workspace/configuration":
		reply["result"] = []any{nil}
	case "window/workDoneProgress/create", "client/registerCapability", "client/unregisterCapability":
		reply["result"] = nil
	case "workspace/workspaceFolders":
		reply["result"] = []map[string]string{{"uri": URI(c.root), "name": filepath.Base(c.root)}}
	default:
		reply["error"] = map[string]any{"code": -32601, "message": "not supported by PyxForge"}
	}
	_ = c.write(reply)
}

// Shutdown asks the server to exit and waits for it, killing it if it has not ended by the
// time ctx is done.
func (c *Client) Shutdown(ctx context.Context) {
	if c.Err() == nil {
		if c.Call(ctx, "shutdown", nil, nil) == nil {
			_ = c.Notify("exit", nil)
		}
	}
	select {
	case <-c.Done:
	case <-ctx.Done():
		c.Kill()
		<-c.Done
	}
}

// Kill ends the server's process at once.
func (c *Client) Kill() {
	if c.cmd.Process != nil {
		_ = c.cmd.Process.Kill()
	}
}

// URI is the file: URI of a path.
func URI(path string) string {
	p, err := filepath.Abs(path)
	if err != nil {
		p = path
	}
	p = filepath.ToSlash(p)
	if runtime.GOOS == "windows" {
		p = "/" + p
	}
	return (&url.URL{Scheme: "file", Path: p}).String()
}

// Path is the file path of a file: URI.
func Path(uri string) string {
	u, err := url.Parse(uri)
	if err != nil || u.Scheme != "file" {
		return ""
	}
	p := u.Path
	if runtime.GOOS == "windows" {
		p = strings.TrimPrefix(p, "/")
	}
	return filepath.FromSlash(p)
}

// timeout is how long one interactive request may take before the editor gives up on it.
const timeout = 5 * time.Second

// Context returns a context for one interactive request.
func Context() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), timeout)
}
