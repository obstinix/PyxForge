package shell

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"github.com/obstinix/PyxForge/internal/gdb"
	"github.com/obstinix/PyxForge/internal/inspect"
	"github.com/obstinix/PyxForge/internal/qemu"
	"github.com/obstinix/PyxForge/internal/snapshot"
	"github.com/obstinix/PyxForge/internal/ui/commandpalette"
	"github.com/obstinix/PyxForge/internal/ui/icons"
	"github.com/obstinix/PyxForge/internal/ui/kit"
	"github.com/obstinix/PyxForge/internal/ui/notifications"
	"github.com/obstinix/PyxForge/internal/ui/theme"
)

// snapshotPanel is the Snapshots tab. It keeps two kinds apart: diagnostic snapshots record
// registers and memory of a paused machine for comparison and cannot restore anything;
// machine snapshots are QEMU's own saved states, which restore the machine exactly.
type snapshotPanel struct {
	s       *Shell
	tab     *container.TabItem
	store   *snapshot.Store
	diag    []snapshot.Snapshot
	machine []qemu.MachineSnapshot
	diagBox *fyne.Container
	machBox *fyne.Container
	out     *outputView
	busy    bool
}

func (s *Shell) buildSnapshots() *container.TabItem {
	p := &snapshotPanel{s: s}
	s.snaps = p
	if st, err := snapshot.DefaultStore(s.root); err == nil {
		p.store = &st
	}
	p.out = newOutputView(s, 2000)
	capture := kit.NewIconButton(icons.Camera, "Capture Snapshot (registers and memory)", p.capture)
	compare := kit.NewIconButton(icons.Compare, "Compare Snapshots", p.chooseCompare)
	save := kit.NewIconButton(icons.Save, "Save Machine State (QEMU)", p.saveMachine)
	refresh := kit.NewIconButton(icons.Refresh, "Refresh", p.refresh)
	title := kit.NewText("Diagnostic snapshots record a paused machine; machine states restore one", kit.Body, kit.Secondary)
	title.TextSize = theme.TextCaption + 1
	bar := kit.Row(theme.TabBarHeight, container.NewHBox(title, layout.NewSpacer(), capture, compare, save, refresh))
	top := container.NewVBox(container.New(layout.NewCustomPaddedLayout(0, 0, theme.Space2, theme.Space1), bar), kit.NewRule(false))
	p.diagBox = container.NewVBox()
	p.machBox = container.NewVBox()
	lists := container.NewVScroll(container.New(layout.NewCustomPaddedLayout(theme.Space1, theme.Space1, theme.Space2, theme.Space2),
		container.NewVBox(p.diagBox, p.machBox)))
	split := container.NewHSplit(lists, p.out.Widget())
	split.Offset = 0.55
	p.tab = container.NewTabItem("Snapshots", container.NewBorder(top, nil, nil, nil, split))
	p.show()
	return p.tab
}

// refresh lists both kinds again.
func (p *snapshotPanel) refresh() {
	if p.busy {
		return
	}
	p.busy = true
	store, inst := p.store, p.s.mach.inst
	go func() {
		var diag []snapshot.Snapshot
		var mach []qemu.MachineSnapshot
		var errs []string
		if store != nil {
			var err error
			if diag, err = store.List(); err != nil {
				errs = append(errs, err.Error())
			}
		}
		if inst != nil && inst.QMP != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			list, err := inst.MachineSnapshots(ctx)
			cancel()
			if err == nil {
				mach = list
			}
		}
		p.s.dispatch(func() {
			p.busy = false
			if p.s.mach.inst != inst {
				mach = nil // that QEMU is gone; its states with it
			}
			p.diag, p.machine = diag, mach
			for _, e := range errs {
				p.out.AddNow("Snapshots could not be listed: " + e)
			}
			p.show()
		})
	}()
}

// show lays out both lists with each entry's actions.
func (p *snapshotPanel) show() {
	p.diagBox.RemoveAll()
	p.diagBox.Add(container.NewPadded(kit.Title(fmt.Sprintf("Diagnostic snapshots (%d)", len(p.diag)))))
	if len(p.diag) == 0 {
		p.diagBox.Add(hint("None yet. Capture records registers, flags, code, stack and memory while the machine is paused."))
	}
	for _, sn := range p.diag {
		label := widget.NewLabel(fmt.Sprintf("%s · %s · pc %x", sn.Name, sn.Taken.Local().Format("Jan 2 15:04:05"), sn.PC))
		label.Truncation = fyne.TextTruncateEllipsis
		cmp := lowButton("Compare", func() { p.compareFrom(sn) })
		del := lowButton("Delete", func() { p.deleteDiag(sn) })
		p.diagBox.Add(container.NewBorder(nil, nil, nil, container.NewHBox(cmp, del), label))
	}
	if p.s.mach.inst == nil {
		p.machine = nil
	}
	p.machBox.RemoveAll()
	p.machBox.Add(container.NewPadded(kit.Title(fmt.Sprintf("Machine states (%d)", len(p.machine)))))
	switch {
	case p.s.mach.inst == nil:
		p.machBox.Add(hint("QEMU is not running. Machine states are listed while it runs; set snapshots = true under [qemu] to save them."))
	case len(p.machine) == 0:
		p.machBox.Add(hint(fmt.Sprintf("None saved. Save Machine State keeps up to %d, each holding the guest's RAM.", qemu.MaxMachineSnapshots)))
	}
	for _, ms := range p.machine {
		label := widget.NewLabel(fmt.Sprintf("%s · %s · %s", ms.Tag, ms.Date, ms.VMSize))
		label.Truncation = fyne.TextTruncateEllipsis
		restore := lowButton("Restore", func() { p.restoreMachine(ms.Tag) })
		del := lowButton("Delete", func() { p.deleteMachine(ms.Tag) })
		p.machBox.Add(container.NewBorder(nil, nil, nil, container.NewHBox(restore, del), label))
	}
}

func hint(text string) fyne.CanvasObject {
	l := widget.NewLabel(text)
	l.Wrapping = fyne.TextWrapWord
	l.Importance = widget.LowImportance
	return l
}

func lowButton(label string, run func()) *widget.Button {
	b := widget.NewButton(label, run)
	b.Importance = widget.LowImportance
	return b
}

// readNow captures the paused machine without saving it.
func (p *snapshotPanel) readNow(done func(*snapshot.Snapshot, error)) {
	m := p.s.mach
	if m.dbg == nil || !m.paused {
		done(nil, fmt.Errorf("the machine is not paused under GDB: Debug in QEMU (Ctrl+Shift+D) stops it"))
		return
	}
	d, mode, pc, memAt, root, cfg := m.dbg, m.mode, m.pc, p.s.mviews.memAt, p.s.root, m.cfg
	reason := m.state.Text
	go func() {
		snap := readSnapshot(d, mode, pc, memAt)
		if snap.err != nil {
			p.s.dispatch(func() { done(nil, snap.err) })
			return
		}
		sn := &snapshot.Snapshot{Taken: time.Now(), Workspace: root, Mode: int(mode), PC: linearPC(snap.regs, mode, pc),
			Reason: reason, Registers: snap.regs}
		if cfg != nil && cfg.Qemu != nil {
			sn.Image = cfg.Qemu.BootImage
			if sn.Image == "" {
				sn.Image = cfg.Qemu.Kernel
			}
			sn.ImageHash = snapshot.HashFile(filepath.Join(root, filepath.FromSlash(sn.Image)))
		}
		sn.Memory = append(sn.Memory, snapshot.Memory{Label: "code", Addr: snap.start, Bytes: snap.code})
		label := "stack"
		if snap.memWhy == "" {
			label = fmt.Sprintf("memory at 0x%x", snap.memAt)
		}
		sn.Memory = append(sn.Memory, snapshot.Memory{Label: label, Addr: snap.memAt, Bytes: snap.mem})
		for _, in := range inspect.Disassemble(snap.code, snap.start, mode, true) {
			sn.Listing = append(sn.Listing, fmt.Sprintf("%08x  %s", in.Addr, in.Text))
		}
		p.s.dispatch(func() { done(sn, nil) })
	}()
}

// capture asks for a name and saves a diagnostic snapshot of the paused machine.
func (p *snapshotPanel) capture() {
	m := p.s.mach
	if m.dbg == nil || !m.paused {
		p.s.Notify(notifications.Info, "Nothing to capture", "Capture records a machine paused under GDB: Debug in QEMU (Ctrl+Shift+D) stops one.")
		return
	}
	if p.store == nil {
		p.s.Notify(notifications.Error, "No place for snapshots", "The user configuration folder is not available.")
		return
	}
	entry := widget.NewEntry()
	entry.SetText(fmt.Sprintf("at %x", m.pc))
	d := dialog.NewForm("Capture Snapshot", "Capture", "Cancel", []*widget.FormItem{widget.NewFormItem("Name", entry)},
		func(ok bool) {
			if ok {
				p.captureNamed(entry.Text)
			}
		}, p.s.win)
	d.Resize(fyne.NewSize(380, d.MinSize().Height))
	d.Show()
	p.s.win.Canvas().Focus(entry)
	p.s.lastDialog = d
}

func (p *snapshotPanel) captureNamed(name string) {
	p.readNow(func(sn *snapshot.Snapshot, err error) {
		if err != nil {
			p.s.Notify(notifications.Error, "Not captured", err.Error())
			return
		}
		sn.Name = name
		removed, err := p.store.Save(sn)
		if err != nil {
			p.s.Notify(notifications.Error, "Not captured", err.Error())
			return
		}
		p.out.AddNow(fmt.Sprintf("==> Captured %q at %x", sn.Name, sn.PC))
		if removed > 0 {
			p.out.AddNow(fmt.Sprintf("The %d oldest snapshots were removed: %d are kept.", removed, snapshot.MaxKept))
		}
		p.refresh()
	})
}

// chooseCompare picks two snapshots to compare, the machine now included when it is paused.
func (p *snapshotPanel) chooseCompare() {
	if len(p.diag) == 0 {
		p.s.Notify(notifications.Info, "No snapshots", "Capture one while the machine is paused, then compare.")
		return
	}
	p.pick("Compare from", false, func(a *snapshot.Snapshot) { p.compareFrom(*a) })
}

func (p *snapshotPanel) compareFrom(a snapshot.Snapshot) {
	p.pick("Compare "+a.Name+" with", true, func(b *snapshot.Snapshot) { p.showCompare(&a, b) })
}

// pick lists the snapshots in the palette; with now, the paused machine comes first.
func (p *snapshotPanel) pick(title string, now bool, run func(*snapshot.Snapshot)) {
	var items []commandpalette.Item
	if now && p.s.mach.dbg != nil && p.s.mach.paused {
		items = append(items, commandpalette.Item{Title: "The machine now", Detail: fmt.Sprintf("paused at %x", p.s.mach.pc), Icon: icons.CPU,
			Run: func() {
				p.readNow(func(sn *snapshot.Snapshot, err error) {
					if err != nil {
						p.s.Notify(notifications.Error, "Cannot read the machine", err.Error())
						return
					}
					sn.Name, sn.Version = "now", snapshot.Version
					run(sn)
				})
			}})
	}
	for _, sn := range p.diag {
		items = append(items, commandpalette.Item{Title: sn.Name, Detail: sn.Taken.Local().Format("Jan 2 15:04:05"), Icon: icons.Camera,
			Run: func() { run(&sn) }})
	}
	p.s.palette.Show(title, items)
}

// showCompare writes what changed from a to b into the panel's output.
func (p *snapshotPanel) showCompare(a, b *snapshot.Snapshot) {
	d := snapshot.Compare(a, b)
	p.out.AddNow(fmt.Sprintf("==> %s (pc %x) → %s (pc %x)", a.Name, a.PC, b.Name, b.PC))
	for _, r := range d.Registers {
		p.out.AddNow(fmt.Sprintf("  %-8s %x → %x", r.Name, r.Old, r.New))
	}
	for _, m := range d.Memory {
		for _, c := range m.Changes {
			p.out.AddNow(fmt.Sprintf("  %s %08x: % x → % x", m.Label, c.Addr, c.Old, c.New))
		}
	}
	for _, n := range d.Notes {
		p.out.AddNow("  note: " + n)
	}
	if d.Same() {
		p.out.AddNow("  No registers or memory changed.")
	}
	p.s.showDockTab(slices.Index(p.s.dock.Items, p.tab))
}

func (p *snapshotPanel) deleteDiag(sn snapshot.Snapshot) {
	p.s.choose("Delete "+sn.Name+"?", "The snapshot's record is deleted.", choice{label: "Delete", primary: true, run: func() {
		if err := p.store.Delete(sn.ID); err != nil {
			p.s.Notify(notifications.Error, "Not deleted", err.Error())
		}
		p.refresh()
	}})
}

// machineOp runs a machine-state operation on the running QEMU, then lists again.
func (p *snapshotPanel) machineOp(what string, op func(ctx context.Context, inst *qemu.Instance) error, after func()) {
	inst := p.s.mach.inst
	if inst == nil {
		p.s.Notify(notifications.Info, "QEMU is not running", "Machine states are saved and restored in a running QEMU.")
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		err := op(ctx, inst)
		cancel()
		p.s.dispatch(func() {
			if err != nil {
				p.out.AddNow(what + " failed: " + err.Error())
				p.s.Notify(notifications.Error, what+" failed", err.Error())
			} else if after != nil {
				after()
			}
			p.refresh()
		})
	}()
}

// saveMachine asks for a name and saves the machine's state in QEMU.
func (p *snapshotPanel) saveMachine() {
	entry := widget.NewEntry()
	entry.SetPlaceHolder("before-int13")
	d := dialog.NewForm("Save Machine State", "Save", "Cancel", []*widget.FormItem{widget.NewFormItem("Name", entry)},
		func(ok bool) {
			if ok {
				p.saveMachineNamed(entry.Text)
			}
		}, p.s.win)
	d.Resize(fyne.NewSize(380, d.MinSize().Height))
	d.Show()
	p.s.win.Canvas().Focus(entry)
	p.s.lastDialog = d
}

func (p *snapshotPanel) saveMachineNamed(name string) {
	p.machineOp("Save machine state", func(ctx context.Context, inst *qemu.Instance) error { return inst.SaveMachine(ctx, name) },
		func() { p.out.AddNow("==> Saved machine state " + name) })
}

// restoreMachine loads a machine state. Under GDB, GDB's cached registers are dropped first
// so the views show the restored machine.
func (p *snapshotPanel) restoreMachine(name string) {
	d := p.s.mach.dbg
	p.machineOp("Restore machine state", func(ctx context.Context, inst *qemu.Instance) error {
		if err := inst.LoadMachine(ctx, name); err != nil {
			return err
		}
		if d != nil {
			_, _ = d.Console(ctx, "maint flush register-cache")
		}
		return nil
	}, func() {
		p.out.AddNow("==> Restored machine state " + name)
		if p.s.mach.dbg != nil && p.s.mach.paused {
			p.refreshPC(d)
		}
	})
}

// refreshPC reads the restored program counter, then redraws the machine views.
func (p *snapshotPanel) refreshPC(d *gdb.Session) {
	m := p.s.mach
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		frames, err := d.Frames(ctx)
		p.s.dispatch(func() {
			if err == nil && len(frames) > 0 && m.dbg == d {
				m.pc = frames[0].Addr
				m.setState(fmt.Sprintf("Paused at 0x%x · restored machine state", m.pc))
			}
			p.s.mviews.refresh()
		})
	}()
}

func (p *snapshotPanel) deleteMachine(name string) {
	p.s.choose("Delete machine state "+name+"?", "QEMU deletes the saved state from the overlay.", choice{label: "Delete", primary: true, run: func() {
		p.machineOp("Delete machine state", func(ctx context.Context, inst *qemu.Instance) error { return inst.DeleteMachine(ctx, name) }, nil)
	}})
}

// chooseMachine lists the machine states in the palette for an action.
func (p *snapshotPanel) chooseMachine(title string, run func(string)) {
	if len(p.machine) == 0 {
		p.s.Notify(notifications.Info, "No machine states", "Save Machine State saves one while QEMU runs.")
		return
	}
	var items []commandpalette.Item
	for _, ms := range p.machine {
		items = append(items, commandpalette.Item{Title: ms.Tag, Detail: ms.Date + " · " + ms.VMSize, Icon: icons.Save, Run: func() { run(ms.Tag) }})
	}
	p.s.palette.Show(title, items)
}
