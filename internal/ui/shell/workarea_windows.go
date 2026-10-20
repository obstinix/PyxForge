//go:build windows

package shell

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	user32            = windows.NewLazySystemDLL("user32.dll")
	procSysParamsInfo = user32.NewProc("SystemParametersInfoW")
	procDpiForSystem  = user32.NewProc("GetDpiForSystem")
)

// workArea returns the primary screen's usable area (the screen less the taskbar) in pixels,
// and the system's display scale (1 at 96 DPI), or ok false when Windows does not say.
func workArea() (w, h int, scale float32, ok bool) {
	var r struct{ Left, Top, Right, Bottom int32 }
	const spiGetWorkArea = 0x0030
	if ret, _, _ := procSysParamsInfo.Call(spiGetWorkArea, 0, uintptr(unsafe.Pointer(&r)), 0); ret == 0 {
		return 0, 0, 0, false
	}
	scale = 1
	if procDpiForSystem.Find() == nil {
		if dpi, _, _ := procDpiForSystem.Call(); dpi > 0 {
			scale = float32(dpi) / 96
		}
	}
	return int(r.Right - r.Left), int(r.Bottom - r.Top), scale, true
}
