// Command forbidcheck fails when web technology appears in PyxForge outside the allowlisted
// paths (refactor-v2.md Section 2.1 and Section 16.4). Run it from the repository root:
//
//	go run ./tools/forbidcheck
//
// It checks tracked files and untracked files that are not ignored, so it also works as a
// pre-commit step.
package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path"
	"regexp"
	"strings"
)

// Paths that may contain forbidden technology: documentation, the 2.x stack until the parity
// gate (D8), and the project's earlier UI kit, which is reference material.
var allowedPrefixes = []string{"docs/", "legacy/", "pyforge_ui_kit/"}

var forbiddenExts = map[string]bool{
	".html": true, ".htm": true, ".css": true, ".js": true, ".mjs": true, ".cjs": true,
	".ts": true, ".jsx": true, ".tsx": true,
}

var forbiddenNames = map[string]bool{
	"package.json": true, "package-lock.json": true, "pnpm-lock.yaml": true, "yarn.lock": true,
	".mcp.json": true, "components.json": true,
}

var forbiddenNamePrefixes = []string{"tsconfig.", "vite.config.", "webpack.config.", "tailwind.config."}

// Words that mean a web runtime or web editor is being pulled in.
var forbiddenWords = regexp.MustCompile(`(?i)\b(tauri|electron|webview|wails|codemirror|monaco)\b`)

// Content is only scanned in text files that can import or configure something. Markdown may
// name these technologies when explaining the migration.
var scannedExts = map[string]bool{
	".go": true, ".mod": true, ".sum": true, ".lua": true, ".vim": true, ".toml": true,
	".yml": true, ".yaml": true, ".json": true, ".sh": true, ".ps1": true, ".bat": true,
}

// This tool's own source has to spell the words out.
const self = "tools/forbidcheck/"

func main() {
	out, err := exec.Command("git", "ls-files", "--cached", "--others", "--exclude-standard").Output()
	if err != nil {
		fmt.Fprintln(os.Stderr, "forbidcheck: listing files:", err)
		os.Exit(2)
	}
	files := strings.Split(strings.TrimSpace(string(out)), "\n")
	problems := check(files, os.ReadFile)
	for _, p := range problems {
		fmt.Println(p)
	}
	if len(problems) > 0 {
		fmt.Fprintf(os.Stderr, "forbidcheck: %d problem(s)\n", len(problems))
		os.Exit(1)
	}
}

// check returns one message per violation. read is os.ReadFile outside tests.
func check(files []string, read func(string) ([]byte, error)) []string {
	var problems []string
	for _, f := range files {
		f = strings.TrimSpace(strings.ReplaceAll(f, "\\", "/"))
		if f == "" || allowed(f) {
			continue
		}
		name := path.Base(f)
		ext := strings.ToLower(path.Ext(f))
		switch {
		case strings.Contains("/"+f, "/node_modules/"):
			problems = append(problems, f+": node_modules is forbidden")
			continue
		case forbiddenExts[ext]:
			problems = append(problems, f+": "+ext+" files are forbidden")
			continue
		case forbiddenNames[name]:
			problems = append(problems, f+": "+name+" is forbidden")
			continue
		}
		for _, prefix := range forbiddenNamePrefixes {
			if strings.HasPrefix(name, prefix) {
				problems = append(problems, f+": "+prefix+"* is forbidden")
			}
		}
		if !scannedExts[ext] || strings.HasPrefix(f, self) {
			continue
		}
		data, err := read(f)
		if err != nil {
			continue // deleted in the working tree but still in the index
		}
		for i, line := range bytes.Split(data, []byte("\n")) {
			if m := forbiddenWords.Find(line); m != nil {
				problems = append(problems, fmt.Sprintf("%s:%d: mentions %q", f, i+1, m))
			}
		}
	}
	return problems
}

func allowed(f string) bool {
	for _, p := range allowedPrefixes {
		if strings.HasPrefix(f, p) {
			return true
		}
	}
	return false
}
