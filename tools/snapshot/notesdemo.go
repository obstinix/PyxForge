package main

import (
	"os"
	"path/filepath"
)

// notesDemo writes the Forge Notes demo project (identity prompt Phase 6): the boot-sector
// example and notes that link to it, in a temporary folder.
func notesDemo(repo string) string {
	dir, err := os.MkdirTemp("", "pyxforge-notes-demo")
	if err != nil {
		fail(err)
	}
	for _, name := range []string{"boot.asm", "pyxforge.toml", "README.md"} {
		if data, err := os.ReadFile(filepath.Join(repo, "examples", "boot-sector", name)); err == nil {
			_ = os.WriteFile(filepath.Join(dir, name), data, 0o644)
		}
	}
	for name, text := range map[string]string{
		"notes/boot-hang.md":      demoNote,
		"notes/serial.md":         "# Serial port\n\nCOM1 is at 0x3f8; [[boot.asm#serial_put]] waits for bit 5 of the line status register. See [[boot-hang]].\n",
		"notes/log/2026-11-27.md": "# Dev log 2026-11-27\n\n## Done\n\n- Traced the hang to [[boot.asm:28]].\n",
	} {
		p := filepath.Join(dir, filepath.FromSlash(name))
		_ = os.MkdirAll(filepath.Dir(p), 0o755)
		_ = os.WriteFile(p, []byte(text), 0o644)
	}
	return dir
}

const demoNote = "# Boot hang on real hardware\n\n" +
	"The sector prints under QEMU but stops after `cli` on the laptop. #boot #debug/serial\n\n" +
	"## What we know\n\n" +
	"- The BIOS loads us at `0x7c00`: see [[boot.asm:5]].\n" +
	"- [[boot.asm#serial_put]] spins on the line status register.\n\n" +
	"- [x] Checked the segment setup\n" +
	"- [ ] Read the BIOS table for COM1\n\n" +
	"> Real hardware may have no COM1; the loop then never ends.\n\n" +
	"## The loop\n\n" +
	"```nasm\n.wait:\n    in al, dx\n    test al, 0x20   ; transmitter empty?\n    jz .wait\n```\n\n" +
	"Next: time out after a while and fall back to the screen. Related: [[serial]].\n"
