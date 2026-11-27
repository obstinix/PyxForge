package graphview

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"github.com/obstinix/PyxForge/internal/ui/icons"
	"github.com/obstinix/PyxForge/internal/ui/kit"
	"github.com/obstinix/PyxForge/internal/ui/theme"
)

// Controls is the panel floating over the graph's top left (Graph view > controls): Filters,
// Groups, Display, Forces and Presets, each a section that opens and closes; the whole panel
// collapses to its title. It is Liquid Glass when that material is chosen.
type Controls struct {
	v        *View
	Float    *kit.Float
	body     *fyne.Container
	collapse *kit.IconButton
	open     bool
	sections map[string]*section
	count    *kit.Text
	search   *kit.SearchField

	// Presets are named settings; OnPresets is told when they change (workspace state).
	Presets   map[string]Settings
	OnPresets func(map[string]Settings)
	presetBox *kit.Dropdown
	presetIn  *widget.Entry
	building  bool
	host      *fyne.Container // the overlay the panel floats in
	head      *fyne.Container
	scroll    *container.Scroll
}

// wantHeight is the panel's height with every open section shown, before any scrolling.
func (c *Controls) wantHeight() float32 {
	h := c.head.MinSize().Height + 2*c.Float.Padding
	if c.open {
		h += c.body.MinSize().Height + theme.Space1
	}
	return h
}

// section is one part of the panel.
type section struct {
	head *kit.Button
	body *fyne.Container
	open bool
}

// ControlsWidth is the panel's width when open.
const ControlsWidth = 300

// NewControls makes the panel for a view.
func NewControls(v *View) *Controls {
	c := &Controls{v: v, open: true, sections: map[string]*section{}, Presets: map[string]Settings{}}
	c.count = kit.NewText("", kit.Label, kit.Secondary)
	c.collapse = kit.NewIconButton(icons.ChevronUp, "Hide Graph Controls", c.toggle)
	c.collapse.Side = 26
	title := kit.NewText("Graph", kit.Strong, kit.Primary)
	c.head = container.NewBorder(nil, nil, title, c.collapse, container.NewCenter(c.count))
	c.body = container.NewVBox()
	// The sections scroll when the panel is taller than the graph's area.
	c.scroll = container.NewVScroll(c.body)
	c.Float = kit.NewGlassFloat(container.NewBorder(c.head, nil, nil, nil, c.scroll))
	c.Float.Padding = theme.Space2
	c.Rebuild()
	return c
}

// toggle opens or collapses the panel.
func (c *Controls) toggle() {
	c.open = !c.open
	if c.open {
		c.scroll.Show()
	} else {
		c.scroll.Hide()
	}
	c.collapse.Icon, c.collapse.Label = icons.ChevronDown, "Show Graph Controls"
	if c.open {
		c.collapse.Icon, c.collapse.Label = icons.ChevronUp, "Hide Graph Controls"
	}
	c.collapse.Refresh()
	c.Float.Refresh()
	if c.host != nil {
		c.host.Refresh() // lays the panel out again at its new height
	}
}

// Open reports whether the panel is open.
func (c *Controls) Open() bool { return c.open }

// Toggle opens or collapses the panel; for the keyboard and tests.
func (c *Controls) Toggle() { c.toggle() }

// set changes the view's settings through f and refreshes the count.
func (c *Controls) set(f func(*Settings)) {
	if c.building {
		return
	}
	s := c.v.Settings()
	f(&s)
	c.v.SetSettings(s)
	c.updateCount()
}

func (c *Controls) updateCount() {
	g := c.v.Graph()
	if g == nil {
		c.count.SetText("")
		return
	}
	n := 0
	for _, n0 := range c.v.vis {
		if n0 {
			n++
		}
	}
	c.count.SetText(fmt.Sprintf("%d of %d files", n, len(g.Nodes)))
}

// Rebuild makes the panel's controls again from the view's settings (a preset was chosen).
func (c *Controls) Rebuild() {
	c.building = true
	defer func() { c.building = false }()
	s := c.v.Settings()
	c.body.Objects = nil
	add := func(name string, rows ...fyne.CanvasObject) {
		sec := c.sections[name]
		if sec == nil {
			sec = &section{open: name == "Filters" || name == "Display"}
			c.sections[name] = sec
		}
		sec.body = container.NewVBox(rows...)
		sec.body.Hidden = !sec.open
		sec.head = kit.NewButtonWithIcon(name, chevron(sec.open), nil)
		sec.head.Variant = kit.ButtonGhost
		sec.head.OnTapped = func() {
			sec.open = !sec.open
			sec.body.Hidden = !sec.open
			sec.head.Icon = chevron(sec.open)
			sec.head.Refresh()
			c.body.Refresh()
			if c.host != nil {
				c.host.Refresh()
			}
		}
		c.body.Add(container.NewBorder(nil, nil, sec.head, nil))
		c.body.Add(container.New(layout.NewCustomPaddedLayout(0, theme.Space1, theme.Space3, 0), sec.body))
	}

	// Filters.
	c.search = kit.NewSearchField("path: ext: tag:  -to exclude", func(q string) { c.set(func(s *Settings) { s.Query = q }) })
	c.search.SetText(s.Query)
	sw := func(label string, on bool, f func(bool)) fyne.CanvasObject {
		k := kit.NewSwitch(on, f)
		k.Label = label
		return container.NewBorder(nil, nil, nil, k, kit.NewText(label, kit.Body, kit.Secondary))
	}
	add("Filters", c.search,
		sw("Orphans", s.Orphans, func(on bool) { c.set(func(s *Settings) { s.Orphans = on }) }),
		sw("Attachments", s.Attachments, func(on bool) { c.set(func(s *Settings) { s.Attachments = on }) }),
		sw("Notes only", s.NotesOnly, func(on bool) { c.set(func(s *Settings) { s.NotesOnly = on }) }),
	)

	// Groups.
	groupNames := map[string]string{"folder": "Folder", "extension": "Extension", "cluster": "Cluster", "query": "Queries"}
	groupKeys := map[string]string{"Folder": "folder", "Extension": "extension", "Cluster": "cluster", "Queries": "query"}
	by := kit.NewDropdown([]string{"Folder", "Extension", "Cluster", "Queries"}, groupNames[s.GroupBy], func(v string) {
		c.set(func(s *Settings) { s.GroupBy = groupKeys[v] })
		c.Rebuild()
	})
	by.Label = "Colour by"
	scheme := kit.NewDropdown(SchemeNames, SchemeNames[s.Scheme], func(v string) {
		c.set(func(s *Settings) { s.Scheme = Scheme(slices.Index(SchemeNames, v)) })
	})
	scheme.Label = "Colours"
	groupRows := []fyne.CanvasObject{labelled("Colour by", by), labelled("Colours", scheme)}
	if s.GroupBy == "query" {
		for gi := range s.Groups {
			q := widget.NewEntry()
			q.SetText(s.Groups[gi].Query)
			q.OnChanged = func(t string) { c.set(func(s *Settings) { s.Groups[gi].Query = t }) }
			del := kit.NewIconButton(icons.X, "Remove Group", func() {
				c.set(func(s *Settings) { s.Groups = slices.Delete(slices.Clone(s.Groups), gi, gi+1) })
				c.Rebuild()
			})
			del.Side = 26
			groupRows = append(groupRows, container.NewBorder(nil, nil, nil, del, q))
		}
		addGroup := kit.NewButton("New group", func() {
			c.set(func(s *Settings) {
				s.Groups = append(slices.Clone(s.Groups), Group{Query: "ext:c", Colour: len(s.Groups)})
			})
			c.Rebuild()
		})
		groupRows = append(groupRows, addGroup)
	}
	add("Groups", groupRows...)

	// Display.
	size := kit.NewDropdown([]string{"Uniform", "Degree", "PageRank"}, map[string]string{"uniform": "Uniform", "degree": "Degree", "pagerank": "PageRank"}[s.NodeSize],
		func(v string) { c.set(func(s *Settings) { s.NodeSize = strings.ToLower(v) }) })
	size.Label = "Node size"
	glow := kit.NewDropdown([]string{"None", "Hubs", "All"}, strings.ToUpper(s.Glow[:1])+s.Glow[1:],
		func(v string) { c.set(func(s *Settings) { s.Glow = strings.ToLower(v) }) })
	glow.Label = "Glow"
	slider := func(label string, lo, hi, step, val float64, f func(float64)) fyne.CanvasObject {
		return kit.NewSlider(label, lo, hi, step, val, f)
	}
	add("Display",
		labelled("Node size", size),
		slider("Node size factor", 0.4, 3, 0.1, s.SizeFactor, func(x float64) { c.set(func(s *Settings) { s.SizeFactor = x }) }),
		slider("Link thickness", 0.5, 4, 0.5, s.LinkWidth, func(x float64) { c.set(func(s *Settings) { s.LinkWidth = x }) }),
		slider("Link opacity", 0.1, 1, 0.05, s.LinkOpacity, func(x float64) { c.set(func(s *Settings) { s.LinkOpacity = x }) }),
		sw("Arrows", s.Arrows, func(on bool) { c.set(func(s *Settings) { s.Arrows = on }) }),
		slider("Label threshold", 1, 20, 1, s.LabelAt, func(x float64) { c.set(func(s *Settings) { s.LabelAt = x }) }),
		labelled("Glow", glow),
	)

	// Forces.
	f := s.Forces
	add("Forces",
		slider("Repulsion", 100, 3000, 50, f.Repulsion, func(x float64) { c.set(func(s *Settings) { s.Forces.Repulsion = x }) }),
		slider("Link length", 20, 200, 5, f.LinkLength, func(x float64) { c.set(func(s *Settings) { s.Forces.LinkLength = x }) }),
		slider("Centre pull", 0, 0.1, 0.002, f.Centre, func(x float64) { c.set(func(s *Settings) { s.Forces.Centre = x }) }),
		slider("Link strength", 0.01, 1, 0.01, f.LinkStrength, func(x float64) { c.set(func(s *Settings) { s.Forces.LinkStrength = x }) }),
		slider("Damping", 0.5, 0.98, 0.01, f.Damping, func(x float64) { c.set(func(s *Settings) { s.Forces.Damping = x }) }),
		kit.NewButton("Re-form", func() { c.v.Reform(uint64(len(c.v.Graph().Nodes)) + 1) }),
	)

	// Presets.
	names := make([]string, 0, len(c.Presets))
	for n := range c.Presets {
		names = append(names, n)
	}
	sort.Strings(names)
	c.presetBox = kit.NewDropdown(names, "", func(name string) { c.Choose(name) })
	c.presetBox.Label = "Preset"
	c.presetIn = widget.NewEntry()
	c.presetIn.SetPlaceHolder("Name this view")
	save := kit.NewButton("Save", func() { c.Save(c.presetIn.Text) })
	del := kit.NewButton("Delete", func() { c.Delete(c.presetBox.Selected) })
	del.Variant = kit.ButtonDanger
	add("Presets", labelled("Preset", c.presetBox), container.NewBorder(nil, nil, nil, save, c.presetIn), del)
	c.body.Refresh()
	c.updateCount()
}

func chevron(open bool) icons.Name {
	if open {
		return icons.ChevronDown
	}
	return icons.ChevronRight
}

func labelled(label string, o fyne.CanvasObject) fyne.CanvasObject {
	return container.NewBorder(nil, nil, kit.NewText(label, kit.Body, kit.Secondary), o)
}

// Save stores the view's settings under a name.
func (c *Controls) Save(name string) {
	name = strings.TrimSpace(name)
	if name == "" {
		return
	}
	c.Presets[name] = c.v.Settings()
	if c.OnPresets != nil {
		c.OnPresets(c.Presets)
	}
	c.Rebuild()
	c.presetBox.Selected = name
	c.presetBox.Refresh()
}

// Choose applies a saved preset.
func (c *Controls) Choose(name string) {
	s, ok := c.Presets[name]
	if !ok {
		return
	}
	c.v.SetSettings(s)
	c.Rebuild()
	c.presetBox.Selected = name
	c.presetBox.Refresh()
}

// Delete removes a preset.
func (c *Controls) Delete(name string) {
	if _, ok := c.Presets[name]; !ok {
		return
	}
	delete(c.Presets, name)
	if c.OnPresets != nil {
		c.OnPresets(c.Presets)
	}
	c.Rebuild()
}

// SectionOpen reports whether a section is open; for tests.
func (c *Controls) SectionOpen(name string) bool {
	s := c.sections[name]
	return s != nil && s.open
}

// OpenSection opens a section (review renders).
func (c *Controls) OpenSection(name string) {
	if s := c.sections[name]; s != nil && !s.open {
		s.head.OnTapped()
	}
}

// Overlay places the panel over a view: the view filling the area, the panel at its top left.
func Overlay(v *View, c *Controls) fyne.CanvasObject {
	c.host = container.New(&overlayLayout{c: c}, v, c.Float)
	return c.host
}

type overlayLayout struct{ c *Controls }

func (l *overlayLayout) Layout(objs []fyne.CanvasObject, s fyne.Size) {
	objs[0].Move(fyne.NewPos(0, 0))
	objs[0].Resize(s)
	ms := objs[1].MinSize()
	w := float32(ControlsWidth)
	h := min(l.c.wantHeight(), s.Height-2*theme.Space3)
	if !l.c.open {
		w = min(ms.Width, ControlsWidth)
	}
	objs[1].Move(fyne.NewPos(theme.Space3, theme.Space3))
	objs[1].Resize(fyne.NewSize(min(w, s.Width-2*theme.Space3), h))
}

func (l *overlayLayout) MinSize(objs []fyne.CanvasObject) fyne.Size { return objs[0].MinSize() }
