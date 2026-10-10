// Package buildinfo reports what this PyxForge binary is: its release, the toolchain that
// built it and the source revision, for `pyxforge version` and the About dialog.
package buildinfo

import (
	"runtime"
	"runtime/debug"
)

// Version is the PyxForge release this build belongs to. Release builds set it with
// -ldflags "-X github.com/obstinix/PyxForge/internal/buildinfo.Version=…" (tools/release).
var Version = "3.0.0-dev"

// Info describes one build.
type Info struct {
	Version  string `json:"version"`
	Go       string `json:"go"`
	Fyne     string `json:"fyne"`
	Revision string `json:"revision,omitempty"` // VCS commit, when built from a checkout
	Modified bool   `json:"modified,omitempty"` // the checkout had uncommitted changes
	OS       string `json:"os"`
	Arch     string `json:"arch"`
}

// Read returns the running binary's build information. Fields the Go toolchain did not
// record (a test binary has no VCS stamp) are left empty.
func Read() Info {
	info := Info{Version: Version, Go: runtime.Version(), Fyne: "unknown", OS: runtime.GOOS, Arch: runtime.GOARCH}
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return info
	}
	for _, d := range bi.Deps {
		if d.Path == "fyne.io/fyne/v2" {
			info.Fyne = d.Version
			if d.Replace != nil {
				info.Fyne = d.Replace.Version
			}
		}
	}
	for _, s := range bi.Settings {
		switch s.Key {
		case "vcs.revision":
			info.Revision = s.Value
		case "vcs.modified":
			info.Modified = s.Value == "true"
		}
	}
	return info
}

// ShortRevision is the first 12 characters of the revision, or "" when unknown.
func (i Info) ShortRevision() string {
	if len(i.Revision) > 12 {
		return i.Revision[:12]
	}
	return i.Revision
}
