package hexview

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
)

// counting is a ReaderAt of any size whose byte at i is byte(i), counting what it is asked for.
type counting struct{ read int64 }

func (c *counting) ReadAt(p []byte, off int64) (int, error) {
	for i := range p {
		p[i] = byte(off + int64(i))
	}
	c.read += int64(len(p))
	return len(p), nil
}

func TestRow(t *testing.T) {
	off, hex, text := Row(0x10, []byte("AB\x00"))
	if off != "00000010" || hex[:9] != "41 42 00 " || text != "AB." {
		t.Fatalf("Row = %q %q %q", off, hex, text)
	}
}

// TestGigabyteReadsOnePage: a 1 GB source costs one screen of reads, wherever the view is.
func TestGigabyteReadsOnePage(t *testing.T) {
	test.NewTempApp(t)
	src := &counting{}
	v := New(src, 1<<30)
	w := test.NewTempWindow(t, v)
	w.Resize(fyne.NewSize(900, 400))
	v.GoTo(1 << 29)
	v.TypedKey(&fyne.KeyEvent{Name: fyne.KeyEnd})
	if src.read > 64<<10 {
		t.Fatalf("read %d bytes of a 1 GB source", src.read)
	}
	if got, want := v.Top(), v.maxTop(); got != want || want <= 0 {
		t.Fatalf("End: top %d, want %d", got, want)
	}
	v.TypedKey(&fyne.KeyEvent{Name: fyne.KeyHome})
	if v.Top() != 0 {
		t.Fatalf("Home: top %d", v.Top())
	}
}
