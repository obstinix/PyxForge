package graphview

import (
	"image"
	"image/color"
	"math"
)

// The graph's own rasteriser (owner review F5). rasterx composites and clears a rasterizer the
// size of the whole image on every Draw, so filling 3,000 nodes one by one at 2880×1800 took
// minutes a frame. These primitives touch only the pixels a shape covers: a disc costs its
// area, a link its length times its width. All are anti-aliased by coverage and blended over
// the image.

// blend puts colour c at coverage a (0…1) over pixel (x, y), in integer arithmetic.
func blend(img *image.NRGBA, x, y int, c color.NRGBA, a float64) {
	if a <= 0 || x < img.Rect.Min.X || y < img.Rect.Min.Y || x >= img.Rect.Max.X || y >= img.Rect.Max.Y {
		return
	}
	k := int(a*float64(c.A)*256/255 + 0.5) // 0…256
	if k <= 0 {
		return
	}
	if k > 256 {
		k = 256
	}
	i := (y-img.Rect.Min.Y)*img.Stride + (x-img.Rect.Min.X)*4
	p := img.Pix[i : i+4 : i+4]
	inv := 256 - k
	p[0] = uint8((int(c.R)*k + int(p[0])*inv) >> 8)
	p[1] = uint8((int(c.G)*k + int(p[1])*inv) >> 8)
	p[2] = uint8((int(c.B)*k + int(p[2])*inv) >> 8)
	p[3] = 0xff
}

// clipBox is the part of a box inside the image, as pixel bounds.
func clipBox(img *image.NRGBA, x0, y0, x1, y1 float64) (int, int, int, int, bool) {
	b := img.Rect
	ix0, iy0 := max(int(math.Floor(x0)), b.Min.X), max(int(math.Floor(y0)), b.Min.Y)
	ix1, iy1 := min(int(math.Ceil(x1)), b.Max.X-1), min(int(math.Ceil(y1)), b.Max.Y-1)
	return ix0, iy0, ix1, iy1, ix0 <= ix1 && iy0 <= iy1
}

// fillDisc fills a disc of radius r about (cx, cy).
func fillDisc(img *image.NRGBA, cx, cy, r float64, c color.NRGBA) {
	x0, y0, x1, y1, ok := clipBox(img, cx-r-1, cy-r-1, cx+r+1, cy+r+1)
	if !ok {
		return
	}
	for y := y0; y <= y1; y++ {
		dy := float64(y) + 0.5 - cy
		for x := x0; x <= x1; x++ {
			dx := float64(x) + 0.5 - cx
			if cov := r + 0.5 - math.Sqrt(dx*dx+dy*dy); cov > 0 {
				blend(img, x, y, c, math.Min(cov, 1))
			}
		}
	}
}

// strokeRing draws a ring of width w about radius r.
func strokeRing(img *image.NRGBA, cx, cy, r, w float64, c color.NRGBA) {
	x0, y0, x1, y1, ok := clipBox(img, cx-r-w-1, cy-r-w-1, cx+r+w+1, cy+r+w+1)
	if !ok {
		return
	}
	for y := y0; y <= y1; y++ {
		dy := float64(y) + 0.5 - cy
		for x := x0; x <= x1; x++ {
			dx := float64(x) + 0.5 - cx
			d := math.Abs(math.Sqrt(dx*dx+dy*dy) - r)
			if cov := w/2 + 0.5 - d; cov > 0 {
				blend(img, x, y, c, math.Min(cov, 1))
			}
		}
	}
}

// fillRoundSquare fills a square of half side s about (cx, cy) with corner radius rad.
func fillRoundSquare(img *image.NRGBA, cx, cy, s, rad float64, c color.NRGBA) {
	x0, y0, x1, y1, ok := clipBox(img, cx-s-1, cy-s-1, cx+s+1, cy+s+1)
	if !ok {
		return
	}
	for y := y0; y <= y1; y++ {
		qy := math.Abs(float64(y)+0.5-cy) - (s - rad)
		for x := x0; x <= x1; x++ {
			qx := math.Abs(float64(x)+0.5-cx) - (s - rad)
			d := math.Hypot(math.Max(qx, 0), math.Max(qy, 0)) + math.Min(math.Max(qx, qy), 0) - rad
			if cov := 0.5 - d; cov > 0 {
				blend(img, x, y, c, math.Min(cov, 1))
			}
		}
	}
}

// drawLine draws a line of width w from (x0, y0) to (x1, y1), stepping along its major axis
// and filling a span across it at each step: its cost is its length times its width.
func drawLine(img *image.NRGBA, x0, y0, x1, y1, w float64, c color.NRGBA) {
	dx, dy := x1-x0, y1-y0
	length := math.Hypot(dx, dy)
	if length < 0.5 {
		return
	}
	// Clip to the image first, so a long link off screen costs nothing.
	b := img.Rect
	if !clipLine(&x0, &y0, &x1, &y1, float64(b.Min.X)-w, float64(b.Min.Y)-w, float64(b.Max.X)+w, float64(b.Max.Y)+w) {
		return
	}
	dx, dy = x1-x0, y1-y0
	if w <= 2.5 { // a thin link: two pixels a step
		wuLine(img, x0, y0, x1, y1, w, c)
		return
	}
	// Across the line, a pixel at minor-axis distance t from its centre is covered by
	// half-width hw (measured perpendicular), scaled by the slope.
	hw := w / 2
	if math.Abs(dx) >= math.Abs(dy) {
		if x0 > x1 {
			x0, x1, y0, y1 = x1, x0, y1, y0
		}
		slope := (y1 - y0) / math.Max(x1-x0, 1e-9)
		span := hw * math.Sqrt(1+slope*slope)
		for x := int(math.Floor(x0)); x <= int(math.Ceil(x1)); x++ {
			cy := y0 + (float64(x)+0.5-x0)*slope
			for y := int(math.Floor(cy - span - 1)); y <= int(math.Ceil(cy+span+1)); y++ {
				if cov := span + 0.5 - math.Abs(float64(y)+0.5-cy); cov > 0 {
					blend(img, x, y, c, math.Min(cov, 1))
				}
			}
		}
		return
	}
	if y0 > y1 {
		x0, x1, y0, y1 = x1, x0, y1, y0
	}
	slope := (x1 - x0) / math.Max(y1-y0, 1e-9)
	span := hw * math.Sqrt(1+slope*slope)
	for y := int(math.Floor(y0)); y <= int(math.Ceil(y1)); y++ {
		cx := x0 + (float64(y)+0.5-y0)*slope
		for x := int(math.Floor(cx - span - 1)); x <= int(math.Ceil(cx+span+1)); x++ {
			if cov := span + 0.5 - math.Abs(float64(x)+0.5-cx); cov > 0 {
				blend(img, x, y, c, math.Min(cov, 1))
			}
		}
	}
}

// wuLine draws a thin line (Xiaolin Wu): two pixels a step, each covered by its distance from
// the line, scaled by the line's width.
func wuLine(img *image.NRGBA, x0, y0, x1, y1, w float64, c color.NRGBA) {
	steep := math.Abs(y1-y0) > math.Abs(x1-x0)
	if steep {
		x0, y0, x1, y1 = y0, x0, y1, x1
	}
	if x0 > x1 {
		x0, x1, y0, y1 = x1, x0, y1, y0
	}
	grad := (y1 - y0) / math.Max(x1-x0, 1e-9)
	a := math.Min(w, 1)
	plot := func(x, y int, cov float64) {
		if steep {
			blend(img, y, x, c, cov*a)
		} else {
			blend(img, x, y, c, cov*a)
		}
	}
	y := y0 + grad*(math.Round(x0)-x0)
	for x := int(math.Round(x0)); x <= int(math.Round(x1)); x++ {
		fy := math.Floor(y)
		f := y - fy
		plot(x, int(fy), 1-f)
		plot(x, int(fy)+1, f)
		y += grad
	}
}

// clipLine clips a segment to a box (Liang–Barsky); false when nothing is inside.
func clipLine(x0, y0, x1, y1 *float64, minX, minY, maxX, maxY float64) bool {
	t0, t1 := 0.0, 1.0
	dx, dy := *x1-*x0, *y1-*y0
	for _, e := range [4][2]float64{{-dx, *x0 - minX}, {dx, maxX - *x0}, {-dy, *y0 - minY}, {dy, maxY - *y0}} {
		p, q := e[0], e[1]
		if p == 0 {
			if q < 0 {
				return false
			}
			continue
		}
		r := q / p
		if p < 0 {
			if r > t1 {
				return false
			}
			t0 = math.Max(t0, r)
		} else {
			if r < t0 {
				return false
			}
			t1 = math.Min(t1, r)
		}
	}
	nx0, ny0 := *x0+t0*dx, *y0+t0*dy
	*x1, *y1 = *x0+t1*dx, *y0+t1*dy
	*x0, *y0 = nx0, ny0
	return true
}

// fillTriangle fills a triangle (an arrowhead) by testing the pixels of its box.
func fillTriangle(img *image.NRGBA, ax, ay, bx, by, cx, cy float64, c color.NRGBA) {
	x0, y0, x1, y1, ok := clipBox(img, math.Min(ax, math.Min(bx, cx)), math.Min(ay, math.Min(by, cy)),
		math.Max(ax, math.Max(bx, cx)), math.Max(ay, math.Max(by, cy)))
	if !ok {
		return
	}
	edge := func(px, py, qx, qy, x, y float64) float64 { return (qx-px)*(y-py) - (qy-py)*(x-px) }
	area := edge(ax, ay, bx, by, cx, cy)
	if area == 0 {
		return
	}
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			px, py := float64(x)+0.5, float64(y)+0.5
			w0, w1, w2 := edge(bx, by, cx, cy, px, py), edge(cx, cy, ax, ay, px, py), edge(ax, ay, bx, by, px, py)
			inside := w0 >= 0 && w1 >= 0 && w2 >= 0
			if area < 0 {
				inside = w0 <= 0 && w1 <= 0 && w2 <= 0
			}
			if inside {
				blend(img, x, y, c, 1)
			}
		}
	}
}
