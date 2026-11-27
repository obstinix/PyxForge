# Liquid Glass spike (D21, identity prompt Phase 3 step 0)

What Fyne 2.8.2 offers for a glass material, what it costs, and whether the material can meet
the contrast rule D21 sets. Measured on 2026-11-27. Tool: `go run ./tools/glassspike` (set
`PYXFORGE_GLASS_RADIUS` for another radius).

## What Fyne provides

| Feature | Finding |
|---|---|
| `canvas.Blur` (since 2.8) | A rectangular region that blurs everything drawn under it; `Radius` 1–50, `CornerRadius`. The GL painter draws it as a two-pass separable blur on the GPU (`internal/painter/gl/draw.go`, `drawBlur`), so it is live: what changes underneath is blurred on the next frame. The software painter (tests, `tools/snapshot`) draws it too (`internal/painter/software/draw.go`), so review renders show it. |
| `Canvas.Capture()` on the GL driver | Paints the frame and reads it back; it works on Windows 11 (used by the spike to time frames). Not needed for glass, since `canvas.Blur` is live. |
| `canvas.LinearGradient` | Present, and drawn by both painters; the rim's top-left to bottom-right fade can use it. |

## Frame cost (Windows 11, 1440×900 at 150 % scaling = 2160×1350 device pixels)

Machine: Intel Core Ultra 9 275HX, hybrid graphics (Intel Graphics and an NVIDIA GeForce RTX 5070
Laptop GPU). Which of the two drew the window was not determined; Windows usually gives a new
program the integrated GPU unless told otherwise.

Method: Fyne has no frame timer, so each number is the median time to paint and read back one
whole frame (`Canvas.Capture`) while a line of the editor-like scene under the glass changes
every frame, 120 frames after 10 of warm-up. The glass is a 640×420 surface (the command
palette's size) with a blur, a 62 % tint and a 1 px rim. The blur's cost is the difference of the
medians with and without the glass; readback is in both and cancels out.

| Blur radius | Frame without glass | Frame with glass | Cost of the glass |
|---|---|---|---|
| 12 (Subtle) | 7.25 ms | 7.46 ms | **0.21 ms** |
| 24 (Normal), three runs | 6.55 / 7.38 / 7.15 ms | 8.82 / 9.31 / 9.15 ms | **2.26 / 1.92 / 2.00 ms** |
| 40 (Strong) | 7.06 ms | 10.27 ms | **3.21 ms** |

All are under D21's 4 ms budget, so the live GPU blur is the method; the cached backdrop is not
needed. For the record, the fallback (crop a captured 960×630 region and run a three-pass box
blur in Go) takes **90 ms** median, far over budget at any refresh rate.

Not measured: Linux (WSL has no comparable GPU path here), macOS, the "floating chrome" variant
(tabs floating over the editor), and a window filled with more than one glass surface at once.

## Contrast: the rule and the tint cannot both hold

D21 requires text on glass to meet 4.5:1 (body) and 3:1 (large) over an all-white and an
all-black backdrop, in every theme. The prompt's tint is about 62 % on dark themes and 72 % on
light ones. With the Overlay surface as the tint, the opacity each text role needs is:

| Theme (worst backdrop) | Primary 4.5:1 | Secondary 4.5:1 | Tertiary 4.5:1 | Large text 3:1 |
|---|---|---|---|---|
| Smoked Kraft (white) | 0.68 | 0.84 | 0.95 | 0.55 |
| Verdigris Forge (white) | 0.67 | 0.79 | 0.98 | 0.54 |
| Monochrome (white) | 0.64 | 0.87 | 0.94 | 0.52 |
| Ink & Paper (black) | 0.52 | 0.71 | 0.90 | 0.41 |

At 62 % the dark themes fail even primary text; at 72 % Ink & Paper passes primary and secondary
but fails tertiary. The palette, notifications and hover boxes all use secondary or tertiary
text, so the recipe as written fails `TestGlassContrast` by construction. Work stopped here for
the owner's choice (identity prompt, stop-and-report).
