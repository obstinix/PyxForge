package shell

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"github.com/obstinix/PyxForge/internal/ui/codeedit"
	"github.com/obstinix/PyxForge/internal/ui/mdview"
)

func notesProject(t *testing.T, root string) {
	t.Helper()
	for name, text := range map[string]string{
		"boot/boot.asm":      "bits 16\nstart:\n    cli\nserial_put:\n    ret\n",
		"kernel/util.c":      "int util(void) { return 1; }\n",
		"drivers/util.c":     "int drv(void) { return 2; }\n",
		"notes/boot-hang.md": "# Boot hang\n\nSee [[boot.asm#serial_put]] and [[util.c]], [[gone.c]].\nThe kernel's util.c is involved, and boot.asm.\n",
	} {
		p := filepath.Join(root, filepath.FromSlash(name))
		os.MkdirAll(filepath.Dir(p), 0o755)
		if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func fixedDay(t *testing.T) {
	old := now
	t.Cleanup(func() { now = old })
	now = func() time.Time { return time.Date(2026, 11, 27, 9, 0, 0, 0, time.UTC) }
}

// TestNoteCommands: New Note makes the notes folder with its first note; Open Today's Dev Log
// makes the day's log from a template it writes for the user to edit, and uses their edit
// next time; New Note from Selection quotes the code with a link back; Insert Link to Current
// Location puts the link into the open note.
func TestNoteCommands(t *testing.T) {
	s, root, q := newEditorShell(t)
	fixedDay(t)
	os.WriteFile(filepath.Join(root, "main.c"), []byte("int main(void)\n{\n    return 0;\n}\n"), 0o644)
	if _, err := os.Stat(filepath.Join(root, "notes")); err == nil {
		t.Fatal("the notes folder exists before the first note")
	}
	p := s.createNote("First idea", "")
	if data, err := os.ReadFile(p); err != nil || string(data) != "# First idea\n\n" || filepath.Dir(p) != filepath.Join(root, "notes") {
		t.Fatalf("new note %s: %q %v", p, data, err)
	}
	if s.activeFile() != p {
		t.Error("the new note is not open")
	}
	if s.createNote("a/b", "") != "" {
		t.Error("a name with a slash made a note")
	}

	s.Commands().Run("notes.devLog")
	log := filepath.Join(root, "notes", "log", "2026-11-27.md")
	if data, _ := os.ReadFile(log); !strings.HasPrefix(string(data), "# Dev log 2026-11-27") {
		t.Fatalf("dev log %q", data)
	}
	tpl := filepath.Join(root, "notes", "templates", "dev-log.md")
	os.WriteFile(tpl, []byte("## {{date}} my own\n"), 0o644)
	os.Remove(log)
	s.Commands().Run("notes.devLog")
	if data, _ := os.ReadFile(log); string(data) != "## 2026-11-27 my own\n" {
		t.Errorf("the edited template was not used: %q", data)
	}

	s.OpenFile(filepath.Join(root, "main.c"))
	d := s.activeDoc()
	d.ed.Select(codeedit.Range{Start: codeedit.Pos{Line: 2, Col: 4}, End: codeedit.Pos{Line: 2, Col: 13}})
	s.Commands().Run("notes.fromSelection")
	q.pumpUntil(t, "the note dialog", func() bool { return s.lastDialog != nil })
	s.lastDialog.Hide()
	h := s.notesHost()
	if body := selectionNote(h.idx.LinkTo("main.c", 3), "c", d.ed.SelectedText()); body != "From [[main.c:3]] on 2026-11-27:\n\n```c\nreturn 0;\n```\n" {
		t.Errorf("the note from the selection reads %q", body)
	}

	// Insert Link to Current Location: into the note last active.
	s.OpenFile(p) // the first note becomes the last active note
	s.OpenFile(filepath.Join(root, "main.c"))
	s.activeDoc().ed.SetCursor(codeedit.Pos{Line: 1}, false)
	s.Commands().Run("notes.insertLink")
	if text := s.docFor(p).ed.Buf.Text(); !strings.Contains(text, "[[main.c:2]]") {
		t.Errorf("the note has %q", text)
	}
}

// TestBacklinksPanel: the notes linking to the active file, each with its line; for a note,
// its unlinked mentions and the links that lead nowhere; it follows renames and deletes.
func TestBacklinksPanel(t *testing.T) {
	s, root, q := newEditorShell(t)
	notesProject(t, root)
	s.OpenFile(filepath.Join(root, "boot", "boot.asm"))
	s.ShowBacklinks()
	h := s.notesHost()
	q.pumpUntil(t, "the index", func() bool { return h.idx != nil && len(h.backBody.Objects) > 0 })
	got := strings.Join(texts(h.backBody), "|")
	if !strings.Contains(got, "LINKED FROM 1") || !strings.Contains(got, "notes/boot-hang.md:3") {
		t.Fatalf("boot.asm's backlinks: %s", got)
	}
	note := filepath.Join(root, "notes", "boot-hang.md")
	s.OpenFile(note)
	got = strings.Join(texts(h.backBody), "|")
	if !strings.Contains(got, "UNLINKED MENTIONS 0") && !strings.Contains(got, "UNLINKED MENTIONS") {
		t.Errorf("no mentions section: %s", got)
	}
	if !strings.Contains(got, "LINKS THAT LEAD NOWHERE 2") || !strings.Contains(got, "[[util.c]] (line 3)") {
		t.Errorf("issues: %s", got)
	}
	if is := h.idx.Issues("notes/boot-hang.md"); len(is) != 2 || len(is[0].To.Ambiguous) != 2 || !is[1].To.Missing {
		t.Errorf("issues %+v", is)
	}
	// boot.asm is renamed on disk: the note's link to it leads nowhere until the note changes.
	os.Rename(filepath.Join(root, "boot", "boot.asm"), filepath.Join(root, "boot", "stage1.asm"))
	s.explorer.OnFilesChanged([]string{filepath.Join(root, "boot", "boot.asm"), filepath.Join(root, "boot", "stage1.asm")})
	got = strings.Join(texts(h.backBody), "|")
	if !strings.Contains(got, "LINKS THAT LEAD NOWHERE 3") {
		t.Errorf("after the rename: %s", got)
	}
	// A deleted note's links go.
	os.Remove(note)
	s.explorer.OnFilesChanged([]string{note})
	if bl := h.idx.Backlinks("kernel/util.c"); len(bl) != 0 {
		t.Errorf("the deleted note still links: %v", bl)
	}
}

// TestMarkdownPreview: the preview opens beside its note, follows edits, and its links open
// files: a symbol link at the symbol's line, an ambiguous one as a choice.
func TestMarkdownPreview(t *testing.T) {
	s, root, q := newEditorShell(t)
	notesProject(t, root)
	note := filepath.Join(root, "notes", "boot-hang.md")
	s.OpenFile(note)
	s.Commands().Run("notes.preview")
	h := s.notesHost()
	pv := h.previews[note]
	if pv == nil || len(s.groups) != 2 || s.groupOf(pv.item) == s.groupOf(s.tabFor(note)) {
		t.Fatalf("the preview is not beside the note: %d groups", len(s.groups))
	}
	heading := func() string {
		for _, seg := range pv.view.Segments() {
			if ts, ok := seg.(*widget.TextSegment); ok {
				return ts.Text
			}
		}
		return ""
	}
	if heading() != "Boot hang" {
		t.Fatalf("preview heading %q", heading())
	}
	d := s.docFor(note)
	d.ed.SetCursor(codeedit.Pos{Line: 0, Col: 11}, false)
	for _, r := range " notes" {
		test.Type(d.ed, string(r))
	}
	if heading() != "Boot hang notes" {
		t.Errorf("the preview did not follow the edit: %q", heading())
	}
	// [[boot.asm#serial_put]]: no language server here, so the text search's line (4).
	h.previewLink("notes/boot-hang.md", note, mdview.Link{Target: "boot.asm#serial_put", Wiki: true})
	q.pumpUntil(t, "the link to open", func() bool { return s.activeFile() == filepath.Join(root, "boot", "boot.asm") })
	if c := s.activeDoc().ed.Cur; c.Line != 3 {
		t.Errorf("serial_put opened at line %d, want 4", c.Line+1)
	}
	h.previewLink("notes/boot-hang.md", note, mdview.Link{Target: "util.c", Wiki: true})
	if !s.palette.Visible() || len(s.palette.Results()) != 2 {
		t.Errorf("an ambiguous name did not offer the choice: %v", s.palette.Results())
	}
	s.palette.Hide()
	names := []string{}
	for _, it := range s.palette.Results() {
		names = append(names, it.Detail)
	}
	if !slices.Equal(names, []string{"drivers/util.c", "kernel/util.c"}) {
		t.Errorf("choices %v", names)
	}
}
