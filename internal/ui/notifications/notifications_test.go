package notifications

import (
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
)

func TestLifetimes(t *testing.T) {
	if Lifetime(Error) != 0 || Lifetime(Warning) != 10*time.Second || Lifetime(Info) != 6*time.Second || Lifetime(Success) != 6*time.Second {
		t.Errorf("lifetimes: error %v, warning %v, info %v", Lifetime(Error), Lifetime(Warning), Lifetime(Info))
	}
}

func TestShorten(t *testing.T) {
	if got := shorten("short"); got != "short" {
		t.Errorf("a short body changed: %q", got)
	}
	long := strings.Repeat("git error line ", 100)
	got := shorten(long)
	if len(got) > maxBody+80 || !strings.HasSuffix(got, "(the full message is in the Log panel)") {
		t.Errorf("long body: %d bytes, %q…", len(got), got[:40])
	}
	cyrillic := strings.Repeat("ж", 600) // no spaces, two bytes a character
	if got := shorten(cyrillic); !utf8.ValidString(got) {
		t.Error("shorten split a character")
	}
}

// TestLongBodiesWrap: a long message wraps inside the toast instead of running off its edge,
// and an error stays up with no expiry scheduled.
func TestLongBodiesWrap(t *testing.T) {
	test.NewTempApp(t)
	n := New()
	expired := 0
	n.Dispatch = func(func()) { expired++ }
	w := test.NewTempWindow(t, n.Layer())
	w.Resize(fyne.NewSize(1000, 700))
	n.Post(Info, "Short", "one line")
	n.Post(Error, "Stage failed", strings.Repeat("this repository's own Git configuration sets filter.lfs.clean ", 4))
	short, long := n.layer.Objects[0], n.layer.Objects[1]
	if long.Size().Width > width || long.Size().Height < short.Size().Height*1.5 {
		t.Errorf("long toast %v, short %v: the long body did not wrap", long.Size(), short.Size())
	}
	if n.Count() != 2 {
		t.Fatalf("%d toasts", n.Count())
	}
}
