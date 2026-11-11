package shell

import (
	"slices"

	"fyne.io/fyne/v2/container"

	"github.com/obstinix/PyxForge/internal/config"
)

// PyxForge is a code editor first. The machine tools (the QEMU, GDB and Snapshots panel tabs,
// and the register Inspector beside the editor) belong to projects that boot something in
// QEMU. A project whose pyxforge.toml has a [qemu] table starts with them; any other project
// starts with the editor, the terminal, the build output, problems and Git only. They are
// never out of reach: starting a machine, or any Show QEMU, Show GDB, Show Snapshots or
// Inspector command, brings them in.

// machineProject reports whether the open project is set up to boot in QEMU.
func (s *Shell) machineProject() bool {
	if s.root == "" {
		return false
	}
	c, err := config.Load(s.root)
	return err == nil && c.Qemu != nil
}

// showMachineTools adds the QEMU, GDB and Snapshots tabs to the panel, before Log, if they are
// not there. The selected tab stays selected.
func (s *Shell) showMachineTools() {
	if s.machineShown {
		return
	}
	s.machineShown = true
	was := s.dock.Selected()
	keep := s.dock.OnSelected
	s.dock.OnSelected = nil // moving Log to the end selects nothing the user chose
	s.dock.Remove(s.logTab)
	for _, it := range s.machineTabs {
		s.dock.Append(it)
	}
	s.dock.Append(s.logTab)
	if was != nil {
		s.dock.Select(was)
	}
	s.dock.OnSelected = keep
}

// showDockItem opens the bottom panel on a tab, bringing the machine tools in if it is one of
// theirs.
func (s *Shell) showDockItem(it *container.TabItem) {
	if slices.Contains(s.machineTabs, it) {
		s.showMachineTools()
	}
	if i := slices.Index(s.dock.Items, it); i >= 0 {
		s.showDockTab(i)
	}
}
