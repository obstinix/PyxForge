package runfile

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// Programs of more than one source file.

var (
	cExts   = []string{".c"}
	cxxExts = []string{".cpp", ".cc", ".cxx", ".c++"}
	// int main(…), void main(…), auto main(…) -> int, and WinMain: the start of a program.
	definesMain = regexp.MustCompile(`(?m)^\s*(?:[\w:<>\*&]+\s+)+(?:w?main|wWinMain|WinMain)\s*\(`)
)

// withSiblings is the file to run and the other sources of its language in its folder that
// are part of the same program: every one that doesn't define a main of its own. A folder of
// main.c, list.c and list.h builds as one program whichever of the two .c files is open with
// main in it; a folder of separate exercises, each with its main, builds only the open one.
// The names are relative to the folder, the open file first, the rest in name order.
func withSiblings(file string, exts []string) []string {
	dir, base := filepath.Dir(file), filepath.Base(file)
	out := []string{base}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return out
	}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || strings.EqualFold(name, base) || !slices.Contains(exts, strings.ToLower(filepath.Ext(name))) {
			continue
		}
		src, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil || definesMain.Match(src) {
			continue
		}
		out = append(out, name)
	}
	slices.Sort(out[1:])
	return out
}

// javaLayout reads where a Java file sits: the class to start (the file's name, in the package
// the file declares) and the source root its package is under, which is the file's folder for
// a file without a package. com/example/app/Main.java declaring package com.example.app has
// its root three folders up; a declaration that doesn't match the folders leaves the root at
// the file's folder, and javac says what is wrong.
func javaLayout(file string) (class, root string) {
	dir := filepath.Dir(file)
	class, root = strings.TrimSuffix(filepath.Base(file), filepath.Ext(file)), dir
	src, err := os.ReadFile(file)
	if err != nil {
		return class, root
	}
	m := javaPackage.FindSubmatch(src)
	if m == nil {
		return class, root
	}
	pkg := string(m[1])
	class = pkg + "." + class
	parts := strings.Split(pkg, ".")
	up := dir
	for i := len(parts) - 1; i >= 0; i-- {
		if !strings.EqualFold(filepath.Base(up), parts[i]) {
			return class, root
		}
		up = filepath.Dir(up)
	}
	return class, up
}
