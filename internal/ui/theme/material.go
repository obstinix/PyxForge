package theme

import (
	"math"
	"sync/atomic"
)

// Material is what floating surfaces are made of (decision D21): Flat, the opaque Overlay
// surface (the default), or Liquid Glass, a blurred, tinted backdrop. It is chosen in Settings
// › Appearance and applies only to things that float over the workspace.
type Material int

const (
	Flat Material = iota
	Glass
)

// Blur radii for Settings › Appearance › Blur strength.
const (
	BlurSubtle float32 = 12
	BlurNormal float32 = 24
	BlurStrong float32 = 40
)

var (
	material   atomic.Int32
	blurRadius atomic.Uint32 // float32 bits
)

func init() { blurRadius.Store(math.Float32bits(BlurNormal)) }

// SetMaterial chooses the material of floating surfaces and the glass's blur radius. The
// shell calls it from the user's preferences; widgets read it when they refresh.
func SetMaterial(m Material, radius float32) {
	material.Store(int32(m))
	if radius > 0 {
		blurRadius.Store(math.Float32bits(radius))
	}
}

// CurrentMaterial is the material chosen.
func CurrentMaterial() Material { return Material(material.Load()) }

// GlassOn reports whether floating surfaces are Liquid Glass.
func GlassOn() bool { return CurrentMaterial() == Glass }

// GlassRadius is the glass's blur radius.
func GlassRadius() float32 { return math.Float32frombits(blurRadius.Load()) }
