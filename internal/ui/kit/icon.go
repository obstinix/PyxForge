package kit

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/widget"
	"github.com/obstinix/PyxForge/internal/ui/icons"
	"github.com/obstinix/PyxForge/internal/ui/theme"
)

// Icon is a themed line icon.
type Icon struct {
	widget.BaseWidget
	Name     icons.Name
	Role     Role
	IconSize float32 // 0 means theme.IconSize
}

// NewIcon returns a themed icon.
func NewIcon(n icons.Name, r Role) *Icon {
	i := &Icon{Name: n, Role: r}
	i.ExtendBaseWidget(i)
	return i
}

func (i *Icon) CreateRenderer() fyne.WidgetRenderer {
	img := canvas.NewImageFromResource(nil)
	img.FillMode = canvas.ImageFillContain
	r := &iconRenderer{i: i, img: img}
	r.Refresh()
	return r
}

type iconRenderer struct {
	i   *Icon
	img *canvas.Image
}

func (r *iconRenderer) size() float32 {
	if r.i.IconSize > 0 {
		return r.i.IconSize
	}
	return theme.IconSize
}

func (r *iconRenderer) Refresh() {
	r.img.Resource = icons.Get(r.i.Name, r.i.Role.color(theme.Current()))
	r.img.Refresh()
}

func (r *iconRenderer) Layout(s fyne.Size) {
	d := r.size()
	r.img.Resize(fyne.NewSquareSize(d))
	r.img.Move(fyne.NewPos((s.Width-d)/2, (s.Height-d)/2))
}

func (r *iconRenderer) MinSize() fyne.Size           { return fyne.NewSquareSize(r.size()) }
func (r *iconRenderer) Objects() []fyne.CanvasObject { return []fyne.CanvasObject{r.img} }
func (r *iconRenderer) Destroy()                     {}
