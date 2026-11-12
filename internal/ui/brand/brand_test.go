package brand

import (
	"fmt"
	"regexp"
	"strconv"
	"testing"
)

var svgSize = regexp.MustCompile(`<svg[^>]* width="(\d+)" height="(\d+)" viewBox="([^"]+)"`)

// TestAssets: the mark's parts share one frame, so laid over each other they make the mark,
// and the lettering's stated proportions are the files' own. A file changed by hand, or by
// tools/appicon without this package following, fails here.
func TestAssets(t *testing.T) {
	frame := ""
	for name, data := range map[string][]byte{"anvil": anvilSVG, "anvil-light": anvilLightSVG, "pyx": pyxSVG,
		"pyx-light": pyxLightSVG, "spark": sparkSVG, "spark-light": sparkLightSVG} {
		m := svgSize.FindSubmatch(data)
		if m == nil {
			t.Fatalf("%s is not an SVG with a size and a frame", name)
		}
		if frame == "" {
			frame = string(m[3])
		}
		if string(m[3]) != frame {
			t.Errorf("%s has the frame %q, the others %q", name, m[3], frame)
		}
	}
	for name, c := range map[string]struct {
		data   []byte
		aspect float32
	}{"name": {nameSVG, nameAspect}, "name-light": {nameLightSVG, nameAspect}, "tagline": {taglineSVG, taglineAspect}, "tagline-light": {taglineLightSVG, taglineAspect}} {
		m := svgSize.FindSubmatch(c.data)
		if m == nil {
			t.Fatalf("%s is not an SVG with a size", name)
		}
		w, _ := strconv.ParseFloat(string(m[1]), 32)
		h, _ := strconv.ParseFloat(string(m[2]), 32)
		if got := float32(w / h); fmt.Sprintf("%.3f", got) != fmt.Sprintf("%.3f", c.aspect) {
			t.Errorf("%s is %v by %v (%.3f), stated as %.3f", name, w, h, got, c.aspect)
		}
	}
	for light, r := range map[bool]interface{ Content() []byte }{false: Mark(false), true: Mark(true)} {
		if len(r.Content()) < 1000 {
			t.Errorf("Mark(%v) is empty", light)
		}
	}
	if len(Icon.Content()) < 1000 {
		t.Error("the icon is empty")
	}
}
