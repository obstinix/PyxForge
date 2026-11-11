package runfile

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var javaPackage = regexp.MustCompile(`(?m)^\s*package\s+([\w.]+)\s*;`)

// javaDebug plans Java under jdb (#65): the file is compiled with debug information into the
// build folder, and jdb starts its class from there. jdb takes no breakpoints on its command line
// but runs the commands of a jdb.ini in its user.home when it starts, so user.home is set to the
// build folder, which holds one; the program itself still runs in its source folder. The class
// is the file's name, in the package the file declares.
func javaDebug(file, dir, out string, look func(string) (string, error), breaks Breakpoints) (Plan, error) {
	jdkHint := hint("pyxforge toolchains install jdk (or Tools > Dependency Manager)", "sudo apt-get install default-jdk")
	for _, tool := range []string{"javac", "jdb"} {
		if _, err := look(tool); err != nil {
			return Plan{}, &MissingError{Language: "Java debugging", Tools: []string{tool}, Hint: jdkHint}
		}
	}
	src, err := os.ReadFile(file)
	if err != nil {
		return Plan{}, err
	}
	class := strings.TrimSuffix(filepath.Base(file), filepath.Ext(file))
	if m := javaPackage.FindSubmatch(src); m != nil {
		class = string(m[1]) + "." + class
	}
	classes := outFor(out, file)
	start := ""
	for _, b := range breaks.at(func(f string) bool { return samePath(f, file) }) {
		start += fmt.Sprintf("stop at %s:%d\n", class, b.line)
	}
	if start == "" {
		start = "stop in " + class + ".main\n"
	}
	return Plan{
		Language: "Java",
		Build:    []Step{{Tool: "javac", Args: []string{"-g", "-d", classes, file}, Dir: dir}},
		Out:      classes,
		Files:    map[string]string{filepath.Join(classes, "jdb.ini"): start + "run\n"},
		Run: Step{Tool: "jdb", Args: []string{"-J-Duser.home=" + classes, "-sourcepath", dir, "-classpath", classes, class},
			Dir: dir},
	}, nil
}
