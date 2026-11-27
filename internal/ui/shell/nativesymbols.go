package shell

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/obstinix/PyxForge/internal/diff"
	"github.com/obstinix/PyxForge/internal/lsp"
	"github.com/obstinix/PyxForge/internal/ui/commandpalette"
	"github.com/obstinix/PyxForge/internal/ui/icons"
	"github.com/obstinix/PyxForge/internal/ui/notifications"
)

// Find References and Go to Symbol in File: the language server's answers listed in the
// command palette, where they can be searched, and the chosen one opened at its place.

// references lists every use of the symbol at the cursor (Shift+F12 in the editor).
func (h *nativeHost) references(d *nativeDoc) {
	if d.client == nil {
		h.s.Notify(notifications.Info, "No language server", "Find References needs a language server for this file type.")
		return
	}
	c, uri := d.client, d.uri
	word := d.ed.Buf.Slice(d.ed.WordAt(d.ed.Cur))
	pos := lsp.Position{Line: d.ed.Cur.Line, Character: c.Character(d.ed.Buf.Line(d.ed.Cur.Line), d.ed.Cur.Col)}
	go func() {
		ctx, cancel := lsp.Context()
		defer cancel()
		locs, err := c.References(ctx, uri, pos)
		h.s.dispatch(func() {
			if err != nil || len(locs) == 0 {
				h.s.Notify(notifications.Info, "No references found", "The language server knows no uses of the symbol at the cursor.")
				return
			}
			h.s.palette.Show(fmt.Sprintf("%d references to %s", len(locs), word), h.locationItems(locs, c))
		})
	}()
}

// locationItems turns locations into palette items: the file and line, then the line's text.
func (h *nativeHost) locationItems(locs []lsp.Location, c *lsp.Client) []commandpalette.Item {
	files := map[string][]string{} // a file's lines, read once
	lineOf := func(path string, n int) string {
		lines, ok := files[path]
		if !ok {
			if d := h.docFor(path); d != nil {
				for i := 0; i < d.ed.Buf.LineCount(); i++ {
					lines = append(lines, d.ed.Buf.LineText(i))
				}
			} else if data, err := os.ReadFile(path); err == nil {
				lines = diff.Lines(string(data))
			}
			files[path] = lines
		}
		if n < len(lines) {
			return strings.TrimSpace(lines[n])
		}
		return ""
	}
	items := make([]commandpalette.Item, 0, len(locs))
	for _, l := range locs {
		path, at := lsp.Path(l.URI), l.Range.Start
		name := path
		if rel, ok := relWithin(h.s.root, path); ok && h.s.root != "" {
			name = rel
		}
		items = append(items, commandpalette.Item{
			Title: fmt.Sprintf("%s:%d", name, at.Line+1), Detail: lineOf(path, at.Line), Icon: icons.FileCode,
			Run: func() { h.goTo(path, at, c) },
		})
	}
	return items
}

// symbols lists the document's symbols (Go to Symbol in File, Ctrl+Shift+O); nested ones are
// indented under the symbol that holds them.
func (h *nativeHost) symbols(d *nativeDoc) {
	if d.client == nil {
		h.s.Notify(notifications.Info, "No language server", "Go to Symbol needs a language server for this file type.")
		return
	}
	c, uri, path := d.client, d.uri, d.path
	go func() {
		ctx, cancel := lsp.Context()
		defer cancel()
		syms, err := c.DocumentSymbols(ctx, uri)
		h.s.dispatch(func() {
			if err != nil || len(syms) == 0 {
				h.s.Notify(notifications.Info, "No symbols found", "The language server lists no symbols in "+filepath.Base(path)+".")
				return
			}
			items := make([]commandpalette.Item, 0, len(syms))
			for _, s := range syms {
				at := s.Selection.Start
				detail := lsp.KindName(s.Kind)
				if s.Detail != "" {
					detail += " · " + s.Detail
				}
				items = append(items, commandpalette.Item{
					Title: strings.Repeat("  ", s.Depth) + s.Name, Detail: fmt.Sprintf("%s · line %d", detail, at.Line+1),
					Icon: icons.ListTree, Run: func() { h.goTo(path, at, c) },
				})
			}
			h.s.palette.Show("Go to a symbol in "+filepath.Base(path), items)
		})
	}()
}
