package shell

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

// findButton returns the visible button labelled text under o, looking inside containers,
// scrolls and other widgets' renderers (dialogs, pop-ups), or nil.
func findButton(o fyne.CanvasObject, text string) *widget.Button {
	if o == nil || !o.Visible() {
		return nil
	}
	switch w := o.(type) {
	case *widget.Button:
		if w.Text == text {
			return w
		}
	case *dialogButton: // a dialog's button (escdialog.go)
		if w.Text == text {
			return &w.Button
		}
	case *fyne.Container:
		for _, c := range w.Objects {
			if b := findButton(c, text); b != nil {
				return b
			}
		}
	case *container.Scroll:
		return findButton(w.Content, text)
	case fyne.Widget:
		for _, c := range test.WidgetRenderer(w).Objects() {
			if b := findButton(c, text); b != nil {
				return b
			}
		}
	}
	return nil
}

// TestBinaryFilesAreNotEditedAsText: a binary file opens in a tab of its own, not in the
// editor, so Save and Save All cannot rewrite it; opened as text on request, it is read-only,
// and saving it leaves every byte as it was. UTF-16 text, full of NUL bytes, still opens in
// the editor.
func TestBinaryFilesAreNotEditedAsText(t *testing.T) {
	s, root, _ := newEditorShell(t)
	data := bytes.Repeat([]byte{0x00, 0x01, 0xFE, 0xFF, 'A'}, 205) // 1,025 bytes, no final newline
	bin := filepath.Join(root, "image.bin")
	utf16 := filepath.Join(root, "notes utf16.txt")
	for p, body := range map[string][]byte{bin: data, utf16: {0xFF, 0xFE, 'h', 0, 'i', 0, '\r', 0, '\n', 0}} {
		if err := os.WriteFile(p, body, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	s.OpenFile(bin)
	tab := s.open[bin]
	if tab == nil || s.editors.Selected() != tab || tab.Text != "image.bin" {
		t.Fatalf("the binary file did not open in its own tab: %+v", s.editors.Items)
	}
	if s.nat != nil && len(s.nat.docs) != 0 {
		t.Fatal("the binary file was opened in the editor")
	}
	s.saveAll()
	s.saveCurrent()
	if got, _ := os.ReadFile(bin); !bytes.Equal(got, data) {
		t.Fatalf("saving changed the binary file: %d bytes, was %d", len(got), len(data))
	}
	s.OpenFile(bin) // already open: selects the tab
	if len(s.editors.Items) != 1 {
		t.Fatalf("%d tabs after opening the binary file twice", len(s.editors.Items))
	}

	// UTF-16 text is full of NUL bytes, but its byte-order mark makes it text.
	s.OpenFile(utf16)
	if d := s.activeDoc(); d == nil || d.item.Text != "notes utf16.txt" || d.ed.Buf.Text() != "hi\n" {
		t.Fatalf("the UTF-16 file did not open as text: tabs %v", tabTitles(s))
	}

	// Open as Text shows the binary file in the editor, read-only.
	s.editors.Select(tab)
	open := findButton(tab.Content, "Open as Text (Read-Only)")
	if open == nil {
		t.Fatal("no Open as Text button")
	}
	test.Tap(open)
	d := s.activeDoc()
	if d == nil || len(s.editors.Items) != 2 || d.item.Text != "image.bin (read-only)" || s.open[bin] != nil {
		t.Fatalf("the binary file is not open as text: tabs %v", tabTitles(s))
	}
	test.Type(d.ed, "x")
	s.saveCurrent()
	if got, _ := os.ReadFile(bin); !bytes.Equal(got, data) || d.ed.Buf.Modified() {
		t.Errorf("typing and saving changed the binary file: %d bytes, was %d", len(got), len(data))
	}
	// Opening it again from the explorer selects the editor tab rather than adding another.
	s.editors.SelectIndex(0)
	s.OpenFile(bin)
	if len(s.editors.Items) != 2 || s.editors.Selected() != d.item {
		t.Errorf("reopening the file gave tabs %v", tabTitles(s))
	}
}

func tabTitles(s *Shell) []string {
	var out []string
	for _, it := range s.editors.Items {
		out = append(out, it.Text)
	}
	return out
}
