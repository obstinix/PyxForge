// Package icons serves PyxForge's bundled Lucide line icons in any colour.
//
// Fyne's themed resources recolour fills, which would fill these stroke-only outlines solid,
// so icons are recoloured here by substituting currentColor.
package icons

import (
	"embed"
	"fmt"
	"image/color"
	"regexp"
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
	Ellipsis      Name = "ellipsis"
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
	Refresh       Name = "refresh-cw"
	Search        Name = "search"
	Server        Name = "server"
	Settings      Name = "settings"
	Square        Name = "square"
	SunMoon       Name = "sun-moon"
	Terminal      Name = "terminal"
	TriangleAlert Name = "triangle-alert"
	X             Name = "x"
	FileDiff      Name = "file-diff"
	GitCommit     Name = "git-commit-horizontal"
	Camera        Name = "camera"
	ChevronUp     Name = "chevron-up"
	Compare       Name = "git-compare"
	Save          Name = "save"
	Minus         Name = "minus"
	Pause         Name = "pause"
	StepForward   Name = "step-forward"
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
	svg := strings.ReplaceAll(explicitArcs(explicitShapes(string(src))), "currentColor", paint)
	if k.rgba.A < 0xff {
		svg = strings.Replace(svg, "<svg", fmt.Sprintf(`<svg stroke-opacity="%.3f"`, float64(k.rgba.A)/255), 1)
	}
	r := fyne.NewStaticResource(fmt.Sprintf("%s-%s.svg", n, paint[1:]), []byte(svg))
	cache[k] = r
	return r
}

var (
	svgTag    = regexp.MustCompile(`<svg\b[^>]*>`)
	lineTag   = regexp.MustCompile(`<line\b([^>]*?)/>`)
	shapeTag  = regexp.MustCompile(`<(?:path|circle|rect|ellipse|polygon)\b[^>]*?/>`)
	attr      = regexp.MustCompile(`\b([a-z0-9-]+)="([^"]*)"`)
	pathData  = regexp.MustCompile(`\bd="([^"]*)"`)
	pathToken = regexp.MustCompile(`[MmZzLlHhVvCcSsQqTtAa]|[-+]?(?:\d+\.?\d*|\.\d+)(?:[eE][-+]?\d+)?`)
)

// explicitShapes keeps an icon whole when Fyne recolours a disabled button's icon. That rewrite
// fills every shape not marked fill="none" itself (Lucide marks only the root), drops <line>
// elements and the root's stroke, which turned the commit icon into a dot (#50). Lines become
// paths, and every shape gets the root's stroke and, without a fill of its own, no fill.
func explicitShapes(svg string) string {
	var stroke strings.Builder
	if root := svgTag.FindString(svg); root != "" {
		for _, kv := range attr.FindAllStringSubmatch(root, -1) {
			if strings.HasPrefix(kv[1], "stroke") {
				fmt.Fprintf(&stroke, ` %s="%s"`, kv[1], kv[2])
			}
		}
	}
	svg = lineTag.ReplaceAllStringFunc(svg, func(m string) string {
		a := map[string]string{}
		for _, kv := range attr.FindAllStringSubmatch(m, -1) {
			a[kv[1]] = kv[2]
		}
		return fmt.Sprintf(`<path d="M%s %sL%s %s" />`, a["x1"], a["y1"], a["x2"], a["y2"])
	})
	return shapeTag.ReplaceAllStringFunc(svg, func(m string) string {
		extra := ""
		if !strings.Contains(m, " fill=") {
			extra = ` fill="none"`
		}
		if !strings.Contains(m, " stroke=") {
			extra += stroke.String()
		}
		return strings.TrimRight(strings.TrimSuffix(m, "/>"), " \t\n") + extra + " />"
	})
}

// explicitArcs rewrites every path so that each arc has its own command letter. Lucide writes
// several arcs after one "a" (implicit repetition, valid SVG); Fyne's SVG renderer draws those
// wrongly, which turned the settings gear into an "8".
func explicitArcs(svg string) string {
	return pathData.ReplaceAllStringFunc(svg, func(m string) string {
		return `d="` + explicitArcPath(pathData.FindStringSubmatch(m)[1]) + `"`
	})
}

func explicitArcPath(d string) string {
	var b strings.Builder
	cmd, n := "", 0 // current command and how many numbers have followed it
	for _, t := range pathToken.FindAllString(d, -1) {
		if c := t[0]; (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') {
			cmd, n = t, 0
			b.WriteString(t)
			continue
		}
		switch {
		case (cmd == "a" || cmd == "A") && n > 0 && n%7 == 0:
			b.WriteString(cmd) // start the next arc explicitly
		case n > 0:
			b.WriteByte(' ')
		}
		b.WriteString(t)
		n++
	}
	return b.String()
}
