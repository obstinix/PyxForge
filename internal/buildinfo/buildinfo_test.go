package buildinfo

import (
	"runtime"
	"testing"
)

func TestRead(t *testing.T) {
	i := Read()
	if i.Version != Version || i.Go != runtime.Version() || i.OS != runtime.GOOS || i.Arch != runtime.GOARCH {
		t.Errorf("Read() = %+v", i)
	}
	// The test binary links Fyne through the module graph only if a test imports it; this
	// package does not, so the version is reported as unknown rather than invented.
	if i.Fyne == "" {
		t.Error("Fyne version must never be empty")
	}
}

func TestShortRevision(t *testing.T) {
	if got := (Info{Revision: "0123456789abcdef"}).ShortRevision(); got != "0123456789ab" {
		t.Errorf("ShortRevision = %q", got)
	}
	if got := (Info{Revision: "abc"}).ShortRevision(); got != "abc" {
		t.Errorf("short revision = %q", got)
	}
}
