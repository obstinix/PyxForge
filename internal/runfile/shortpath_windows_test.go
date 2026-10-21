package runfile

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/obstinix/PyxForge/internal/toolchain"
	"golang.org/x/sys/windows"
)

// TestPdbBreakpointUnderAShortName: a script whose folder is named by its short (8.3) name, as
// in GitHub's Windows runner's temp folder, still stops at its breakpoint: pdb runs it under its
// long name, which a breakpoint naming the short path never matched.
func TestPdbBreakpointUnderAShortName(t *testing.T) {
	long := filepath.Join(t.TempDir(), "a folder with a long name")
	if err := os.Mkdir(long, 0o755); err != nil {
		t.Fatal(err)
	}
	buf := make([]uint16, windows.MAX_PATH)
	n, err := windows.GetShortPathName(windows.StringToUTF16Ptr(long), &buf[0], uint32(len(buf)))
	short := windows.UTF16ToString(buf[:n])
	if err != nil || n == 0 || strings.EqualFold(short, long) {
		t.Skip("this volume gives no short names")
	}
	write(t, filepath.Join(long, "hello.py"), "answer = 6 * 7\nprint(answer)\n")
	file := filepath.Join(short, "hello.py")
	p, err := ForDebug(file, t.TempDir(), toolchain.LookPath, Breakpoints{file: {2}})
	var missing *MissingError
	if errors.As(err, &missing) {
		t.Skip(missing.Error())
	}
	if err != nil {
		t.Fatal(err)
	}
	path, err := toolchain.LookPath(p.Run.Tool)
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(path, p.Run.Args...)
	cmd.Dir, cmd.Stdin = p.Run.Dir, strings.NewReader("p answer + 1\ncontinue\n")
	out, _ := cmd.CombinedOutput()
	if !strings.Contains(string(out), "hello.py(2)") || !strings.Contains(string(out), "43") {
		t.Errorf("pdb did not stop at the breakpoint:\n%s", out)
	}
}

// TestRealPathOfAMissingFile: a file that does not exist gets its folder's resolved name, so it
// still compares equal to the same path spelled with short names.
func TestRealPathOfAMissingFile(t *testing.T) {
	long := filepath.Join(t.TempDir(), "a folder with a long name")
	if err := os.Mkdir(long, 0o755); err != nil {
		t.Fatal(err)
	}
	buf := make([]uint16, windows.MAX_PATH)
	n, err := windows.GetShortPathName(windows.StringToUTF16Ptr(long), &buf[0], uint32(len(buf)))
	short := windows.UTF16ToString(buf[:n])
	if err != nil || n == 0 || strings.EqualFold(short, long) {
		t.Skip("this volume gives no short names")
	}
	if !samePath(filepath.Join(short, "missing.py"), filepath.Join(long, "missing.py")) {
		t.Errorf("%s and %s differ", realPath(filepath.Join(short, "missing.py")), realPath(filepath.Join(long, "missing.py")))
	}
}
