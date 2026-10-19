// Command appicon renders PyxForge's icon from internal/ui/brand/icon.svg: the PNG the app
// shows as its window and taskbar icon, and the Windows .ico (16 to 256 pixels) that
// tools/winpkg puts into PyxForge.exe and the installer. TestIconsAreCurrent checks that the
// files in the repository match what this writes.
//
//	go run ./tools/appicon
package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"

	"github.com/srwiley/oksvg"
	"github.com/srwiley/rasterx"
)

const (
	source  = "internal/ui/brand/icon.svg"
	appPNG  = "internal/ui/brand/icon.png"     // 256 pixels, embedded in the app
	winICO  = "packaging/windows/pyxforge.ico" // every size Windows asks for
	appSize = 256
)

// icoSizes are the sizes Windows uses for shell, taskbar, Start menu and high-DPI displays.
var icoSizes = []int{16, 20, 24, 32, 40, 48, 64, 128, 256}

func main() {
	files, err := render(source)
	if err != nil {
		fmt.Fprintln(os.Stderr, "appicon:", err)
		os.Exit(1)
	}
	for path, data := range files {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			fmt.Fprintln(os.Stderr, "appicon:", err)
			os.Exit(1)
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			fmt.Fprintln(os.Stderr, "appicon:", err)
			os.Exit(1)
		}
		fmt.Println("wrote", path)
	}
}

// render returns the files appicon writes, by path.
func render(svgPath string) (map[string][]byte, error) {
	src, err := os.ReadFile(svgPath)
	if err != nil {
		return nil, err
	}
	pngAt := func(size int) ([]byte, error) {
		icon, err := oksvg.ReadIconStream(bytes.NewReader(src), oksvg.StrictErrorMode)
		if err != nil {
			return nil, err
		}
		icon.SetTarget(0, 0, float64(size), float64(size))
		img := image.NewRGBA(image.Rect(0, 0, size, size))
		icon.Draw(rasterx.NewDasher(size, size, rasterx.NewScannerGV(size, size, img, img.Bounds())), 1)
		var buf bytes.Buffer
		if err := (&png.Encoder{CompressionLevel: png.BestCompression}).Encode(&buf, img); err != nil {
			return nil, err
		}
		return buf.Bytes(), nil
	}
	app, err := pngAt(appSize)
	if err != nil {
		return nil, err
	}
	var images [][]byte
	for _, s := range icoSizes {
		b, err := pngAt(s)
		if err != nil {
			return nil, err
		}
		images = append(images, b)
	}
	return map[string][]byte{appPNG: app, winICO: ico(icoSizes, images)}, nil
}

// ico packs PNG images into an .ico file: a header, one directory entry per image, then the
// images. Windows Vista and later read PNG entries at every size.
func ico(sizes []int, images [][]byte) []byte {
	var b bytes.Buffer
	le := func(v any) { _ = binary.Write(&b, binary.LittleEndian, v) }
	le(uint16(0))           // reserved
	le(uint16(1))           // type: icon
	le(uint16(len(images))) // count
	offset := 6 + 16*len(images)
	for i, img := range images {
		dim := uint8(sizes[i] % 256) // 0 means 256
		le(dim)
		le(dim)
		le(uint8(0))  // no palette
		le(uint8(0))  // reserved
		le(uint16(1)) // colour planes
		le(uint16(32))
		le(uint32(len(img)))
		le(uint32(offset))
		offset += len(img)
	}
	for _, img := range images {
		b.Write(img)
	}
	return b.Bytes()
}
