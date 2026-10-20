package shell

import "fyne.io/fyne/v2"

// Room a window needs around its content on the screen: the title bar above it, the borders
// beside it (logical pixels, generous so every Windows theme fits).
const (
	frameWidth  = 16
	frameHeight = 48
)

// fitSize shrinks a window content size to fit a usable screen area of workW×workH pixels at
// the given display scale, with room for the window's frame.
func fitSize(size fyne.Size, workW, workH int, scale float32) fyne.Size {
	if scale <= 0 || workW <= 0 || workH <= 0 {
		return size
	}
	maxW := float32(workW)/scale - frameWidth
	maxH := float32(workH)/scale - frameHeight
	return fyne.NewSize(min(size.Width, maxW), min(size.Height, maxH))
}

// fitWindow sizes the window to size, kept inside the screen's usable area (the taskbar
// excluded), and centres it on its screen (PXF-DEF-008): Windows otherwise places each new
// window 38 px lower and further right than the last, until the status bar is under the
// taskbar.
func (s *Shell) fitWindow(size fyne.Size) {
	if w, h, scale, ok := workArea(); ok {
		size = fitSize(size, w, h, scale)
	}
	s.win.Resize(size)
	s.win.CenterOnScreen()
}
