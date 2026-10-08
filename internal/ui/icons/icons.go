// Package icons serves PyxForge's bundled Lucide line icons in any colour.
//
// Fyne's themed resources recolour fills, which would fill these stroke-only outlines solid,
// so icons are recoloured here by substituting currentColor.
package icons

import (
	"embed"
	"fmt"
	"image/color"
	"strings"
	"sync"

	"fyne.io/fyne/v2"
)

//go:embed svg/*.svg
var files embed.FS

// Name is a bundled icon's file name without extension.
type Name string

// The icons the app uses. Adding one means copying its SVG from the same Lucide release and
// listing it in docs/design/FONT_LICENSES.md.
const (
	Binary        Name = "binary"
	Bot           Name = "bot"
	Bug           Name = "bug"
	Check         Name = "check"
	ChevronDown   Name = "chevron-down"
	ChevronRight  Name = "chevron-right"
	Circle        Name = "circle"
	CircleCheck   Name = "circle-check"
	CircleX       Name = "circle-x"
	Command       Name = "command"
	CPU           Name = "cpu"
	File          Name = "file"
	FileCode      Name = "file-code"
	FileText      Name = "file-text"
	Flag          Name = "flag"
	Folder        Name = "folder"
	FolderOpen    Name = "folder-open"
	FolderTree    Name = "folder-tree"
	GitBranch     Name = "git-branch"
	Hammer        Name = "hammer"
	Hash          Name = "hash"
	Info          Name = "info"
	List          Name = "list"
	ListTree      Name = "list-tree"
	MemoryStick   Name = "memory-stick"
	Palette       Name = "palette"
	PanelBottom   Name = "panel-bottom"
	PanelLeft     Name = "panel-left"
	PanelRight    Name = "panel-right"
	Play          Name = "play"
	Plus          Name = "plus"
	Search        Name = "search"
	Server        Name = "server"
	Settings      Name = "settings"
	Square        Name = "square"
	SunMoon       Name = "sun-moon"
	Terminal      Name = "terminal"
	TriangleAlert Name = "triangle-alert"
	X             Name = "x"
)

type key struct {
	name Name
	rgba color.NRGBA
}

var (
	mu    sync.Mutex
	cache = map[key]fyne.Resource{}
)

// Get returns the icon drawn in c. Results are cached per name and colour. An unknown name
// panics: names are constants and TestEveryIconLoads loads them all.
func Get(n Name, c color.Color) fyne.Resource {
	k := key{n, color.NRGBAModel.Convert(c).(color.NRGBA)}
	mu.Lock()
	defer mu.Unlock()
	if r, ok := cache[k]; ok {
		return r
	}
	src, err := files.ReadFile("svg/" + string(n) + ".svg")
	if err != nil {
		panic(fmt.Sprintf("icons: no icon %q", n))
	}
	paint := fmt.Sprintf("#%02x%02x%02x", k.rgba.R, k.rgba.G, k.rgba.B)
	svg := strings.ReplaceAll(string(src), "currentColor", paint)
	if k.rgba.A < 0xff {
		svg = strings.Replace(svg, "<svg", fmt.Sprintf(`<svg stroke-opacity="%.3f"`, float64(k.rgba.A)/255), 1)
	}
	r := fyne.NewStaticResource(fmt.Sprintf("%s-%s.svg", n, paint[1:]), []byte(svg))
	cache[k] = r
	return r
}
