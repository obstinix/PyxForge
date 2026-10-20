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

// findButton returns the button labelled text under o, looking inside containers and
// scrolls, or nil.
func findButton(o fyne.CanvasObject, text string) *widget.Button {
	switch w := o.(type) {
	case *widget.Button:
		if w.Text == text {
			return w
		}
	case *fyne.Container:
		for _, c := range w.Objects {
			if b := findButton(c, text); b != nil {
				return b
			}
		}
	case *container.Scroll:
		return findButton(w.Content, text)
	}
	return nil
}

// TestBinaryFilesAreNotEditedAsText: a binary file opens in a tab of its own, not in Neovim,
// so Save and Save All cannot rewrite it; opened as text on request, it is read-only in binary
// mode, and writing it leaves every byte as it was. Text files with unusual bytes still open
// in the editor.
func TestBinaryFilesAreNotEditedAsText(t *testing.T) {
	s, root, q := newEditorShell(t)
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
	if s.ed != nil && len(s.ed.bufs) != 0 {
		t.Fatal("the binary file was sent to Neovim")
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
	q.pumpUntil(t, "the UTF-16 file in Neovim", func() bool {
		return len(s.editors.Items) == 2 && s.editors.Selected().Text == "notes utf16.txt"
	})

	// Open as Text puts the binary file in Neovim, read-only, in binary mode.
	s.editors.Select(tab)
	open := findButton(tab.Content, "Open as Text (Read-Only)")
	if open == nil {
		t.Fatal("no Open as Text button")
	}
	test.Tap(open)
	q.pumpUntil(t, "the binary file in Neovim", func() bool {
		return len(s.editors.Items) == 2 && s.editors.Selected().Text == "image.bin" && s.open[bin] == nil
	})
	var mode string
	if err := s.ed.sess.ExecLua(`return tostring(vim.bo.binary) .. " " .. tostring(vim.bo.readonly)`, &mode); err != nil || mode != "true true" {
		t.Fatalf("binary readonly = %q, %v; want true true", mode, err)
	}
	if err := s.ed.sess.Command("write!"); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(bin); !bytes.Equal(got, data) {
		t.Errorf("writing the binary file from Neovim changed it: %d bytes, was %d", len(got), len(data))
	}
	// Opening it again from the explorer selects the editor tab rather than adding another.
	s.editors.SelectIndex(0)
	s.OpenFile(bin)
	if len(s.editors.Items) != 2 || s.editors.Selected().Text != "image.bin" {
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
