// Command appicon writes PyxForge's brand assets from the one definition in brand.go: the
// vector files (the icon, the mark for dark and light backgrounds and in one colour, and the
// logo with the name as outlines), the PNGs the app embeds, and the Windows .ico (16 to 256
// pixels) that tools/winpkg puts into PyxForge.exe and the installer. TestIconsAreCurrent
// checks that the files in the repository match what this writes.
//
//	go run ./tools/appicon
package main

import (
	"bytes"
	"encoding/binary"
	"flag"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"

	"github.com/srwiley/oksvg"
	"github.com/srwiley/rasterx"
)

const (
	displayFace  = "internal/ui/theme/fonts/Geist-SemiBold.ttf" // the name in the logo
	appPNG       = markFolder + "icon.png"                      // 256 pixels, the window and taskbar icon
	markPNG      = markFolder + "mark.png"                      // the mark on dark surfaces: start-up, About
	markLightPNG = markFolder + "mark-light.png"                // and on light ones
	winICO       = "packaging/windows/pyxforge.ico"             // every size Windows asks for
	appSize      = 256
	smallBelow   = 32 // sizes under this use the simplified icon
)

// icoSizes are the sizes Windows uses for shell, taskbar, Start menu and high-DPI displays.
var icoSizes = []int{16, 20, 24, 32, 40, 48, 64, 128, 256}

func main() {
	face := flag.String("face", displayFace, "the TrueType face for the name in the logo")
	out := flag.String("out", "", "write under this folder instead of the repository (to compare faces)")
	flag.Parse()
	files, err := render(*face)
	if err != nil {
		fmt.Fprintln(os.Stderr, "appicon:", err)
		os.Exit(1)
	}
	for path, data := range files {
		path = filepath.Join(*out, filepath.FromSlash(path))
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

// rasterize draws an SVG to a PNG, width pixels wide, keeping its proportions.
func rasterize(svg string, width int) ([]byte, error) {
	icon, err := oksvg.ReadIconStream(strings.NewReader(svg), oksvg.StrictErrorMode)
	if err != nil {
		return nil, err
	}
	height := int(float64(width)*icon.ViewBox.H/icon.ViewBox.W + 0.5)
	icon.SetTarget(0, 0, float64(width), float64(height))
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	icon.Draw(rasterx.NewDasher(width, height, rasterx.NewScannerGV(width, height, img, img.Bounds())), 1)
	var buf bytes.Buffer
	if err := (&png.Encoder{CompressionLevel: png.BestCompression}).Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// render returns the files appicon writes, by path.
func render(face string) (map[string][]byte, error) {
	svgs, err := brandSVGs(face)
	if err != nil {
		return nil, err
	}
	files := map[string][]byte{}
	for path, svg := range svgs {
		files[path] = []byte(svg)
	}
	for path, from := range map[string]string{appPNG: "icon.svg", markPNG: "mark.svg", markLightPNG: "mark-light.svg"} {
		if files[path], err = rasterize(svgs[markFolder+from], appSize); err != nil {
			return nil, err
		}
	}
	var images [][]byte
	for _, s := range icoSizes {
		from := "icon.svg"
		if s < smallBelow {
			from = "icon-small.svg"
		}
		b, err := rasterize(svgs[markFolder+from], s)
		if err != nil {
			return nil, err
		}
		images = append(images, b)
	}
	files[winICO] = ico(icoSizes, images)
	return files, nil
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
