package shell

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"github.com/obstinix/PyxForge/internal/config"
	"github.com/obstinix/PyxForge/internal/gdb"
	"github.com/obstinix/PyxForge/internal/inspect"
	"github.com/obstinix/PyxForge/internal/ui/commandpalette"
	"github.com/obstinix/PyxForge/internal/ui/icons"
	"github.com/obstinix/PyxForge/internal/ui/kit"
	"github.com/obstinix/PyxForge/internal/ui/theme"
)

// machineViews are the inspector's tabs: the machine's registers, flags and memory while a
// debug session is paused, and the boot image's bytes and code at any time.
type machineViews struct {
	s *Shell

	regs     *rowsView
	flags    *rowsView
	hex      *rowsView
	disasm   *rowsView
	memory   *rowsView
	sector   *mapView
	memEntry *widget.Entry
	memAt    uint64       // 0: follow the stack
	mode     inspect.Mode // the image listing's mode chosen with Disassemble As…; 0 follows [gdb]
	prev     map[string]uint64

	busy, again bool
}

// row is a line in an inspector view: a name, a value, and how to draw it.
type row struct {
	name, value string
	role        kit.Role // the value's colour
	mark        bool     // the current instruction
}

// rowsView is a list of rows with a message in its place when there is nothing to show.
type rowsView struct {
	body  *fyne.Container
	list  *widget.List
	rows  []row
	head  *kit.Text // a summary line above the rows
	frame fyne.CanvasObject
	empty fyne.CanvasObject
}

func newRowsView(empty fyne.CanvasObject, nameWidth float32, top fyne.CanvasObject) *rowsView {
	v := &rowsView{empty: empty}
	v.head = kit.NewText("", kit.Body, kit.Secondary)
	v.head.TextSize = theme.TextCaption + 1
	v.list = widget.NewList(func() int { return len(v.rows) },
		func() fyne.CanvasObject {
			mark := kit.NewText("", kit.Mono, kit.Accent)
			name := kit.NewText("", kit.Mono, kit.Tertiary)
			name.TextSize = theme.TextCaption + 1
			value := kit.NewText("", kit.Mono, kit.Primary)
			value.TextSize = theme.TextCaption + 1
			left := container.NewHBox(mark, container.New(&fixedWidth{w: nameWidth}, name))
			return container.New(layout.NewCustomPaddedLayout(0, 0, theme.Space1, theme.Space2), container.NewBorder(nil, nil, left, nil, value))
		},
		func(i widget.ListItemID, o fyne.CanvasObject) {
			r := v.rows[i]
			b := o.(*fyne.Container).Objects[0].(*fyne.Container)
			value := b.Objects[0].(*kit.Text)
			left := b.Objects[1].(*fyne.Container)
			mark := left.Objects[0].(*kit.Text)
			name := left.Objects[1].(*fyne.Container).Objects[0].(*kit.Text)
			if r.mark {
				mark.SetText("▶")
			} else {
				mark.SetText(" ")
			}
			name.SetText(r.name)
			value.Role = r.role
			value.SetText(r.value)
		})
	v.list.HideSeparators = true
	parts := []fyne.CanvasObject{container.New(layout.NewCustomPaddedLayout(theme.Space1, theme.Space1, theme.Space2, theme.Space2), v.head)}
	if top != nil {
		parts = append([]fyne.CanvasObject{top}, parts...)
	}
	v.frame = container.NewBorder(container.NewVBox(parts...), nil, nil, nil, v.list)
	v.body = container.NewStack(empty)
	return v
}

// set shows rows under a summary; with no rows the message returns.
func (v *rowsView) set(head string, rows []row) {
	v.rows, v.head.Text = rows, head
	v.head.Refresh()
	v.list.Refresh()
	if len(v.body.Objects) != 1 || v.body.Objects[0] != v.frame {
		v.body.Objects = []fyne.CanvasObject{v.frame}
		v.body.Refresh()
	}
}

// clear brings the message back.
func (v *rowsView) clear() {
	v.rows = nil
	if len(v.body.Objects) != 1 || v.body.Objects[0] != v.empty {
		v.body.Objects = []fyne.CanvasObject{v.empty}
		v.body.Refresh()
	}
}

// fixedWidth gives its one object a fixed width, so register names line up.
type fixedWidth struct{ w float32 }

func (l *fixedWidth) Layout(objs []fyne.CanvasObject, s fyne.Size) {
	objs[0].Resize(fyne.NewSize(l.w, s.Height))
}

func (l *fixedWidth) MinSize(objs []fyne.CanvasObject) fyne.Size {
	return fyne.NewSize(l.w, objs[0].MinSize().Height)
}

func (s *Shell) buildMachineViews() []*container.TabItem {
	v := &machineViews{s: s, prev: map[string]uint64{}}
	s.mviews = v
	noSession := func(icon icons.Name, title string) fyne.CanvasObject {
		return placeholderAction(icon, title, "Debug in QEMU (Ctrl+Shift+D) stops at the program's first instruction; this view then follows every stop.",
			"Debug in QEMU", func() { s.mach.start(true) })
	}
	noImage := func(icon icons.Name) fyne.CanvasObject {
		return placeholder(icon, "No image to show", "Build the project; the image that [qemu] boots appears here.")
	}
	v.regs = newRowsView(noSession(icons.CPU, "No debug session"), 56, nil)
	v.flags = newRowsView(noSession(icons.Flag, "No debug session"), 40, nil)
	v.hex = newRowsView(noImage(icons.Binary), 0, nil)
	v.disasm = newRowsView(noImage(icons.List), 64, nil)
	v.sector = newMapView(v.seekHex)
	v.memEntry = widget.NewEntry()
	v.memEntry.SetPlaceHolder("Address, such as 0x7c00 (empty: the stack)")
	v.memEntry.OnSubmitted = v.setMemoryAddress
	v.memory = newRowsView(noSession(icons.MemoryStick, "No debug session"), 0, container.NewPadded(v.memEntry))
	return []*container.TabItem{
		container.NewTabItem("Registers", v.regs.body),
		container.NewTabItem("Flags", v.flags.body),
		container.NewTabItem("Hex", v.hex.body),
		container.NewTabItem("Map", v.sector.body),
		container.NewTabItem("Disasm", v.disasm.body),
		container.NewTabItem("Memory", v.memory.body),
	}
}

// seekHex shows the Hex tab at the line holding offset, marked.
func (v *machineViews) seekHex(offset int) {
	// Select the tab first: selecting it reloads the image, which rebuilds the rows.
	for _, it := range v.s.inspectorTabs.Items {
		if it.Text == "Hex" {
			v.s.inspectorTabs.Select(it)
		}
	}
	line := offset / 16
	if line >= len(v.hex.rows) {
		return
	}
	for i := range v.hex.rows {
		v.hex.rows[i].mark = i == line
	}
	v.hex.list.Refresh()
	v.hex.list.ScrollTo(line)
}

func (v *machineViews) setMemoryAddress(text string) {
	text = strings.TrimSpace(text)
	v.memAt = 0
	if text != "" {
		a, err := strconv.ParseUint(strings.TrimPrefix(strings.ToLower(text), "0x"), 16, 64)
		if err != nil {
			v.memory.head.SetText("Not a hexadecimal address: " + text)
			return
		}
		v.memAt = a
	}
	if v.s.mach.paused && v.s.mach.dbg != nil {
		v.refresh()
	}
}

// running is called when the machine runs again or stops: values read while paused go stale,
// and with no machine the session views clear.
func (v *machineViews) running() {
	if v.s.mach.inst != nil {
		if len(v.regs.rows) > 0 {
			v.regs.head.SetText("Running: values are from the last stop")
		}
		return
	}
	v.regs.clear()
	v.flags.clear()
	v.memory.clear()
	v.prev = map[string]uint64{}
	v.loadImage()
}

// machineRead is what one stop reads from GDB.
type machineRead struct {
	regs   map[string]uint64
	order  []string
	code   []byte
	start  uint64
	mem    []byte
	memAt  uint64
	memWhy string
	err    error
}

// refresh reads the stopped machine in the background and redraws the views.
func (v *machineViews) refresh() {
	m := v.s.mach
	if m.dbg == nil {
		return
	}
	if v.busy {
		v.again = true
		return
	}
	v.busy = true
	d, mode, pc, memAt := m.dbg, m.mode, m.pc, v.memAt
	go func() {
		snap := readSnapshot(d, mode, pc, memAt)
		v.s.dispatch(func() {
			v.busy = false
			v.apply(snap, mode, pc)
			if v.again {
				v.again = false
				v.refresh()
			}
		})
	}()
}

func readSnapshot(d *gdb.Session, mode inspect.Mode, pc, memAt uint64) machineRead {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	snap := machineRead{regs: map[string]uint64{}}
	regs, err := d.Registers(ctx)
	if err != nil {
		snap.err = err
		return snap
	}
	for _, r := range regs {
		if n, err := strconv.ParseUint(strings.TrimPrefix(r.Value, "0x"), 16, 64); err == nil {
			snap.regs[r.Name] = n
			snap.order = append(snap.order, r.Name)
		}
	}
	lin := linearPC(snap.regs, mode, pc)
	snap.start, snap.code = lin, nil
	n := 96
	if mode == 16 && lin >= 0x7c00 && lin < 0x7e00 {
		snap.start, n = 0x7c00, 512 // the whole boot sector, decoded from its start
	}
	snap.code, _ = d.ReadMemory(ctx, snap.start, n)
	snap.memAt, snap.memWhy = memAt, ""
	if memAt == 0 {
		snap.memAt, snap.memWhy = stackAddr(snap.regs, mode), "the stack"
	}
	snap.mem, _ = d.ReadMemory(ctx, snap.memAt, 128)
	return snap
}

// reg reads a register by its widest name, so x86-64 and i386 targets both work.
func reg(regs map[string]uint64, names ...string) (uint64, bool) {
	for _, n := range names {
		if v, ok := regs[n]; ok {
			return v, true
		}
	}
	return 0, false
}

// linearPC is the address of the next instruction: CS*16+IP in real mode.
func linearPC(regs map[string]uint64, mode inspect.Mode, pc uint64) uint64 {
	if mode != 16 {
		return pc
	}
	ip, _ := reg(regs, "rip", "eip", "pc")
	cs, _ := reg(regs, "cs")
	return cs<<4 + ip&0xffff
}

func stackAddr(regs map[string]uint64, mode inspect.Mode) uint64 {
	sp, _ := reg(regs, "rsp", "esp", "sp")
	if mode == 16 {
		ss, _ := reg(regs, "ss")
		return ss<<4 + sp&0xffff
	}
	return sp
}

// registerSets are the registers each mode shows, as label and the GDB names to read it from.
var registerSets = map[inspect.Mode][][2]string{
	16: {{"ax", "rax eax"}, {"bx", "rbx ebx"}, {"cx", "rcx ecx"}, {"dx", "rdx edx"}, {"si", "rsi esi"}, {"di", "rdi edi"},
		{"bp", "rbp ebp"}, {"sp", "rsp esp"}, {"ip", "rip eip"}, {"flags", "eflags"}, {"cs", "cs"}, {"ds", "ds"},
		{"es", "es"}, {"ss", "ss"}, {"fs", "fs"}, {"gs", "gs"}},
	32: {{"eax", "rax eax"}, {"ebx", "rbx ebx"}, {"ecx", "rcx ecx"}, {"edx", "rdx edx"}, {"esi", "rsi esi"},
		{"edi", "rdi edi"}, {"ebp", "rbp ebp"}, {"esp", "rsp esp"}, {"eip", "rip eip"}, {"eflags", "eflags"},
		{"cs", "cs"}, {"ds", "ds"}, {"es", "es"}, {"ss", "ss"}, {"fs", "fs"}, {"gs", "gs"}, {"cr0", "cr0"}},
	64: {{"rax", "rax"}, {"rbx", "rbx"}, {"rcx", "rcx"}, {"rdx", "rdx"}, {"rsi", "rsi"}, {"rdi", "rdi"}, {"rbp", "rbp"},
		{"rsp", "rsp"}, {"r8", "r8"}, {"r9", "r9"}, {"r10", "r10"}, {"r11", "r11"}, {"r12", "r12"}, {"r13", "r13"},
		{"r14", "r14"}, {"r15", "r15"}, {"rip", "rip"}, {"eflags", "eflags"}, {"cs", "cs"}, {"ss", "ss"},
		{"cr0", "cr0"}, {"cr3", "cr3"}, {"cr4", "cr4"}, {"efer", "efer"}},
}

func (v *machineViews) apply(snap machineRead, mode inspect.Mode, pc uint64) {
	if snap.err != nil {
		v.regs.set("Registers could not be read: "+snap.err.Error(), nil)
		return
	}
	width := map[inspect.Mode]int{16: 4, 32: 8, 64: 16}[mode]
	mask := uint64(1)<<(uint(width)*4) - 1
	if width == 16 {
		mask = ^uint64(0)
	}
	var regRows []row
	for _, rs := range registerSets[mode] {
		val, ok := reg(snap.regs, strings.Fields(rs[1])...)
		if !ok {
			continue
		}
		w := width
		if rs[0] == "cs" || rs[0] == "ds" || rs[0] == "es" || rs[0] == "ss" || rs[0] == "fs" || rs[0] == "gs" {
			w = 4
		}
		val &= mask
		r := row{name: rs[0], value: fmt.Sprintf("%0*x", w, val), role: kit.Primary}
		if old, seen := v.prev[rs[0]]; seen && old != val {
			r.role = kit.Warning // changed since the last stop
		}
		v.prev[rs[0]] = val
		regRows = append(regRows, r)
	}
	lin := linearPC(snap.regs, mode, pc)
	head := fmt.Sprintf("%s mode · next instruction 0x%x", modeName(mode), lin)
	v.regs.set(head, regRows)

	flags, _ := reg(snap.regs, "eflags")
	var flagRows []row
	for _, f := range eflagBits {
		set := flags>>f.bit&1 == 1
		r := row{name: f.name, value: "0  " + f.what, role: kit.Tertiary}
		if set {
			r.value, r.role = "1  "+f.what, kit.Primary
		}
		flagRows = append(flagRows, r)
	}
	flagRows = append(flagRows, row{name: "IOPL", value: strconv.FormatUint(flags>>12&3, 10) + "  I/O privilege level", role: kit.Secondary})
	v.flags.set(fmt.Sprintf("EFLAGS %08x", flags), flagRows)

	var codeRows []row
	for _, in := range inspect.Disassemble(snap.code, snap.start, mode, true) {
		codeRows = append(codeRows, row{name: fmt.Sprintf("%08x", in.Addr), value: in.Text, role: kit.Secondary, mark: in.Addr == lin})
		if in.Addr == lin {
			codeRows[len(codeRows)-1].role = kit.Primary
		}
	}
	v.disasm.set(fmt.Sprintf("Live code from 0x%x, %s mode", snap.start, modeName(mode)), codeRows)

	what := snap.memWhy
	if what == "" {
		what = fmt.Sprintf("0x%x", snap.memAt)
	}
	var memRows []row
	for _, l := range inspect.Dump(snap.mem, snap.memAt) {
		s := l.String()
		memRows = append(memRows, row{value: s, role: kit.Secondary})
	}
	v.memory.set("Memory at "+what, memRows)
}

func modeName(m inspect.Mode) string {
	switch m {
	case 32:
		return "Protected"
	case 64:
		return "Long"
	}
	return "Real"
}

var eflagBits = []struct {
	name string
	bit  uint
	what string
}{
	{"CF", 0, "carry"}, {"PF", 2, "parity"}, {"AF", 4, "adjust"}, {"ZF", 6, "zero"}, {"SF", 7, "sign"},
	{"TF", 8, "trap (single step)"}, {"IF", 9, "interrupts enabled"}, {"DF", 10, "direction"}, {"OF", 11, "overflow"},
}

// loadImage shows the image [qemu] boots: hex and boot-sector checks for a raw image, the
// header, sections and symbols for an ELF kernel, and its code in the Disasm tab when no
// session is paused.
func (v *machineViews) loadImage() {
	c, err := config.Load(v.s.root)
	if err != nil || c.Qemu == nil {
		v.hex.clear()
		v.sector.clear()
		if !v.s.mach.paused {
			v.disasm.clear()
		}
		return
	}
	rel := c.Qemu.BootImage
	if rel == "" {
		rel = c.Qemu.Kernel
	}
	data, err := os.ReadFile(filepath.Join(v.s.root, filepath.FromSlash(rel)))
	if err != nil {
		v.hex.clear()
		v.sector.clear()
		if !v.s.mach.paused {
			v.disasm.clear()
		}
		return
	}
	mode := inspect.ModeOf(c.GdbFor("").Architecture)
	if v.mode != 0 {
		mode = v.mode // Disassemble As… chose one
	}
	live := v.s.mach.paused && v.s.mach.dbg != nil
	if f := inspect.Format(data); f == "pe" || f == "mach-o" {
		v.sector.clear()
		v.hex.set(fmt.Sprintf("%s · a %s executable, not a boot image; its first bytes", rel, strings.ToUpper(f)), dumpRows(data[:min(len(data), 512)]))
		if !live {
			v.disasm.clear()
		}
		return
	}
	if strings.HasPrefix(string(data), "\x7fELF") {
		v.sector.clear() // an ELF kernel has no boot sector of its own
		v.showELF(rel, data, live)
		return
	}
	v.sector.set(rel, data)
	b := inspect.BootSector(data)
	head := fmt.Sprintf("%s · %d bytes", rel, b.Size)
	switch {
	case b.Size < 512:
		head += " · shorter than a sector: not bootable"
	case b.Signature:
		head += fmt.Sprintf(" · signature 55 aa · %d of 510 bytes used, %d free", b.Used, b.Free)
	default:
		head += " · no 55 aa signature at 510: not bootable"
	}
	const shown = 64 * 1024
	if len(data) > shown {
		head += " · first 64 KiB shown"
	}
	rows := dumpRows(data[:min(len(data), shown)])
	if b.Size >= 512 {
		rows[0x1f0/16].role = kit.Primary // the line with the signature
	}
	v.hex.set(head, rows)
	if !live {
		var code []row
		for _, in := range inspect.Disassemble(data[:min(len(data), max(b.Used, 1))], 0x7c00, mode, true) {
			code = append(code, row{name: fmt.Sprintf("%08x", in.Addr), value: in.Text, role: kit.Secondary})
		}
		v.disasm.set(fmt.Sprintf("%s at 0x7c00, %s mode (not running)", rel, modeName(mode)), code)
	}
}

func dumpRows(data []byte) []row {
	var rows []row
	for _, l := range inspect.Dump(data, 0) {
		rows = append(rows, row{value: l.String(), role: kit.Secondary})
	}
	return rows
}

// disassembleAs offers the x86 modes for the image's listing; the choice holds until another.
func (v *machineViews) disassembleAs() {
	items := []commandpalette.Item{{Title: "As configured", Detail: "the [gdb] architecture in pyxforge.toml", Icon: icons.List,
		Run: func() { v.mode = 0; v.loadImage() }}}
	for _, m := range []inspect.Mode{16, 32, 64} {
		items = append(items, commandpalette.Item{Title: modeName(m) + " mode", Detail: fmt.Sprintf("%d-bit", m), Icon: icons.List,
			Run: func() {
				v.mode = m
				v.loadImage()
				for _, it := range v.s.inspectorTabs.Items {
					if it.Text == "Disasm" {
						v.s.inspectorTabs.Select(it)
					}
				}
			}})
	}
	v.s.palette.Show("Disassemble the image as", items)
}

func (v *machineViews) showELF(rel string, data []byte, live bool) {
	e, err := inspect.ReadELF(strings.NewReader(string(data)))
	if err != nil {
		v.hex.set(rel+": "+err.Error(), nil)
		return
	}
	var rows []row
	add := func(name, value string) { rows = append(rows, row{name: name, value: value, role: kit.Secondary}) }
	for _, s := range e.Sections {
		add("", fmt.Sprintf("%-14s %08x %7d  %s", s.Name, s.Addr, s.Size, s.Flags))
	}
	for _, s := range e.Symbols {
		add("", fmt.Sprintf("%08x  %-6s %s", s.Addr, s.Kind, s.Name))
	}
	head := fmt.Sprintf("%s · %s %s · entry 0x%x", rel, e.Class, e.Machine, e.Entry)
	mode := e.Mode
	if mb := inspect.Multiboot(data); mb != 0 {
		head += fmt.Sprintf(" · Multiboot %d", mb)
		if mode == 64 && v.mode == 0 {
			mode = 32 // a Multiboot loader enters the kernel in protected mode
		}
	}
	if v.mode != 0 {
		mode = v.mode // Disassemble As… chose one
	}
	v.hex.set(head, rows)
	if !live && mode != 0 {
		if code, start, err := inspect.CodeAt(strings.NewReader(string(data)), e.Entry); err == nil {
			off := e.Entry - start
			var crow []row
			for _, in := range inspect.Disassemble(code[off:min(off+128, uint64(len(code)))], e.Entry, mode, true) {
				crow = append(crow, row{name: fmt.Sprintf("%08x", in.Addr), value: in.Text, role: kit.Secondary})
			}
			v.disasm.set(fmt.Sprintf("%s from its entry, %s mode (not running)", rel, modeName(mode)), crow)
		}
	}
}
