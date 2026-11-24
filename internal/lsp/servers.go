package lsp

import (
	"os"
	"path/filepath"
	"strings"
)

// Server is a language server PyxForge knows how to start: the same table as the Neovim
// configuration's (nvim/lua/pyxforge/lsp.lua), keyed by file extension instead of filetype.
type Server struct {
	Name string
	// Commands are the ways to start it, in order of preference; the first whose program is
	// installed is used.
	Commands [][]string
	// LanguageID is the LSP language identifier for each extension the server takes.
	Exts map[string]string
	// Markers are files whose folder is the project root, searched upwards from the file.
	Markers []string
	// Settings are sent with workspace/didChangeConfiguration after start-up (pyright
	// analyses nothing without them).
	Settings any
}

// Servers is every server PyxForge starts.
var Servers = []Server{
	{Name: "clangd", Commands: [][]string{{"clangd"}},
		Exts:    map[string]string{".c": "c", ".h": "c", ".cc": "cpp", ".cpp": "cpp", ".cxx": "cpp", ".hpp": "cpp", ".hh": "cpp", ".hxx": "cpp"},
		Markers: []string{"compile_commands.json", ".clangd", "pyxforge.toml", ".git"}},
	{Name: "gopls", Commands: [][]string{{"gopls"}},
		Exts: map[string]string{".go": "go", ".mod": "go.mod"}, Markers: []string{"go.mod", ".git"}},
	{Name: "rust-analyzer", Commands: [][]string{{"rust-analyzer"}},
		Exts: map[string]string{".rs": "rust"}, Markers: []string{"Cargo.toml", ".git"}},
	{Name: "asm-lsp", Commands: [][]string{{"asm-lsp"}},
		Exts:    map[string]string{".asm": "nasm", ".nasm": "nasm", ".s": "asm", ".inc": "nasm"},
		Markers: []string{".asm-lsp.toml", "pyxforge.toml", ".git"}},
	{Name: "lua-language-server", Commands: [][]string{{"lua-language-server"}},
		Exts: map[string]string{".lua": "lua"}, Markers: []string{".luarc.json", ".git"}},
	{Name: "taplo", Commands: [][]string{{"taplo", "lsp", "stdio"}},
		Exts: map[string]string{".toml": "toml"}, Markers: []string{"pyxforge.toml", ".git"}},
	{Name: "marksman", Commands: [][]string{{"marksman", "server"}},
		Exts: map[string]string{".md": "markdown", ".markdown": "markdown"}, Markers: []string{".marksman.toml", ".git"}},
	{Name: "bash-language-server", Commands: [][]string{{"bash-language-server", "start"}},
		Exts: map[string]string{".sh": "shellscript", ".bash": "shellscript"}, Markers: []string{".git"}},
	{Name: "pyright", Commands: [][]string{{"pyright-langserver", "--stdio"}, {"basedpyright-langserver", "--stdio"}},
		Exts: map[string]string{".py": "python", ".pyi": "python"},
		Settings: map[string]any{
			"python":       map[string]any{"analysis": map[string]any{"diagnosticMode": "openFilesOnly", "autoSearchPaths": true}},
			"basedpyright": map[string]any{"analysis": map[string]any{"diagnosticMode": "openFilesOnly"}},
		},
		Markers: []string{"pyproject.toml", "pyrightconfig.json", "setup.py", "setup.cfg", "requirements.txt", ".git"}},
	{Name: "typescript-language-server", Commands: [][]string{{"typescript-language-server", "--stdio"}},
		Exts:    map[string]string{".js": "javascript", ".mjs": "javascript", ".cjs": "javascript", ".jsx": "javascriptreact", ".ts": "typescript", ".tsx": "typescriptreact"},
		Markers: []string{"tsconfig.json", "jsconfig.json", "package.json", ".git"}},
}

// ForFile is the server for a file, by its extension, and the file's language identifier.
func ForFile(path string) (*Server, string) {
	ext := strings.ToLower(filepath.Ext(path))
	for i := range Servers {
		if id, ok := Servers[i].Exts[ext]; ok {
			return &Servers[i], id
		}
	}
	return nil, ""
}

// Command is the first of the server's commands whose program lookPath finds, with the
// program resolved to its full path (on Windows that finds npm's .cmd launchers). nil when
// none is installed.
func (s *Server) Command(lookPath func(string) (string, error)) []string {
	for _, c := range s.Commands {
		if p, err := lookPath(c[0]); err == nil {
			return append([]string{p}, c[1:]...)
		}
	}
	return nil
}

// Root is the folder of the nearest marker above path, else fallback (the workspace).
func (s *Server) Root(path, fallback string) string {
	dir := filepath.Dir(path)
	for {
		for _, m := range s.Markers {
			if _, err := os.Stat(filepath.Join(dir, m)); err == nil {
				return dir
			}
		}
		up := filepath.Dir(dir)
		if up == dir {
			break
		}
		dir = up
	}
	if fallback != "" {
		return fallback
	}
	return filepath.Dir(path)
}
