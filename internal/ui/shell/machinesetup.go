package shell

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/widget"
	"github.com/BurntSushi/toml"
	"github.com/obstinix/PyxForge/internal/config"
	"github.com/obstinix/PyxForge/internal/toolchain"
	"github.com/obstinix/PyxForge/internal/ui/icons"
	"github.com/obstinix/PyxForge/internal/ui/kit"
	"github.com/obstinix/PyxForge/internal/ui/notifications"
)

// Machine setup (PXF-DEF-023). Run and Debug in QEMU come from the [qemu] and [gdb] tables of
// pyxforge.toml. A project without them is offered Set Up Machine…: what to boot, which QEMU
// boots it, and how GDB connects, checked as it is typed, then written into pyxforge.toml (a new
// file, or the tables added at the end of the existing one; nothing else in it changes). For a
// project that has a [qemu] table the form starts from it and changes its keys in place, which
// keeps the file's comments (#69).

// machineDraft is what Set Up Machine collects.
type machineDraft struct {
	Kernel  bool   // boot a kernel with -kernel, not a disk image
	Image   string // the file to boot, relative to the project
	QEMU    string // the QEMU system emulator
	Machine string // -machine
	Memory  string // -m
	Port    string // the GDB stub's port
	Arch    string // GDB's architecture
	GDB     string // the GDB program
}

// qemuSystems are the QEMU system emulators offered, in order.
var qemuSystems = []string{"qemu-system-i386", "qemu-system-x86_64", "qemu-system-arm", "qemu-system-aarch64", "qemu-system-riscv64"}

// defaultsFor gives the machine and GDB architecture that usually go with an emulator.
func defaultsFor(qemuName string) (machine, arch string) {
	switch base := strings.TrimSuffix(filepath.Base(qemuName), ".exe"); base {
	case "qemu-system-i386", "qemu-system-x86_64":
		return "pc", "i8086" // a PC starts in real mode, as a boot sector does
	case "qemu-system-arm":
		return "virt", "arm"
	}
	return "virt", "auto"
}

var memoryPattern = regexp.MustCompile(`^[1-9][0-9]*[KMGT]?$`)

// checkImage is the boot file's rule: given, and inside the project.
func checkImage(root, image string) error {
	image = strings.TrimSpace(image)
	if image == "" {
		return errors.New("name the disk image or kernel to boot, such as build/boot.bin")
	}
	p := image
	if !filepath.IsAbs(p) {
		p = filepath.Join(root, filepath.FromSlash(p))
	}
	rel, err := filepath.Rel(root, p)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return errors.New("the file to boot must be inside the project folder")
	}
	return nil
}

func checkPort(s string) error {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil || n < 1 || n > 65535 {
		return errors.New("a port is a number from 1 to 65535")
	}
	return nil
}

func checkMemory(s string) error {
	if !memoryPattern.MatchString(strings.TrimSpace(s)) {
		return errors.New("memory is a size such as 128M or 1G")
	}
	return nil
}

// relImage is the boot file relative to the project, with forward slashes, as pyxforge.toml
// keeps paths.
func relImage(root, image string) string {
	image = strings.TrimSpace(image)
	if filepath.IsAbs(image) {
		if rel, err := filepath.Rel(root, image); err == nil {
			image = rel
		}
	}
	return filepath.ToSlash(filepath.Clean(filepath.FromSlash(image)))
}

// machineFile returns pyxforge.toml with the draft's tables: a new file for the folder, or old
// with [qemu], [qemu.debug] and, if it has none, [gdb] added at the end; a file that has [qemu]
// already has its keys changed in place (editMachine). The result parses and validates.
func machineFile(root string, old []byte, d machineDraft) (string, error) {
	for _, err := range []error{checkImage(root, d.Image), checkMemory(d.Memory), checkPort(d.Port)} {
		if err != nil {
			return "", err
		}
	}
	if !slices.Contains(config.ValidGdbArchitectures, d.Arch) {
		return "", fmt.Errorf("GDB architecture %q is not one of %s", d.Arch, strings.Join(config.ValidGdbArchitectures, ", "))
	}
	text := string(old)
	if old == nil {
		var b bytes.Buffer
		if err := toml.NewEncoder(&b).Encode(map[string]any{"project": map[string]string{"name": filepath.Base(root)}}); err != nil {
			return "", err
		}
		text = b.String()
	}
	c, err := config.Parse(text)
	switch {
	case err != nil:
		return "", fmt.Errorf("%s does not parse, so nothing was added to it: %w", config.FileName, err)
	case c.Qemu != nil:
		return editMachine(root, text, d)
	}
	port, _ := strconv.Atoi(strings.TrimSpace(d.Port))
	q := map[string]any{
		"executable": strings.TrimSpace(d.QEMU),
		"machine":    strings.TrimSpace(d.Machine),
		"memory":     strings.TrimSpace(d.Memory),
		"debug":      map[string]any{"enabled": true, "gdb_port": port},
	}
	key := "boot_image"
	if d.Kernel {
		key = "kernel"
	}
	q[key] = relImage(root, d.Image)
	add := map[string]any{"qemu": q}
	if c.Gdb == nil {
		add["gdb"] = map[string]string{"executable": strings.TrimSpace(d.GDB), "architecture": d.Arch}
	}
	var b bytes.Buffer
	enc := toml.NewEncoder(&b)
	enc.Indent = ""
	if err := enc.Encode(add); err != nil {
		return "", err
	}
	text = strings.TrimRight(text, "\r\n") + "\n\n" + b.String()
	c, err = config.Parse(text)
	if err == nil {
		err = c.Validate()
	}
	if err != nil {
		return "", fmt.Errorf("the machine settings do not make a valid %s: %w", config.FileName, err)
	}
	return text, nil
}

// machineSetUp reports whether the project has a [qemu] table, and its configuration; problem
// is set when its pyxforge.toml cannot be read.
func machineSetUp(root string) (c *config.Config, ok bool, problem error) {
	c, err := config.Load(root)
	switch {
	case errors.Is(err, config.ErrNotFound):
		return nil, false, nil
	case err != nil:
		return nil, false, err
	}
	return c, c.Qemu != nil, nil
}

// setup offers the form, empty for a project without a [qemu] table and filled from it for one
// that has it (#69); pyxforge.toml in the editor when it does not load.
func (m *machine) setup() {
	if !m.s.requireWorkspace() {
		return
	}
	c, ok, problem := machineSetUp(m.s.root)
	file := filepath.Join(m.s.root, config.FileName)
	switch {
	case problem != nil:
		m.s.choose(config.FileName+" has a problem", problem.Error(),
			choice{label: "Open " + config.FileName, primary: true, run: func() { m.s.OpenFile(file) }})
	case ok:
		d := draftOf(c)
		m.setupFormFrom(&d)
	default:
		m.setupForm()
	}
}

// setupForm is Set Up Machine…: the boot file, QEMU and GDB, with the defaults that fit.
func (m *machine) setupForm() { m.setupFormFrom(nil) }

// setupFormFrom is the form, starting from the machine a project already has when from is set.
func (m *machine) setupFormFrom(from *machineDraft) {
	if !m.s.requireWorkspace() {
		return
	}
	s := m.s
	found := func(names ...string) (out []string) {
		for _, n := range names {
			if _, err := toolchain.LookPath(n); err == nil {
				out = append(out, n)
			}
		}
		return out
	}
	systems := found(qemuSystems...)
	if len(systems) == 0 {
		systems = slices.Clone(qemuSystems) // none installed: still choose one, and say so
	}
	gdbs := found("gdb", "gdb-multiarch")
	if len(gdbs) == 0 {
		gdbs = []string{"gdb"}
	}
	kind := widget.NewRadioGroup([]string{"Disk image", "Kernel (-kernel)"}, nil)
	kind.Horizontal = true
	kind.SetSelected("Disk image")
	image := s.newDialogEntry()
	image.SetPlaceHolder("build/boot.bin")
	image.Validator = func(v string) error { return checkImage(s.root, v) }
	browse := kit.NewIconButton(icons.FolderOpen, "Browse for the file to boot", func() {
		d := dialog.NewFileOpen(func(r fyne.URIReadCloser, err error) {
			if err != nil || r == nil {
				return
			}
			_ = r.Close()
			image.SetText(relImage(s.root, r.URI().Path()))
		}, s.win)
		if l, err := storage.ListerForURI(storage.NewFileURI(s.root)); err == nil {
			d.SetLocation(l)
		}
		s.present(d, nil)
		d.Resize(fyne.NewSize(720, 480)) // after Show: a FileDialog has no window to size before it
	})
	image.ActionItem = browse
	machineEntry := widget.NewSelectEntry([]string{"pc", "q35", "isapc", "virt"})
	arch := widget.NewSelect(config.ValidGdbArchitectures, nil)
	qemuSel := widget.NewSelectEntry(systems)
	qemuSel.OnChanged = func(v string) {
		mach, a := defaultsFor(v)
		machineEntry.SetText(mach)
		arch.SetSelected(a)
	}
	qemuSel.SetText(systems[0])
	memory := s.newDialogEntry()
	memory.SetText("128M")
	memory.Validator = checkMemory
	port := s.newDialogEntry()
	port.SetText("1234")
	port.Validator = checkPort
	gdbSel := widget.NewSelectEntry(gdbs)
	gdbSel.SetText(gdbs[0])
	note := widget.NewLabel(fmt.Sprintf("Saved as [qemu] and [gdb] tables in %s. Run in QEMU starts the machine; Debug in QEMU starts it paused with GDB attached at this port.", config.FileName))
	note.Wrapping = fyne.TextWrapWord

	items := []*widget.FormItem{
		widget.NewFormItem("Boot", kind),
		widget.NewFormItem("File", image), // the entry itself, so the form sees its validation (#26)
		widget.NewFormItem("QEMU", qemuSel),
		widget.NewFormItem("Machine", machineEntry),
		widget.NewFormItem("Memory", memory),
		widget.NewFormItem("GDB", gdbSel),
		widget.NewFormItem("Architecture", arch),
		widget.NewFormItem("GDB port", port),
		widget.NewFormItem("", note),
	}
	if from != nil { // the machine the project has: change it rather than start again
		if from.Kernel {
			kind.SetSelected("Kernel (-kernel)")
		}
		image.SetText(from.Image)
		qemuSel.SetText(from.QEMU) // sets the machine and architecture that usually go with it…
		machineEntry.SetText(from.Machine)
		if from.Arch != "" {
			arch.SetSelected(from.Arch) // …and these say what the project has
		}
		memory.SetText(from.Memory)
		port.SetText(from.Port)
		if from.GDB != "" {
			gdbSel.SetText(from.GDB)
		}
		note.SetText(fmt.Sprintf("Saved into the [qemu] and [gdb] tables of %s, changing only these settings; its comments and other tables stay as they are.", config.FileName))
	}
	d := escForm("Set Up Machine", "Save", "Cancel", items, func(ok bool) {
		if !ok {
			return
		}
		m.saveSetup(machineDraft{Kernel: kind.Selected != "Disk image", Image: image.Text, QEMU: qemuSel.Text,
			Machine: machineEntry.Text, Memory: memory.Text, Port: port.Text, Arch: arch.Selected, GDB: gdbSel.Text})
	}, s.win)
	d.Resize(fyne.NewSize(600, d.MinSize().Height))
	s.submitForm(d, image, memory, port)
	s.win.Canvas().Focus(image)
	s.lastDialog = d
}

// editMachine changes the machine keys of a pyxforge.toml that has a [qemu] table, line by
// line, so its comments and other tables stay as they were (#69). The result is parsed back and
// must say what the draft says, or nothing is written.
func editMachine(root, text string, d machineDraft) (string, error) {
	image := relImage(root, d.Image)
	key, other := "boot_image", "kernel"
	if d.Kernel {
		key, other = "kernel", "boot_image"
	}
	port := strings.TrimSpace(d.Port)
	type edit struct{ table, key, value string }
	var edits []edit
	for _, e := range []struct{ table, key, value string }{
		{"qemu", "executable", strings.TrimSpace(d.QEMU)},
		{"qemu", "machine", strings.TrimSpace(d.Machine)},
		{"qemu", "memory", strings.TrimSpace(d.Memory)},
		{"qemu", key, image},
		{"gdb", "executable", strings.TrimSpace(d.GDB)},
		{"gdb", "architecture", d.Arch},
	} {
		v, err := config.TOMLString(e.value)
		if err != nil {
			return "", err
		}
		edits = append(edits, edit{e.table, e.key, v})
	}
	edits = append(edits, edit{"qemu.debug", "gdb_port", port})
	var err error
	for _, e := range edits {
		if text, err = config.SetValue(text, e.table, e.key, e.value); err != nil {
			return "", err
		}
	}
	text = config.RemoveKey(text, "qemu", other)
	c, err := config.Parse(text)
	if err == nil {
		err = c.Validate()
	}
	if err != nil {
		return "", fmt.Errorf("the machine settings do not make a valid %s: %w", config.FileName, err)
	}
	q, g := c.Qemu, c.Gdb
	got := []string{q.Executable, q.Machine, q.Memory, q.BootImage + q.Kernel, strconv.Itoa(int(q.Debug.GdbPort)), g.Executable, g.Architecture}
	want := []string{strings.TrimSpace(d.QEMU), strings.TrimSpace(d.Machine), strings.TrimSpace(d.Memory), image, port, strings.TrimSpace(d.GDB), d.Arch}
	if !slices.Equal(got, want) {
		return "", fmt.Errorf("%s did not take the change (it says %v); change it in the file", config.FileName, got)
	}
	return text, nil
}

// draftOf is the machine a project's configuration describes, for the form to start from.
func draftOf(c *config.Config) machineDraft {
	d := machineDraft{QEMU: c.Qemu.Executable, Machine: c.Qemu.Machine, Memory: c.Qemu.Memory,
		Image: c.Qemu.BootImage, Port: strconv.Itoa(int(c.Qemu.Debug.GdbPort))}
	if c.Qemu.Kernel != "" {
		d.Kernel, d.Image = true, c.Qemu.Kernel
	}
	if c.Gdb != nil {
		d.GDB, d.Arch = c.Gdb.Executable, c.Gdb.Architecture
	}
	return d
}

// saveSetup writes the draft into pyxforge.toml and says what comes next.
func (m *machine) saveSetup(d machineDraft) {
	path := filepath.Join(m.s.root, config.FileName)
	old, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		old, err = nil, nil
	}
	var text string
	if err == nil {
		text, err = machineFile(m.s.root, old, d)
	}
	if err == nil {
		err = os.WriteFile(path, []byte(text), 0o644)
	}
	if err != nil {
		m.s.Notify(notifications.Error, "Machine not set up", err.Error())
		return
	}
	detail := fmt.Sprintf("%s boots %s. Run in QEMU: Ctrl+Shift+R; Debug in QEMU: Ctrl+Shift+D.", strings.TrimSpace(d.QEMU), relImage(m.s.root, d.Image))
	if _, err := os.Stat(filepath.Join(m.s.root, filepath.FromSlash(relImage(m.s.root, d.Image)))); err != nil {
		detail += " " + relImage(m.s.root, d.Image) + " does not exist yet: a build profile has to make it."
	}
	if _, err := toolchain.LookPath(strings.TrimSpace(d.QEMU)); err != nil {
		detail += " " + strings.TrimSpace(d.QEMU) + " is not installed or not on PATH (Tools: Doctor)."
	}
	m.s.Notify(notifications.Success, "Machine set up in "+config.FileName, detail)
	m.showSetup()
	m.s.explorer.Reload()
}
