package shell

import (
	"fmt"
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"github.com/obstinix/PyxForge/internal/inspect"
	"github.com/obstinix/PyxForge/internal/ui/icons"
	"github.com/obstinix/PyxForge/internal/ui/kit"
	"github.com/obstinix/PyxForge/internal/ui/theme"
)

const mapCols = 16 // bytes per row, as in the hex dump

// regionColor gives each kind of region a colour from the theme. The signature is the one
// status-coloured region: green when valid, red when not.
func regionColor(r inspect.Region, ok bool) color.NRGBA {
	t := theme.Current()
	switch r.Kind {
	case "code":
		return t.Text.Secondary
	case "jump", "oem", "bpb", "ebpb":
		return t.Syntax.Constant
	case "disk-signature", "partition-table":
		return t.Syntax.Number
	case "padding":
		return t.Border.Hairline
	case "signature":
		if ok {
			return t.Status.Success
		}
		return t.Status.Error
	case "short":
		return t.Status.Warning
	}
	return t.Text.Disabled
}

// sectorGrid draws the 512 bytes of a sector as cells, 16 to a row, coloured by region.
// Tapping a cell reports its offset.
type sectorGrid struct {
	widget.BaseWidget
	m     inspect.SectorMap
	onTap func(offset int)
}

func newSectorGrid(onTap func(int)) *sectorGrid {
	g := &sectorGrid{onTap: onTap}
	g.ExtendBaseWidget(g)
	return g
}

func (g *sectorGrid) set(m inspect.SectorMap) {
	g.m = m
	g.Refresh()
}

// cell is the edge of one byte's square, with a 1 px gap.
func (g *sectorGrid) cell(width float32) float32 { return max(4, min(16, width/mapCols)) }

func (g *sectorGrid) Tapped(e *fyne.PointEvent) {
	c := g.cell(g.Size().Width)
	col, row := int(e.Position.X/c), int(e.Position.Y/c)
	if off := row*mapCols + col; col < mapCols && off < min(g.m.Size, 512) && g.onTap != nil {
		g.onTap(off)
	}
}

func (g *sectorGrid) CreateRenderer() fyne.WidgetRenderer {
	r := &sectorGridRenderer{g: g}
	for range 512 {
		r.cells = append(r.cells, canvas.NewRectangle(color.Transparent))
	}
	r.Refresh()
	return r
}

type sectorGridRenderer struct {
	g     *sectorGrid
	cells []*canvas.Rectangle
}

func (r *sectorGridRenderer) Refresh() {
	m := r.g.m
	empty := theme.Current().Surface.Sunken
	for i, c := range r.cells {
		c.FillColor = empty
		if i >= min(m.Size, 512) {
			c.FillColor = color.Transparent
		}
		c.CornerRadius = 1
	}
	for _, reg := range m.Regions {
		col := regionColor(reg, m.SignatureOK)
		for i := reg.Start; i < reg.End && i < len(r.cells); i++ {
			r.cells[i].FillColor = col
		}
	}
	for _, c := range r.cells {
		c.Refresh()
	}
}

func (r *sectorGridRenderer) Layout(s fyne.Size) {
	c := r.g.cell(s.Width)
	for i, cell := range r.cells {
		cell.Move(fyne.NewPos(float32(i%mapCols)*c, float32(i/mapCols)*c))
		cell.Resize(fyne.NewSquareSize(c - 1))
	}
}

func (r *sectorGridRenderer) MinSize() fyne.Size {
	return fyne.NewSize(mapCols*4, 512/mapCols*4)
}

func (r *sectorGridRenderer) Objects() []fyne.CanvasObject {
	out := make([]fyne.CanvasObject, len(r.cells))
	for i, c := range r.cells {
		out[i] = c
	}
	return out
}

func (r *sectorGridRenderer) Destroy() {}

// gridHeight keeps the grid square-celled: its height follows its width.
type gridHeight struct{ g *sectorGrid }

func (l gridHeight) Layout(objs []fyne.CanvasObject, s fyne.Size) {
	c := l.g.cell(s.Width)
	objs[0].Resize(fyne.NewSize(c*mapCols, c*512/mapCols))
}

func (l gridHeight) MinSize([]fyne.CanvasObject) fyne.Size {
	c := l.g.cell(l.g.Size().Width)
	return fyne.NewSize(mapCols*4, c*512/mapCols)
}

// mapView is the inspector's Map tab: the sector grid, a summary, and the regions.
type mapView struct {
	body   *fyne.Container
	frame  fyne.CanvasObject
	empty  fyne.CanvasObject
	head   *kit.Text
	grid   *sectorGrid
	legend *widget.List
	m      inspect.SectorMap
	onSeek func(offset int)
}

func newMapView(onSeek func(int)) *mapView {
	v := &mapView{onSeek: onSeek}
	v.empty = placeholder(icons.Binary, "No boot image", "Build a project whose [qemu] boot_image is a raw image; its first sector is mapped here.")
	v.head = kit.NewText("", kit.Body, kit.Secondary)
	v.head.TextSize = theme.TextCaption + 1
	v.grid = newSectorGrid(onSeek)
	v.legend = widget.NewList(func() int { return len(v.m.Regions) },
		func() fyne.CanvasObject {
			sw := canvas.NewRectangle(color.Transparent)
			sw.SetMinSize(fyne.NewSquareSize(10))
			sw.CornerRadius = 2
			where := kit.NewText("", kit.Mono, kit.Tertiary)
			where.TextSize = theme.TextCaption + 1
			label := widget.NewLabel("")
			label.Truncation = fyne.TextTruncateEllipsis
			return container.NewBorder(nil, nil, container.NewHBox(container.NewCenter(sw), where), nil, label)
		},
		func(i widget.ListItemID, o fyne.CanvasObject) {
			reg := v.m.Regions[i]
			row := o.(*fyne.Container)
			label := row.Objects[0].(*widget.Label)
			lead := row.Objects[1].(*fyne.Container)
			sw := lead.Objects[0].(*fyne.Container).Objects[0].(*canvas.Rectangle)
			where := lead.Objects[1].(*kit.Text)
			sw.FillColor = regionColor(reg, v.m.SignatureOK)
			sw.Refresh()
			where.SetText(fmt.Sprintf("%03x–%03x %3d B", reg.Start, reg.End-1, reg.Size()))
			text := reg.Label
			if !reg.Known && reg.Kind != "code" {
				text += " · detected"
			}
			label.SetText(text)
			row.Refresh()
		})
	v.legend.OnSelected = func(i widget.ListItemID) {
		v.legend.UnselectAll()
		if i < len(v.m.Regions) && v.onSeek != nil {
			v.onSeek(v.m.Regions[i].Start)
		}
	}
	top := container.NewVBox(
		container.New(layout.NewCustomPaddedLayout(theme.Space1, theme.Space1, theme.Space2, theme.Space2), v.head),
		container.New(layout.NewCustomPaddedLayout(0, theme.Space2, theme.Space2, theme.Space2), container.New(gridHeight{v.grid}, v.grid)))
	v.frame = container.NewBorder(top, nil, nil, nil, v.legend)
	v.body = container.NewStack(v.empty)
	return v
}

// set maps an image's first sector.
func (v *mapView) set(name string, data []byte) {
	v.m = inspect.MapSector(data)
	switch {
	case v.m.Size < 512:
		v.head.SetText(fmt.Sprintf("%s · %d bytes, shorter than a sector", name, v.m.Size))
	case v.m.SignatureOK:
		v.head.SetText(fmt.Sprintf("%s · signature %02x %02x at 0x1FE: bootable", name, v.m.Signature[0], v.m.Signature[1]))
	default:
		v.head.SetText(fmt.Sprintf("%s · %02x %02x at 0x1FE, not 55 aa: not bootable", name, v.m.Signature[0], v.m.Signature[1]))
	}
	v.grid.set(v.m)
	v.legend.Refresh()
	if len(v.body.Objects) != 1 || v.body.Objects[0] != v.frame {
		v.body.Objects = []fyne.CanvasObject{v.frame}
		v.body.Refresh()
	}
}

func (v *mapView) clear() {
	v.m = inspect.SectorMap{}
	if len(v.body.Objects) != 1 || v.body.Objects[0] != v.empty {
		v.body.Objects = []fyne.CanvasObject{v.empty}
		v.body.Refresh()
	}
}
