// Command glassspike measures what Liquid Glass (decision D21) would cost before any of it is
// built: Fyne's GPU blur (canvas.Blur, Fyne 2.8) under a floating surface, and the fallback, a
// captured backdrop blurred on the CPU. It opens a real window (the GL driver), draws a busy
// editor-like scene at 1440×900 and reports medians.
//
//	go run ./tools/glassspike
//
// Fyne has no frame timer, so the GPU number is a difference: the time to paint and read back a
// whole frame (Canvas.Capture) with the blurred overlay, less the time without it, both while the
// scene under the overlay changes every frame. Readback is in both and cancels out.
package main

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"math/rand"
	"os"
	"sort"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
)

const (
	winW, winH    = 1440, 900
	glassW        = 640 // the command palette's width
	glassH        = 420
	defaultRadius = 24
	frames        = 120
)

// radius is the blur radius measured: PYXFORGE_GLASS_RADIUS, else 24 (Normal).
var radius = func() float32 {
	var r float32 = defaultRadius
	if v := os.Getenv("PYXFORGE_GLASS_RADIUS"); v != "" {
		fmt.Sscan(v, &r)
	}
	return r
}()

func main() {
	a := app.NewWithID("io.github.obstinix.pyxforge.glassspike")
	w := a.NewWindow("PyxForge glass spike")
	w.SetPadded(false)

	// The scene: 45 lines of coloured "code" over a dark editor, which changes every frame.
	bg := canvas.NewRectangle(color.NRGBA{0x1c, 0x19, 0x16, 0xff})
	var lines []*canvas.Text
	scene := container.NewWithoutLayout(bg)
	r := rand.New(rand.NewSource(1))
	words := []string{"mov", "ax,", "0x7c00", "int", "0x10", "call", "serial_put", "jmp", ".next", "; comment", "lodsb", "test", "al,", "al"}
	line := func() string {
		var b strings.Builder
		for range 8 + r.Intn(10) {
			b.WriteString(words[r.Intn(len(words))] + " ")
		}
		return b.String()
	}
	for i := range 45 {
		t := canvas.NewText(line(), color.NRGBA{uint8(150 + r.Intn(100)), uint8(150 + r.Intn(100)), uint8(150 + r.Intn(100)), 0xff})
		t.TextStyle.Monospace = true
		t.TextSize = 13
		t.Move(fyne.NewPos(60, float32(i)*19+8))
		t.Resize(fyne.NewSize(1300, 18))
		lines = append(lines, t)
		scene.Add(t)
	}
	bg.Resize(fyne.NewSize(winW, winH))

	// The floating surface: blur, a translucent tint, a rim.
	gx, gy := float32(winW-glassW)/2, float32(90)
	blur := canvas.NewBlur(radius)
	blur.CornerRadius = 8
	blur.Move(fyne.NewPos(gx, gy))
	blur.Resize(fyne.NewSize(glassW, glassH))
	tint := canvas.NewRectangle(color.NRGBA{0x22, 0x1e, 0x1a, 158}) // about 62 %
	tint.CornerRadius = 8
	tint.StrokeColor, tint.StrokeWidth = color.NRGBA{255, 255, 255, 90}, 1
	tint.Move(blur.Position())
	tint.Resize(blur.Size())
	overlay := container.NewWithoutLayout(blur, tint)

	w.SetContent(container.NewStack(scene, overlay))
	w.Resize(fyne.NewSize(winW, winH))
	w.SetFixedSize(true)

	report := &strings.Builder{}
	go func() {
		time.Sleep(1500 * time.Millisecond) // shown and settled
		measure := func(name string, withGlass bool) time.Duration {
			fyne.DoAndWait(func() {
				overlay.Hidden = !withGlass
				overlay.Refresh()
			})
			var ds []time.Duration
			for i := range frames + 10 {
				var d time.Duration
				fyne.DoAndWait(func() {
					lines[i%len(lines)].Text = line() // the content under the glass changes
					lines[i%len(lines)].Refresh()
					start := time.Now()
					_ = w.Canvas().Capture()
					d = time.Since(start)
				})
				if i >= 10 { // warm-up
					ds = append(ds, d)
				}
			}
			m := median(ds)
			fmt.Fprintf(report, "%-34s median %6.2f ms  p90 %6.2f ms  (%d frames)\n", name, ms(m), ms(pct(ds, 0.9)), len(ds))
			return m
		}
		var img image.Image
		fyne.DoAndWait(func() { img = w.Canvas().Capture() })
		scale := float32(img.Bounds().Dx()) / winW
		fmt.Fprintf(report, "window %dx%d, %d×%d device pixels (scale %.2f)\n", winW, winH, img.Bounds().Dx(), img.Bounds().Dy(), scale)
		flat := measure("frame without glass", false)
		glass := measure(fmt.Sprintf("frame with canvas.Blur r=%g + tint", radius), true)
		fmt.Fprintf(report, "canvas.Blur cost per frame: %.2f ms (difference of medians)\n", ms(glass-flat))

		// The fallback: crop the captured frame, three-pass box blur on the CPU.
		var cpu []time.Duration
		sx, sy := int(gx*scale), int(gy*scale)
		rect := image.Rect(sx, sy, sx+int(glassW*scale), sy+int(glassH*scale))
		for range 40 {
			start := time.Now()
			crop := image.NewNRGBA(image.Rect(0, 0, rect.Dx(), rect.Dy()))
			draw.Draw(crop, crop.Bounds(), img, rect.Min, draw.Src)
			boxBlur3(crop, int(radius*scale))
			cpu = append(cpu, time.Since(start))
		}
		fmt.Fprintf(report, "%-34s median %6.2f ms  p90 %6.2f ms  (%d runs, %dx%d px)\n", "CPU crop + 3-pass box blur", ms(median(cpu)), ms(pct(cpu, 0.9)), len(cpu), rect.Dx(), rect.Dy())
		fmt.Print(report.String())
		if len(os.Args) > 1 {
			os.WriteFile(os.Args[1], []byte(report.String()), 0o644)
		}
		fyne.Do(a.Quit)
	}()
	w.ShowAndRun()
}

// boxBlur3 approximates a Gaussian of radius r with three box blurs, each horizontal then
// vertical, using running sums: O(pixels) per pass whatever the radius.
func boxBlur3(img *image.NRGBA, r int) {
	box := max(r/3, 1)
	tmp := image.NewNRGBA(img.Bounds())
	for range 3 {
		boxPass(img, tmp, box, true)
		boxPass(tmp, img, box, false)
	}
}

func boxPass(src, dst *image.NRGBA, r int, horizontal bool) {
	w, h := src.Bounds().Dx(), src.Bounds().Dy()
	n, m := h, w
	if !horizontal {
		n, m = w, h
	}
	at := func(i, j int) int { // pixel j along line i
		if horizontal {
			return i*src.Stride + j*4
		}
		return j*src.Stride + i*4
	}
	div := 2*r + 1
	for i := range n {
		var s [4]int
		for k := -r; k <= r; k++ {
			p := at(i, max(0, min(k, m-1)))
			for c := range 4 {
				s[c] += int(src.Pix[p+c])
			}
		}
		for j := range m {
			o := at(i, j)
			for c := range 4 {
				dst.Pix[o+c] = uint8(s[c] / div)
			}
			add, sub := at(i, min(j+r+1, m-1)), at(i, max(j-r, 0))
			for c := range 4 {
				s[c] += int(src.Pix[add+c]) - int(src.Pix[sub+c])
			}
		}
	}
}

func median(ds []time.Duration) time.Duration { return pct(ds, 0.5) }

func pct(ds []time.Duration, p float64) time.Duration {
	s := append([]time.Duration(nil), ds...)
	sort.Slice(s, func(i, j int) bool { return s[i] < s[j] })
	return s[int(float64(len(s)-1)*p)]
}

func ms(d time.Duration) float64 { return float64(d.Microseconds()) / 1000 }
