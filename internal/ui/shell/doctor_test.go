package shell

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/obstinix/PyxForge/internal/toolchain"
	"github.com/obstinix/PyxForge/internal/ui/kit"
)

// texts collects every piece of text shown under o.
func texts(o fyne.CanvasObject) []string {
	var out []string
	var walk func(fyne.CanvasObject)
	walk = func(o fyne.CanvasObject) {
		switch w := o.(type) {
		case *kit.Text:
			out = append(out, w.Text)
		case *widget.Label:
			out = append(out, w.Text)
		case *widget.Button:
			out = append(out, w.Text)
		case *fyne.Container:
			for _, c := range w.Objects {
				walk(c)
			}
		}
	}
	walk(o)
	return out
}

// TestDoctor shows found, bundled, missing and broken tools with what each is for and how to
// get it, saves a chosen path per user, and goes back to finding the tool by itself.
func TestDoctor(t *testing.T) {
	cfg := t.TempDir()
	t.Setenv("APPDATA", cfg)
	t.Setenv("XDG_CONFIG_HOME", cfg)
	t.Setenv("HOME", cfg)
	toolchain.SetOverrides(nil)
	t.Cleanup(func() { toolchain.SetOverrides(nil) })

	tool := func(id string) toolchain.Tool {
		for _, tl := range toolchain.Tools {
			if tl.ID == id {
				return tl
			}
		}
		t.Fatalf("no tool %s", id)
		return toolchain.Tool{}
	}
	chosen := filepath.Join(cfg, "my tools", "gdb.exe")
	probe := func(context.Context) []toolchain.Status {
		gdb := toolchain.Status{Tool: tool("gdb")}
		if p := toolchain.CurrentOverrides()["gdb"]; p != "" {
			gdb.Path, gdb.Source, gdb.Version = p, toolchain.SourceConfigured, "17.2"
		}
		return []toolchain.Status{
			{Tool: tool("nvim"), Path: `C:\Program Files\PyxForge\nvim\bin\nvim.exe`, Source: toolchain.SourceBundled, Version: "0.12.6"},
			{Tool: tool("qemu-x86_64")},
			gdb,
			{Tool: tool("qemu-arm")},
		}
	}
	q := make(queue, 256)
	s := NewWithOptions(test.NewTempApp(t), t.TempDir(), Options{Dispatch: q.post, Probe: probe})
	s.openDoctor()
	dc := s.doctor
	q.pumpUntil(t, "the first check", func() bool { return dc.checks == 1 })

	all := strings.Join(texts(dc.body), "\n")
	for _, want := range []string{
		"Neovim", "0.12.6", "(bundled)", "Used for the editor",
		"QEMU (x86-64)", "Install: " + tool("qemu-x86_64").Hint[runtime.GOOS], "Used for Run and Debug",
		"QEMU (ARM) (optional)", "Choose…",
	} {
		if !strings.Contains(all, want) {
			t.Errorf("Doctor does not show %q:\n%s", want, all)
		}
	}
	if !strings.Contains(dc.summary.Text, "1 of 4 tools found") || !strings.Contains(dc.summary.Text, "2 that a workflow needs are missing") {
		t.Errorf("summary %q", dc.summary.Text)
	}

	// Choosing GDB's executable saves it for this user and applies it at once.
	dc.set(tool("gdb"), chosen)
	q.pumpUntil(t, "the check after choosing", func() bool { return dc.checks == 2 })
	if toolchain.CurrentOverrides()["gdb"] != chosen {
		t.Errorf("override %v", toolchain.CurrentOverrides())
	}
	file, _ := toolchain.OverridesFile()
	if data, err := os.ReadFile(file); err != nil || !strings.Contains(string(data), "gdb") {
		t.Errorf("settings file: %s, %v", data, err)
	}
	all = strings.Join(texts(dc.body), "\n")
	if !strings.Contains(all, "(set in Doctor)") || !strings.Contains(all, "Use Default") {
		t.Errorf("the chosen path is not shown as set in Doctor:\n%s", all)
	}

	dc.set(tool("gdb"), "")
	q.pumpUntil(t, "the check after Use Default", func() bool { return dc.checks == 3 })
	if len(toolchain.CurrentOverrides()) != 0 || strings.Contains(strings.Join(texts(dc.body), "\n"), "Use Default") {
		t.Errorf("Use Default left %v", toolchain.CurrentOverrides())
	}
}
