package shell

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"github.com/obstinix/PyxForge/internal/buildinfo"
	"github.com/obstinix/PyxForge/internal/managed"
	"github.com/obstinix/PyxForge/internal/toolchain"
	"github.com/obstinix/PyxForge/internal/ui/icons"
	"github.com/obstinix/PyxForge/internal/ui/kit"
	"github.com/obstinix/PyxForge/internal/ui/theme"
)

// The setup wizard (Help > Setup Wizard…, and once on the first start) walks someone new
// through what PyxForge needs on this computer: what they will build, which of its tools are
// here, installing the missing ones PyxForge manages (internal/managed: pinned, checked
// downloads into PyxForge's own folder), and a first project. Every step acts on the real
// state: the tool check, the Dependency Manager's installs, the New Project presets. Skipping
// it leaves everything working; nothing is installed unless asked.

const (
	prefSetupDone    = "setup.wizardDone"
	prefSetupProfile = "setup.profile"
)

// devProfile is a kind of work, and the tools it uses (toolchain IDs).
type devProfile struct {
	id, name, about string
	tools           []string
}

var devProfiles = []devProfile{
	{"systems", "Systems and OS development", "C, C++ and assembly; boot sectors and kernels in QEMU, debugged with GDB",
		[]string{"cc", "cxx", "ld", "nasm", "make", "gdb", "objdump", "qemu-x86_64", "qemu-img", "git", "clangd", "asm-lsp"}},
	{"rust", "Rust", "Cargo projects, debugged with GDB",
		[]string{"cargo", "rustc", "gdb", "git", "rust-analyzer"}},
	{"go", "Go", "Go modules, debugged with Delve",
		[]string{"go", "git", "gopls"}},
	{"python", "Python", "Scripts and packages, debugged with pdb",
		[]string{"python", "git", "pyright"}},
	{"java", "Java", "Java programs, debugged with jdb",
		[]string{"java", "git", "jdtls"}},
	{"web", "JavaScript and TypeScript", "Node.js programs, debugged with node inspect",
		[]string{"node", "git", "typescript-language-server"}},
	{"general", "General", "Editing, the terminal and Git",
		[]string{"git"}},
}

func profileByID(id string) devProfile {
	for _, p := range devProfiles {
		if p.id == id {
			return p
		}
	}
	return devProfiles[0]
}

type setupWizard struct {
	s       *Shell
	d       dialog.Dialog
	step    int
	title   *kit.Text
	count   *widget.Label
	body    *fyne.Container
	back    *kit.Button
	next    *kit.Button
	profile devProfile
	picks   map[string]bool // managed packages chosen to install, by ID
	touched bool            // the user changed the picks: profile changes no longer reset them
}

var setupSteps = []string{"Welcome", "What you will build", "Your tools", "Install", "First project"}

// openSetupWizard shows the wizard, from its first step.
func (s *Shell) openSetupWizard() {
	if s.setup != nil {
		return
	}
	w := &setupWizard{s: s, picks: map[string]bool{},
		profile: profileByID(s.app.Preferences().StringWithFallback(prefSetupProfile, "systems"))}
	s.setup = w
	w.title = kit.NewText("", kit.Heading, kit.Primary)
	w.count = widget.NewLabel("")
	w.count.Alignment = fyne.TextAlignTrailing
	w.count.Importance = widget.LowImportance
	w.body = container.NewStack()
	w.back = kit.NewButton("Back", func() { w.go_(w.step - 1) })
	w.next = kit.NewButton("Next", func() { w.go_(w.step + 1) })
	w.next.Variant = kit.ButtonCTA
	skip := kit.NewButton("Skip Setup", w.finish)
	scroll := container.NewVScroll(w.body)
	scroll.SetMinSize(fyne.NewSize(680, 400))
	head := container.NewVBox(container.NewBorder(nil, nil, w.title, nil, w.count), kit.NewRule(false))
	foot := container.NewVBox(kit.NewRule(false), container.NewHBox(skip, layout.NewSpacer(), w.back, w.next))
	content := container.NewBorder(head, foot, nil, nil, scroll)
	w.d = dialog.NewCustomWithoutButtons("Set Up PyxForge", content, s.win)
	w.d.Resize(fyne.NewSize(760, 560))
	s.presentWithClosed(w.d, func() { w.go_(w.step + 1) }, func() {
		s.app.Preferences().SetBool(prefSetupDone, true) // closed is dismissed: not again at start-up
		if s.setup == w {
			s.setup = nil
		}
	})
	s.lastDialog = w.d
	w.render()
}

// go_ moves to step i; past the last step it finishes.
func (w *setupWizard) go_(i int) {
	if i >= len(setupSteps) {
		w.finish()
		return
	}
	w.step = max(i, 0)
	w.render()
}

// finish records that setup was done (or skipped) and closes the wizard.
func (w *setupWizard) finish() {
	p := w.s.app.Preferences()
	p.SetBool(prefSetupDone, true)
	p.SetBool(prefSetupOffered, true) // the Dependency Manager is not offered on top of it
	w.d.Hide()
}

// render draws the current step; it runs again when the tool check or an install moves on.
func (w *setupWizard) render() {
	w.title.SetText(setupSteps[w.step])
	w.count.SetText(fmt.Sprintf("Step %d of %d", w.step+1, len(setupSteps)))
	if w.step == 0 {
		w.back.Disable()
	} else {
		w.back.Enable()
	}
	w.next.SetText("Next")
	if w.step == len(setupSteps)-1 {
		w.next.SetText("Finish")
	}
	var c fyne.CanvasObject
	switch w.step {
	case 0:
		c = w.welcome()
	case 1:
		c = w.profiles()
	case 2:
		c = w.tools()
	case 3:
		c = w.install()
	default:
		c = w.project()
	}
	w.body.Objects = []fyne.CanvasObject{container.New(layout.NewCustomPaddedLayout(theme.Space2, theme.Space2, 0, theme.Space2), c)}
	w.body.Refresh()
}

func para(text string, role kit.Role) fyne.CanvasObject {
	return kit.NewParagraph(text, role)
}

func (w *setupWizard) welcome() fyne.CanvasObject {
	settings, _ := os.UserConfigDir()
	toolsDir := "your package manager's folders"
	if m, err := w.s.toolchainManager(); err == nil && len(managed.ForPlatform()) > 0 {
		toolsDir = m.Root
	}
	facts := container.New(layout.NewFormLayout(),
		kit.NewText("PyxForge", kit.Label, kit.Tertiary), kit.NewText(buildinfo.Version, kit.Mono, kit.Secondary),
		kit.NewText("System", kit.Label, kit.Tertiary), kit.NewText(runtime.GOOS+" "+runtime.GOARCH, kit.Mono, kit.Secondary),
		kit.NewText("Settings", kit.Label, kit.Tertiary), kit.NewText(filepath.Join(settings, "PyxForge"), kit.Mono, kit.Secondary),
		kit.NewText("Toolchains", kit.Label, kit.Tertiary), kit.NewText(toolsDir, kit.Mono, kit.Secondary),
	)
	return container.NewVBox(
		para("PyxForge edits, builds, runs and debugs your code in one window. Its editor and terminal need nothing else; compilers, debuggers, QEMU and language servers each add a feature when they are installed.", kit.Primary),
		para("This takes a minute: say what you will build, see which tools are already here, install the missing ones PyxForge can manage, and open a first project. Nothing is downloaded unless you choose it, and nothing outside PyxForge's own folder changes.", kit.Secondary),
		widget.NewSeparator(), facts)
}

func (w *setupWizard) profiles() fyne.CanvasObject {
	var names []string
	for _, p := range devProfiles {
		names = append(names, p.name)
	}
	about := kit.NewParagraph(w.profile.about, kit.Secondary)
	r := widget.NewRadioGroup(names, nil)
	r.Required = true
	r.SetSelected(w.profile.name)
	r.OnChanged = func(name string) {
		for _, p := range devProfiles {
			if p.name == name && p.id != w.profile.id {
				w.profile = p
				w.s.app.Preferences().SetString(prefSetupProfile, p.id)
				if !w.touched {
					clear(w.picks) // the next step suggests again for this profile
				}
				about.Segments[0].(*widget.TextSegment).Text = p.about
				about.Refresh()
			}
		}
	}
	return container.NewVBox(
		para("Choose the work you do most. It decides which tools the next steps check and suggest; you can install anything else later from Run > Dependency Manager…", kit.Secondary),
		r, about)
}

// wanted are the profile's tools as the last check found them.
func (w *setupWizard) wanted() []toolchain.Status {
	var out []toolchain.Status
	for _, id := range w.profile.tools {
		if st, ok := toolchain.ByID(w.s.tools, id); ok {
			out = append(out, st)
		}
	}
	return out
}

func (w *setupWizard) tools() fyne.CanvasObject {
	if len(w.s.tools) == 0 {
		return container.NewVBox(para("Checking which tools are on this computer…", kit.Secondary), widget.NewProgressBarInfinite())
	}
	box := container.NewVBox(para("What "+w.profile.name+" uses, as PyxForge finds it now. Tools: Doctor… shows every tool and lets you point at one by hand.", kit.Secondary))
	found := 0
	for _, st := range w.wanted() {
		icon, detail := kit.NewIcon(icons.CircleCheck, kit.Success), st.Version+" · "+sourceName(st.Source)
		if !st.Found() {
			icon, detail = kit.NewIcon(icons.Circle, kit.Tertiary), "missing"
			if st.Tool.Optional {
				detail += " (optional)"
			}
		} else {
			found++
		}
		feature := kit.NewText(st.Tool.Feature, kit.Body, kit.Tertiary)
		feature.TextSize = theme.TextSmall
		box.Add(container.NewBorder(nil, nil, container.NewCenter(icon), kit.NewText(detail, kit.Mono, kit.Secondary),
			container.NewVBox(kit.NewText(st.Tool.Label, kit.Strong, kit.Primary), feature)))
	}
	box.Objects = slices.Insert(box.Objects, 1, fyne.CanvasObject(kit.NewText(fmt.Sprintf("%d of %d found", found, len(w.wanted())), kit.Label, kit.Secondary)))
	return box
}

// candidates are the packages PyxForge manages that provide a missing tool of the profile, and
// whether each is installed.
func (w *setupWizard) candidates() []managed.Package {
	var out []managed.Package
	for _, p := range managed.ForPlatform() {
		for _, st := range w.wanted() {
			if !st.Found() && slices.Contains(p.Provides, st.Tool.ID) {
				out = append(out, p)
				break
			}
		}
	}
	return out
}

func (w *setupWizard) install() fyne.CanvasObject {
	s := w.s
	box := container.NewVBox()
	cands := w.candidates()
	m, merr := s.toolchainManager()
	if len(cands) > 0 && merr == nil {
		box.Add(para("PyxForge can install these into its own folder, each pinned to a version and checked against its published SHA-256. Your PATH and your own installations are not changed.", kit.Secondary))
		if len(w.picks) == 0 {
			for _, p := range cands {
				w.picks[p.ID] = m.Status(p).State != managed.Installed
			}
		}
		var total int64
		for _, p := range cands {
			p := p
			st := m.Status(p)
			busy := s.depInstalls[p.ID]
			label := fmt.Sprintf("%s %s (%.0f MB)", p.Name, p.Version, float64(p.Size)/1e6)
			var row fyne.CanvasObject
			switch {
			case busy != nil:
				bar := widget.NewProgressBar()
				if busy.last.Total > 0 {
					bar.SetValue(float64(busy.last.Done) / float64(busy.last.Total))
				}
				phase := "Starting"
				if busy.last.Phase != "" {
					phase = busy.last.Phase
				}
				row = container.NewVBox(kit.NewText(label, kit.Strong, kit.Primary),
					container.NewBorder(nil, nil, nil, container.NewHBox(kit.NewText(phase+"…", kit.Body, kit.Secondary),
						kit.NewButton("Cancel", busy.cancel)), bar))
			case st.State == managed.Installed || st.State == managed.External:
				row = container.NewHBox(kit.NewIcon(icons.CircleCheck, kit.Success), kit.NewText(label+" · installed", kit.Body, kit.Primary))
			default:
				c := widget.NewCheck(label, func(on bool) { w.picks[p.ID] = on; w.touched = true })
				c.SetChecked(w.picks[p.ID])
				row = container.NewVBox(c, para(p.About, kit.Tertiary))
				if w.picks[p.ID] {
					total += p.Size
				}
			}
			box.Add(row)
		}
		start := kit.NewButton("Install Selected", func() {
			for _, p := range cands {
				if w.picks[p.ID] && s.depInstalls[p.ID] == nil && m.Status(p).State != managed.Installed {
					s.installToolchain(p)
				}
			}
			w.render()
		})
		start.Variant = kit.ButtonCTA
		if total == 0 {
			start.Disable()
		}
		box.Add(container.NewHBox(start, kit.NewText(fmt.Sprintf("%.0f MB to download", float64(total)/1e6), kit.Body, kit.Tertiary)))
		box.Add(para("Installs go on in the background; you can carry on and finish setup meanwhile.", kit.Tertiary))
	}
	// Tools PyxForge does not install: how to get them, to review and run.
	var cmds []string
	for _, st := range w.wanted() {
		if st.Found() || slices.ContainsFunc(cands, func(p managed.Package) bool { return slices.Contains(p.Provides, st.Tool.ID) }) {
			continue
		}
		if h := st.Hint(); h != "" {
			cmds = append(cmds, st.Tool.Label+": "+h)
		}
	}
	if len(cmds) > 0 {
		box.Add(widget.NewSeparator())
		box.Add(para("Install these yourself; PyxForge finds them on PATH or in their usual folders the next time it checks:", kit.Secondary))
		list := widget.NewLabel(strings.Join(cmds, "\n"))
		list.TextStyle = fyne.TextStyle{Monospace: true}
		list.Wrapping = fyne.TextWrapWord
		box.Add(list)
		box.Add(container.NewHBox(kit.NewButton("Copy", func() {
			s.app.Clipboard().SetContent(strings.Join(cmds, "\n"))
		}), kit.NewButton("Check Again", func() { s.checkTools(false) })))
	}
	if len(box.Objects) == 0 {
		box.Add(para("Everything "+w.profile.name+" uses is already here. Nothing to install.", kit.Primary))
	}
	return box
}

func (w *setupWizard) project() fyne.CanvasObject {
	s := w.s
	open := kit.NewButton("Open Folder…", func() { w.finish(); s.openFolder() })
	create := kit.NewButton("New Project…", func() { w.finish(); s.newProject() })
	create.Variant = kit.ButtonCTA
	return container.NewVBox(
		para("Open the folder you work in, or start a project from a preset: an empty folder, a boot sector that builds and boots in QEMU, or a C program. Each comes with a pyxforge.toml that builds as it is.", kit.Secondary),
		container.NewHBox(create, open),
		para("Or press Finish and open one later from File. Help > Setup Wizard… brings this back any time.", kit.Tertiary))
}
