package lsp

import (
	"context"
	"encoding/json"
	"strings"
	"unicode/utf16"
)

type docID struct {
	URI string `json:"uri"`
}

type docPos struct {
	TextDocument docID    `json:"textDocument"`
	Position     Position `json:"position"`
}

// DidOpen tells the server a document is open, with its whole text.
func (c *Client) DidOpen(uri, languageID string, version int, text string) error {
	return c.Notify("textDocument/didOpen", map[string]any{
		"textDocument": map[string]any{"uri": uri, "languageId": languageID, "version": version, "text": text},
	})
}

// DidChange sends a document's whole new text (full synchronisation, which every server
// accepts; documents the editor opens are small enough that it costs little).
func (c *Client) DidChange(uri string, version int, text string) error {
	return c.Notify("textDocument/didChange", map[string]any{
		"textDocument":   map[string]any{"uri": uri, "version": version},
		"contentChanges": []map[string]string{{"text": text}},
	})
}

// DidSave tells the server the document was written.
func (c *Client) DidSave(uri string) error {
	return c.Notify("textDocument/didSave", map[string]any{"textDocument": docID{uri}})
}

// DidClose tells the server the document is closed.
func (c *Client) DidClose(uri string) error {
	return c.Notify("textDocument/didClose", map[string]any{"textDocument": docID{uri}})
}

// Completion asks for completions at p.
func (c *Client) Completion(ctx context.Context, uri string, p Position) ([]CompletionItem, error) {
	var raw json.RawMessage
	if err := c.Call(ctx, "textDocument/completion", docPos{docID{uri}, p}, &raw); err != nil {
		return nil, err
	}
	var list struct {
		Items []CompletionItem `json:"items"`
	}
	if len(raw) > 0 && raw[0] == '[' {
		err := json.Unmarshal(raw, &list.Items)
		return list.Items, err
	}
	if len(raw) == 0 {
		return nil, nil
	}
	err := json.Unmarshal(raw, &list)
	return list.Items, err
}

// Hover returns the server's description of the symbol at p as plain text ("" for none).
func (c *Client) Hover(ctx context.Context, uri string, p Position) (string, error) {
	var res struct {
		Contents json.RawMessage `json:"contents"`
	}
	if err := c.Call(ctx, "textDocument/hover", docPos{docID{uri}, p}, &res); err != nil {
		return "", err
	}
	return markupText(res.Contents), nil
}

// markupText flattens the three shapes hover contents come in.
func markupText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return strings.TrimSpace(s)
	}
	var m struct {
		Value string `json:"value"`
	}
	if raw[0] == '{' && json.Unmarshal(raw, &m) == nil {
		return strings.TrimSpace(m.Value)
	}
	var list []json.RawMessage
	if json.Unmarshal(raw, &list) == nil {
		var parts []string
		for _, r := range list {
			if t := markupText(r); t != "" {
				parts = append(parts, t)
			}
		}
		return strings.Join(parts, "\n\n")
	}
	return ""
}

// locations decodes Location, []Location or []LocationLink.
func locations(raw json.RawMessage) []Location {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	if raw[0] == '{' {
		var l Location
		if json.Unmarshal(raw, &l) == nil {
			return []Location{l}
		}
		return nil
	}
	var items []struct {
		Location
		TargetURI            string `json:"targetUri"`
		TargetSelectionRange *Range `json:"targetSelectionRange"`
	}
	if json.Unmarshal(raw, &items) != nil {
		return nil
	}
	out := make([]Location, 0, len(items))
	for _, it := range items {
		if it.TargetURI != "" && it.TargetSelectionRange != nil {
			out = append(out, Location{URI: it.TargetURI, Range: *it.TargetSelectionRange})
		} else {
			out = append(out, it.Location)
		}
	}
	return out
}

// Definition returns where the symbol at p is defined.
func (c *Client) Definition(ctx context.Context, uri string, p Position) ([]Location, error) {
	var raw json.RawMessage
	err := c.Call(ctx, "textDocument/definition", docPos{docID{uri}, p}, &raw)
	return locations(raw), err
}

// References returns every use of the symbol at p, its declaration included.
func (c *Client) References(ctx context.Context, uri string, p Position) ([]Location, error) {
	var raw json.RawMessage
	err := c.Call(ctx, "textDocument/references", map[string]any{
		"textDocument": docID{uri}, "position": p, "context": map[string]bool{"includeDeclaration": true},
	}, &raw)
	return locations(raw), err
}

// Rename returns the edits that rename the symbol at p.
func (c *Client) Rename(ctx context.Context, uri string, p Position, newName string) (WorkspaceEdit, error) {
	var w WorkspaceEdit
	err := c.Call(ctx, "textDocument/rename", map[string]any{"textDocument": docID{uri}, "position": p, "newName": newName}, &w)
	return w, err
}

// Format returns the edits that format the whole document.
func (c *Client) Format(ctx context.Context, uri string, tabSize int, spaces bool) ([]TextEdit, error) {
	var edits []TextEdit
	err := c.Call(ctx, "textDocument/formatting", map[string]any{
		"textDocument": docID{uri}, "options": map[string]any{"tabSize": tabSize, "insertSpaces": spaces},
	}, &edits)
	return edits, err
}

// CodeActions returns the fixes and refactorings for r, given the diagnostics there.
func (c *Client) CodeActions(ctx context.Context, uri string, r Range, diags []Diagnostic) ([]CodeAction, error) {
	var raw []json.RawMessage
	if diags == nil {
		diags = []Diagnostic{}
	}
	err := c.Call(ctx, "textDocument/codeAction", map[string]any{
		"textDocument": docID{uri}, "range": r, "context": map[string]any{"diagnostics": diags},
	}, &raw)
	var out []CodeAction
	for _, r := range raw {
		var a CodeAction
		if json.Unmarshal(r, &a) == nil && a.Title != "" && a.Edit != nil { // commands are not run
			out = append(out, a)
		}
	}
	return out, err
}

// Character converts a rune column on line to the server's position encoding.
func (c *Client) Character(line []rune, col int) int {
	col = min(max(col, 0), len(line))
	if c.Encoding == "utf-32" {
		return col
	}
	return len(utf16.Encode(line[:col]))
}

// Column converts a server character offset on line back to a rune column.
func (c *Client) Column(line []rune, character int) int {
	if c.Encoding == "utf-32" {
		return min(max(character, 0), len(line))
	}
	units := 0
	for i, r := range line {
		if units >= character {
			return i
		}
		units += len(utf16.Encode([]rune{r}))
	}
	return len(line)
}
