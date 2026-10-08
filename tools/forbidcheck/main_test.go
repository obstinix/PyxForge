package main

import (
	"errors"
	"strings"
	"testing"
)

func TestCheck(t *testing.T) {
	contents := map[string]string{
		"internal/ui/shell/shell.go": "package shell\n",
		"internal/editor/web.go":     "package editor\n\nimport \"github.com/webview/webview_go\"\n",
		"go.sum":                     "fyne.io/fyne/v2 v2.8.1 h1:abc=\n",
		"nvim/init.lua":              "-- Monaco keymap emulation\n",
		"README.md":                  "PyxForge 2.x used Tauri.\n",
		"tools/forbidcheck/main.go":  "tauri electron webview\n",
	}
	read := func(name string) ([]byte, error) {
		if s, ok := contents[name]; ok {
			return []byte(s), nil
		}
		return nil, errors.New("not found")
	}
	files := []string{
		"internal/ui/shell/shell.go",
		"internal/editor/web.go",
		"go.sum",
		"nvim/init.lua",
		"README.md",
		"tools/forbidcheck/main.go",
		"legacy/desktop/index.html",
		"legacy/extension/package.json",
		"docs/design/reference-screens/x.css",
		"pyforge_ui_kit/ink-and-paper/main-workspace/code.html",
		"internal/ui/preview.html",
		"package.json",
		".mcp.json",
		"internal/ui/tsconfig.json",
		"assets/node_modules/left-pad/index.txt",
		`internal\ui\styles.CSS`,
	}

	got := check(files, read)
	want := []string{
		"internal/editor/web.go:3:",
		"nvim/init.lua:1:",
		"internal/ui/preview.html:",
		"package.json:",
		".mcp.json:",
		"internal/ui/tsconfig.json:",
		"assets/node_modules/left-pad/index.txt:",
		"internal/ui/styles.CSS:",
	}
	if len(got) != len(want) {
		t.Fatalf("got %d problems, want %d:\n%s", len(got), len(want), strings.Join(got, "\n"))
	}
	for i := range want {
		if !strings.HasPrefix(got[i], want[i]) {
			t.Errorf("problem %d = %q, want prefix %q", i, got[i], want[i])
		}
	}
}
