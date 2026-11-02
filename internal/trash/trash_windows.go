package trash

import (
	"errors"
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

// shFileOpStruct is SHFILEOPSTRUCTW.
type shFileOpStruct struct {
	hwnd          uintptr
	function      uint32
	from          *uint16
	to            *uint16
	flags         uint16
	aborted       int32
	nameMappings  uintptr
	progressTitle *uint16
}

// move sends abs to the Recycle Bin, silently, as Explorer's Delete does.
func move(abs string) error {
	const (
		foDelete         = 3
		fofSilent        = 0x4
		fofNoConfirm     = 0x10
		fofAllowUndo     = 0x40
		fofNoErrorUI     = 0x400
		fofNoConfirmMkdr = 0x200
	)
	from, err := windows.UTF16FromString(abs)
	if err != nil {
		return err
	}
	from = append(from, 0) // the list of paths ends with an empty one
	op := shFileOpStruct{function: foDelete, from: &from[0], flags: fofAllowUndo | fofNoConfirm | fofSilent | fofNoErrorUI | fofNoConfirmMkdr}
	r, _, _ := windows.NewLazySystemDLL("shell32.dll").NewProc("SHFileOperationW").Call(uintptr(unsafe.Pointer(&op)))
	switch {
	case r != 0:
		return fmt.Errorf("could not move it to the Recycle Bin (code %#x); is it open in another program?", r)
	case op.aborted != 0:
		return errors.New("moving it to the Recycle Bin was cancelled")
	}
	return nil
}

func isCrossDevice(err error) bool { return errors.Is(err, windows.ERROR_NOT_SAME_DEVICE) }
