package shell

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"fyne.io/fyne/v2"
)

// TestMoveByDragAndUndo: a file moved into a folder takes its open tab along, and Ctrl+Z in
// the Explorer moves it back.
func TestMoveByDragAndUndo(t *testing.T) {
	s, root, _ := newEditorShell(t)
	src := filepath.Join(root, "main.c")
	dst := filepath.Join(root, "src")
	os.WriteFile(src, []byte("int main;\n"), 0o644)
	os.Mkdir(dst, 0o755)
	s.OpenFile(src)
	if !canMove(src, dst) || canMove(src, root) || canMove(dst, filepath.Join(dst, "x")) {
		t.Fatal("canMove is wrong")
	}
	s.moveEntry(src, dst)
	moved := filepath.Join(dst, "main.c")
	if _, err := os.Stat(moved); err != nil {
		t.Fatalf("not moved: %v", err)
	}
	if s.nat.docFor(moved) == nil {
		t.Fatal("the open tab did not follow the file")
	}
	s.explorer.OnUndo()
	if _, err := os.Stat(src); err != nil {
		t.Fatalf("not moved back: %v", err)
	}
	if s.nat.docFor(src) == nil {
		t.Fatal("the open tab did not follow the file back")
	}
	if s.undoMove != nil {
		t.Fatal("the move can be undone twice")
	}
}

// TestGhostFollowsTheDrag: while a row is dragged a label is drawn, and it goes when the row
// is let go.
func TestGhostFollowsTheDrag(t *testing.T) {
	s, root, _ := newEditorShell(t)
	src := filepath.Join(root, "a.c")
	os.WriteFile(src, nil, 0o644)
	s.dragRow(src, false, fyne.NewPos(5000, 5000)) // nowhere: no target
	if s.ghost == nil || !s.ghost.Visible() {
		t.Fatal("no label while dragging")
	}
	s.dropRow(src, false, fyne.NewPos(5000, 5000))
	if s.ghost.Visible() {
		t.Fatal("the label stayed after the drop")
	}
	if _, err := os.Stat(src); err != nil {
		t.Fatal("a drop on nothing moved the file")
	}
}

func TestShellQuote(t *testing.T) {
	if got := shellQuote(`C:\src\main.c`); got != `C:\src\main.c` {
		t.Errorf("plain path quoted: %s", got)
	}
	want := `'/tmp/my file'`
	if runtime.GOOS == "windows" {
		want = `"/tmp/my file"`
	}
	if got := shellQuote("/tmp/my file"); got != want {
		t.Errorf("quoted %s, want %s", got, want)
	}
}
