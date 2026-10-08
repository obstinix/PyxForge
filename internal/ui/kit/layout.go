package kit

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
)

// Row gives content a fixed height on the 4 px grid and an 8 px leading and trailing inset,
// centring it vertically. List and tree rows use it so density does not depend on font metrics.
func Row(height float32, content fyne.CanvasObject) *fyne.Container {
	return container.New(rowLayout{height}, content)
}

type rowLayout struct{ h float32 }

func (l rowLayout) Layout(objs []fyne.CanvasObject, s fyne.Size) {
	const inset = 8
	for _, o := range objs {
		h := o.MinSize().Height
		o.Resize(fyne.NewSize(s.Width-2*inset, h))
		o.Move(fyne.NewPos(inset, (s.Height-h)/2))
	}
}

func (l rowLayout) MinSize(objs []fyne.CanvasObject) fyne.Size {
	var w float32
	for _, o := range objs {
		w = max(w, o.MinSize().Width)
	}
	return fyne.NewSize(w+16, l.h)
}
