package splash

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

// SystemReducesMotion reports whether Windows has animations turned off (Settings >
// Accessibility > Visual effects > Animation effects).
func SystemReducesMotion() bool {
	const spiGetClientAreaAnimation = 0x1042
	var on uint32
	r, _, _ := windows.NewLazySystemDLL("user32.dll").NewProc("SystemParametersInfoW").
		Call(spiGetClientAreaAnimation, 0, uintptr(unsafe.Pointer(&on)), 0)
	return r != 0 && on == 0
}
