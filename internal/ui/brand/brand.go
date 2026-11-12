// Package brand carries PyxForge's identity: the forge mark, an anvil with the piece being
// worked above its face and one spark. Every file here is written by tools/appicon from one
// definition (tools/appicon/brand.go): the application icon, the mark for dark and light
// surfaces, and the mark's parts and the lettering the start-up screen animates. The Windows
// executable and installer carry the same icon as an .ico.
package brand

import (
	_ "embed"

	"fyne.io/fyne/v2"
)

//go:embed icon.png
var iconPNG []byte

//go:embed mark.png
var markPNG []byte

//go:embed mark-light.png
var markLightPNG []byte

//go:embed splash/anvil.svg
var anvilSVG []byte

//go:embed splash/anvil-light.svg
var anvilLightSVG []byte

//go:embed splash/pyx.svg
var pyxSVG []byte

//go:embed splash/pyx-light.svg
var pyxLightSVG []byte

//go:embed splash/spark.svg
var sparkSVG []byte

//go:embed splash/spark-light.svg
var sparkLightSVG []byte

//go:embed splash/name.svg
var nameSVG []byte

//go:embed splash/name-light.svg
var nameLightSVG []byte

//go:embed splash/tagline.svg
var taglineSVG []byte

//go:embed splash/tagline-light.svg
var taglineLightSVG []byte

// The lettering's proportions, as tools/appicon writes it (splash/name.svg, splash/tagline.svg).
const (
	nameAspect    = 401.0 / 72
	taglineAspect = 561.0 / 25
)

// Icon is the application icon, 256 pixels square: the mark on its charcoal tile.
var Icon fyne.Resource = fyne.NewStaticResource("pyxforge.png", iconPNG)

// Mark is the mark alone, 256 pixels square, for a dark surface or, with light, a light one.
func Mark(light bool) fyne.Resource {
	if light {
		return fyne.NewStaticResource("pyxforge-mark-light.png", markLightPNG)
	}
	return fyne.NewStaticResource("pyxforge-mark.png", markPNG)
}

// Parts are the mark's three shapes, each on the mark's own frame so that laid over each
// other they make the mark, and the name and the line under it as lettering.
type Parts struct {
	Anvil, Pyx, Spark fyne.Resource
	Name, Tagline     fyne.Resource
	NameAspect        float32 // the lettering's width over its height
	TaglineAspect     float32
}

// SplashParts are the pieces of the start-up screen for a dark surface or a light one.
func SplashParts(light bool) Parts {
	p := Parts{NameAspect: nameAspect, TaglineAspect: taglineAspect}
	res := func(name string, dark, lightData []byte) fyne.Resource {
		if light {
			return fyne.NewStaticResource("pyxforge-"+name+"-light.svg", lightData)
		}
		return fyne.NewStaticResource("pyxforge-"+name+".svg", dark)
	}
	p.Anvil, p.Pyx, p.Spark = res("anvil", anvilSVG, anvilLightSVG), res("pyx", pyxSVG, pyxLightSVG), res("spark", sparkSVG, sparkLightSVG)
	p.Name, p.Tagline = res("name", nameSVG, nameLightSVG), res("tagline", taglineSVG, taglineLightSVG)
	return p
}
