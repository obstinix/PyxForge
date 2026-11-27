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

## The owner's choice and what was built

The owner kept the tint and chose to measure against the workspace actually behind the glass,
with text-dense content on opaque plates (D21 as revised). `TestGlassContrast` renders the
palette, Settings and a notification as glass over a busy editor with everything selected (and
the accent-filled Open Terminal button in the panel), at the Subtle blur (the least smoothing),
in a 1024×640 and a 1440×900 window, hides their content, and measures every pixel of the
glass inside the rounded corners. Worst contrast across the eight theme and accent
combinations:

| Surface, window | Primary | Secondary | Tertiary |
|---|---|---|---|
| Palette, 1024×640 | 9.70–14.23 | 5.19–7.77 | 3.56–4.70 |
| Settings, 1024×640 | 5.95–11.42 | 3.10–6.32 | 2.27–3.77 |
| Notification, 1024×640 | 5.66–11.96 | 2.90–6.53 | 2.15–3.95 |
| Palette, 1440×900 | 10.72–14.88 | 5.71–8.12 | 3.99–4.91 |
| Settings, 1440×900 | 10.45–13.21 | 5.49–7.21 | 3.83–4.36 |
| Notification, 1440×900 | 14.17–16.54 | 7.38–10.04 | 5.38–5.74 |

Only primary text passes everywhere. Secondary passed at 1440×900 but not at 1024×640, where the
accent button sits under Settings and the notifications; so only primary text sits on bare glass
(the Settings page list is all primary on glass, its selected page marked by its fill), and the
rest is on plates. Two measurement lessons: a first run counted pixels outside the rounded
corners, where the workspace shows unblurred, and reported failures that were not on the glass;
and one window size hid the secondary failure.

The test renders and blurs in software, which the race detector slows by about fifteen times
(373 s for the package's share), past CI's timeout. It starts no goroutines, so under `-race`
it skips and says so; CI's test job without the race detector runs it on every OS.

**Floating chrome (step 3b): PLANNED.** Tabs and breadcrumbs floating as glass over the editor
would put a blur on screen all the time, over text that scrolls under it every frame, and a
window with several glass surfaces at once was not measured. It is not built.
