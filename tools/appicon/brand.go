package main

import (
	"fmt"
	"os"
	"strings"

	"golang.org/x/image/font"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
)

// PyxForge's identity, drawn once.
//
// The mark is an anvil with the piece being worked above its face: an anvil (horn to the
// left, face, waist, foot), a diamond, the "pyx", struck above it, and one spark. Three flat
// shapes on a 256-unit square, no gradients, so it holds from a 16-pixel taskbar icon to a
// start-up screen and reads in a single colour.
//
// Every asset below is written from these shapes, so there is one logo, not several.
const (
	anvilPath = "M 40 118 L 98 104 L 216 104 L 216 132 L 178 132 C 164 132 154 142 154 156 L 154 166 L 188 180 L 188 204 L 68 204 L 68 180 L 102 166 L 102 156 C 102 144 94 136 82 134 C 66 131 52 126 40 118 Z"
	pyxPath   = "M 134 40 L 160 66 L 134 92 L 108 66 Z"
	sparkPath = "M 186 46 L 196 56 L 186 66 L 176 56 Z"
)

// The identity's colours: deep charcoal, off-white, a muted crimson and a restrained amber.
const (
	charcoal   = "#1b1815"
	tileEdge   = "#3a342d"
	offWhite   = "#ece9e2"
	crimson    = "#d2475a" // on charcoal
	amber      = "#d9a05b"
	ink        = "#1b1815" // the anvil on a light background
	darkRed    = "#a8283b" // the pyx on a light background
	darkAmber  = "#a86a1f"
	wordmark   = "PyxForge"
	markFolder = "internal/ui/brand/"
)

// markShapes is the mark in three colours, as SVG path elements.
func markShapes(anvil, pyx, spark string) string {
	return fmt.Sprintf("  <path fill=%q d=%q/>\n  <path fill=%q d=%q/>\n  <path fill=%q d=%q/>\n",
		anvil, anvilPath, pyx, pyxPath, spark, sparkPath)
}

// iconSVG is the application icon: the mark on a charcoal tile. small drops the spark and
// the tile's edge, which only smear at 16 to 24 pixels.
func iconSVG(small bool) string {
	var b strings.Builder
	b.WriteString(`<svg xmlns="http://www.w3.org/2000/svg" width="256" height="256" viewBox="0 0 256 256">` + "\n")
	b.WriteString("  <!-- Written by tools/appicon (brand.go): PyxForge's application icon. Do not edit by hand. -->\n")
	if small {
		fmt.Fprintf(&b, "  <rect x=\"4\" y=\"4\" width=\"248\" height=\"248\" rx=\"56\" ry=\"56\" fill=%q/>\n", charcoal)
		fmt.Fprintf(&b, "  <path fill=%q d=%q/>\n  <path fill=%q d=%q/>\n", offWhite, anvilPath, crimson, pyxPath)
	} else {
		fmt.Fprintf(&b, "  <rect x=\"8\" y=\"8\" width=\"240\" height=\"240\" rx=\"54\" ry=\"54\" fill=%q stroke=%q stroke-width=\"4\"/>\n", charcoal, tileEdge)
		b.WriteString(markShapes(offWhite, crimson, amber))
	}
	b.WriteString("</svg>\n")
	return b.String()
}

// markSVG is the mark alone, cropped to its shapes, for a dark or a light background or in
// one colour.
func markSVG(what, anvil, pyx, spark string) string {
	return `<svg xmlns="http://www.w3.org/2000/svg" width="256" height="256" viewBox="24 24 208 208">` + "\n" +
		"  <!-- Written by tools/appicon (brand.go): PyxForge's mark, " + what + ". Do not edit by hand. -->\n" +
		markShapes(anvil, pyx, spark) + "</svg>\n"
}

// logoSVG is the primary logo: the mark and the name, set in the display face and drawn as
// outlines, so the file needs no font where it is shown.
func logoSVG(what string, face []byte, anvil, pyx, spark, text string) (string, error) {
	const size, baseline, left = 104.0, 168.0, 238.0
	path, width, err := textOutline(face, wordmark, size, left, baseline, -1.5)
	if err != nil {
		return "", err
	}
	w := int(left + width + 28)
	var b strings.Builder
	fmt.Fprintf(&b, "<svg xmlns=\"http://www.w3.org/2000/svg\" width=\"%d\" height=\"232\" viewBox=\"0 0 %d 232\">\n", w, w)
	b.WriteString("  <!-- Written by tools/appicon (brand.go): PyxForge's logo, " + what + ". Do not edit by hand. -->\n")
	b.WriteString("  <g transform=\"translate(-12 -12)\">\n" + markShapes(anvil, pyx, spark) + "  </g>\n")
	fmt.Fprintf(&b, "  <path fill=%q d=%q/>\n</svg>\n", text, path)
	return b.String(), nil
}

// textOutline sets text in a TrueType face at size, from (x, baseline), with tracking added
// between letters, and returns its outline as SVG path data and its width.
func textOutline(face []byte, text string, size, x, baseline, tracking float64) (string, float64, error) {
	f, err := sfnt.Parse(face)
	if err != nil {
		return "", 0, err
	}
	var buf sfnt.Buffer
	ppem := fixed.Int26_6(size * 64)
	var b strings.Builder
	pen := x
	pt := func(p fixed.Point26_6) string {
		return fmt.Sprintf("%.2f %.2f", pen+float64(p.X)/64, baseline+float64(p.Y)/64)
	}
	var prev sfnt.GlyphIndex
	for i, r := range text {
		g, err := f.GlyphIndex(&buf, r)
		if err != nil || g == 0 {
			return "", 0, fmt.Errorf("the face has no glyph for %q", r)
		}
		if i > 0 {
			if k, err := f.Kern(&buf, prev, g, ppem, font.HintingNone); err == nil {
				pen += float64(k) / 64
			}
			pen += tracking
		}
		segs, err := f.LoadGlyph(&buf, g, ppem, nil)
		if err != nil {
			return "", 0, err
		}
		for _, s := range segs {
			switch s.Op {
			case sfnt.SegmentOpMoveTo:
				b.WriteString("M " + pt(s.Args[0]) + " ")
			case sfnt.SegmentOpLineTo:
				b.WriteString("L " + pt(s.Args[0]) + " ")
			case sfnt.SegmentOpQuadTo:
				b.WriteString("Q " + pt(s.Args[0]) + " " + pt(s.Args[1]) + " ")
			case sfnt.SegmentOpCubeTo:
				b.WriteString("C " + pt(s.Args[0]) + " " + pt(s.Args[1]) + " " + pt(s.Args[2]) + " ")
			}
		}
		b.WriteString("Z ")
		adv, err := f.GlyphAdvance(&buf, g, ppem, font.HintingNone)
		if err != nil {
			return "", 0, err
		}
		pen += float64(adv) / 64
		prev = g
	}
	return strings.TrimSpace(b.String()), pen - x, nil
}

// partSVG is one shape of the mark on the mark's own frame, so the parts laid over each other
// make the mark: the start-up screen moves them separately.
func partSVG(fill, d string) string {
	return `<svg xmlns="http://www.w3.org/2000/svg" width="256" height="256" viewBox="24 24 208 208">` + "\n" +
		"  <!-- Written by tools/appicon (brand.go): a part of PyxForge's mark. Do not edit by hand. -->\n" +
		fmt.Sprintf("  <path fill=%q d=%q/>\n</svg>\n", fill, d)
}

// lineSVG is a line of text as outlines, letter-spaced, cropped to its own box: the name and
// the line under it on the start-up screen, which Fyne's own text cannot letter-space.
func lineSVG(face []byte, text string, size, tracking float64, fill string) (string, error) {
	path, width, err := textOutline(face, text, size, 0, size*0.86, tracking)
	if err != nil {
		return "", err
	}
	w, h := int(width+1), int(size*1.12+1)
	return fmt.Sprintf("<svg xmlns=\"http://www.w3.org/2000/svg\" width=\"%d\" height=\"%d\" viewBox=\"0 0 %d %d\">\n"+
		"  <!-- Written by tools/appicon (brand.go): %q as outlines. Do not edit by hand. -->\n"+
		"  <path fill=%q d=%q/>\n</svg>\n", w, h, w, h, text, fill, path), nil
}

// Start-up screen text.
const (
	splashName    = "PYXFORGE"
	splashTagline = "A NATIVE DEVELOPMENT ENVIRONMENT"
	taglineFace   = "internal/ui/theme/fonts/Geist-Medium.ttf"
)

// brandSVGs are the vector assets, by path.
func brandSVGs(displayFace string) (map[string]string, error) {
	face, err := os.ReadFile(displayFace)
	if err != nil {
		return nil, err
	}
	medium, err := os.ReadFile(taglineFace)
	if err != nil {
		return nil, err
	}
	splash := map[string]string{}
	for _, v := range []struct{ suffix, anvil, pyx, spark, text, quiet string }{
		{"", offWhite, crimson, amber, offWhite, "#a8a299"},
		{"-light", ink, darkRed, darkAmber, ink, "#5f5a52"},
	} {
		splash["anvil"+v.suffix] = partSVG(v.anvil, anvilPath)
		splash["pyx"+v.suffix] = partSVG(v.pyx, pyxPath)
		splash["spark"+v.suffix] = partSVG(v.spark, sparkPath)
		if splash["name"+v.suffix], err = lineSVG(face, splashName, 64, 9, v.text); err != nil {
			return nil, err
		}
		if splash["tagline"+v.suffix], err = lineSVG(medium, splashTagline, 22, 4.4, v.quiet); err != nil {
			return nil, err
		}
	}
	dark, err := logoSVG("for a dark background", face, offWhite, crimson, amber, offWhite)
	if err != nil {
		return nil, err
	}
	light, err := logoSVG("for a light background", face, ink, darkRed, darkAmber, ink)
	if err != nil {
		return nil, err
	}
	mono, err := logoSVG("in one colour", face, "#000000", "#000000", "#000000", "#000000")
	if err != nil {
		return nil, err
	}
	out := map[string]string{}
	for name, svg := range splash {
		out[markFolder+"splash/"+name+".svg"] = svg
	}
	for path, svg := range map[string]string{
		markFolder + "icon.svg":       iconSVG(false),
		markFolder + "icon-small.svg": iconSVG(true),
		markFolder + "mark.svg":       markSVG("for a dark background", offWhite, crimson, amber),
		markFolder + "mark-light.svg": markSVG("for a light background", ink, darkRed, darkAmber),
		markFolder + "mark-mono.svg":  markSVG("in one colour", "#000000", "#000000", "#000000"),
		markFolder + "logo.svg":       dark,
		markFolder + "logo-light.svg": light,
		markFolder + "logo-mono.svg":  mono,
	} {
		out[path] = svg
	}
	return out, nil
}
