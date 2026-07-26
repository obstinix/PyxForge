---
name: Ink & Glass
colors:
  surface: '#f9f9f9'
  surface-dim: '#dadada'
  surface-bright: '#f9f9f9'
  surface-container-lowest: '#ffffff'
  surface-container-low: '#f3f3f4'
  surface-container: '#eeeeee'
  surface-container-high: '#e8e8e8'
  surface-container-highest: '#e2e2e2'
  on-surface: '#1a1c1c'
  on-surface-variant: '#4c4546'
  inverse-surface: '#2f3131'
  inverse-on-surface: '#f0f1f1'
  outline: '#7e7576'
  outline-variant: '#cfc4c5'
  surface-tint: '#5e5e5e'
  primary: '#000000'
  on-primary: '#ffffff'
  primary-container: '#1b1b1b'
  on-primary-container: '#848484'
  inverse-primary: '#c6c6c6'
  secondary: '#5e5f5d'
  on-secondary: '#ffffff'
  secondary-container: '#e0e0dd'
  on-secondary-container: '#626361'
  tertiary: '#000000'
  on-tertiary: '#ffffff'
  tertiary-container: '#1c1b1b'
  on-tertiary-container: '#858383'
  error: '#ba1a1a'
  on-error: '#ffffff'
  error-container: '#ffdad6'
  on-error-container: '#93000a'
  primary-fixed: '#e2e2e2'
  primary-fixed-dim: '#c6c6c6'
  on-primary-fixed: '#1b1b1b'
  on-primary-fixed-variant: '#474747'
  secondary-fixed: '#e3e2e0'
  secondary-fixed-dim: '#c7c6c4'
  on-secondary-fixed: '#1a1c1a'
  on-secondary-fixed-variant: '#464745'
  tertiary-fixed: '#e5e2e1'
  tertiary-fixed-dim: '#c8c6c5'
  on-tertiary-fixed: '#1c1b1b'
  on-tertiary-fixed-variant: '#474746'
  background: '#f9f9f9'
  on-background: '#1a1c1c'
  surface-variant: '#e2e2e2'
typography:
  wordmark:
    fontFamily: Yellowtail
    fontSize: 32px
    fontWeight: '400'
    lineHeight: '1.2'
  nav-title:
    fontFamily: Unbounded
    fontSize: 14px
    fontWeight: '600'
    lineHeight: 20px
    letterSpacing: 0.05em
  code-base:
    fontFamily: JetBrains Mono
    fontSize: 14px
    fontWeight: '400'
    lineHeight: 24px
  annotation:
    fontFamily: Caveat
    fontSize: 18px
    fontWeight: '500'
    lineHeight: '1.1'
  label-sm:
    fontFamily: JetBrains Mono
    fontSize: 11px
    fontWeight: '500'
    lineHeight: 16px
rounded:
  sm: 0.25rem
  DEFAULT: 0.5rem
  md: 0.75rem
  lg: 1rem
  xl: 1.5rem
  full: 9999px
spacing:
  unit: 4px
  gutter: 24px
  margin: 32px
  panel-padding: 16px
---

## Brand & Style

The design system is built on the contrast between raw, tactile analog medium and advanced digital transparency. It is an "Ink & Paper" aesthetic augmented by "Liquid-glass" overlays. The goal is to evoke the feeling of a master developer’s high-tech workspace cluttered with hand-drawn napkin sketches.

The visual style merges **Glassmorphism** and **Brutalism**. It utilizes the airy, frosted nature of modern glass interfaces but grounds them with the high-contrast, black-on-warm-white palette of a vintage sketchbook. Micro-motifs—such as wobbly gears, coffee rings, and hand-sketched bugs—humanize the technical environment, creating a workspace that feels creative rather than corporate.

Key aesthetic pillars:
- **Imperfect Precision:** Clean typography paired with hand-drawn, "wobbly" iconography.
- **Layered Tactility:** Frosted surfaces floating over a textured paper base.
- **Focus through Contrast:** Pure black ink ensures maximum readability against the warmth of the off-white paper.

## Colors

This design system uses a strict monochromatic base with specific theme-swatch accents located only in the global title bar.

- **Background:** A warm, textured off-white (#FAF9F6) mimicking premium heavy-weight paper.
- **Ink (Text/Primary):** Pure black (#000000). Used for all body text, primary icons, and doodle linework.
- **Glass (Panels):** A semi-transparent white (#FFFFFF80) with a heavy backdrop-blur (20px-32px). Every glass panel must feature a 1px top-edge specular highlight (pure white) to define the "thickness" of the material.
- **Accent (Selected):** Charcoal Black (#1A1A1A) is used for active states, giving a deep, recessed "pressed" look compared to the flat ink lines.

**Theme Selector:** A dedicated 4-swatch row is positioned in the title bar. These colors (Signal Yellow, Verdigris, etc.) are only to be used as UI indicators (active status pips or theme toggles) and never for large surface areas.

## Typography

Typography in this design system serves three distinct roles: the **Formal**, the **Technical**, and the **Human**.

1.  **The Formal (Unbounded):** Used for panel headers and navigation. This geometric sans-serif provides a rigid structure that offsets the fluid doodles.
2.  **The Technical (JetBrains Mono):** This is the workhorse. All functional data, logs, code, and UI labels use this monospaced font to maintain a sense of precision and alignment.
3.  **The Human (Yellowtail & Caveat):** The wordmark uses Yellowtail for a hand-lettered "craftsman" signature. Tooltips and side-notes use Caveat, appearing as if the developer scribbled a reminder directly onto the glass.

For mobile layouts, `nav-title` scales down to 12px, while `annotation` maintains its size to ensure the handwritten legibility remains clear.

## Layout & Spacing

The layout utilizes a **Fluid Grid** model with high-density spacing within panels, contrasted by wide margins between panels to allow the background "paper" to breathe.

- **Breakpoints:** Mobile (<768px), Tablet (768px-1200px), Desktop (>1200px).
- **Desktop:** A 12-column system. Panels float with 24px gutters. 
- **The "Glass" Margin:** Panels should never touch the edge of the screen; a minimum 32px safety margin of "paper" must always be visible.
- **Inner Density:** Inside glass panels (terminal, editor), spacing is tight (8px or 16px) to maximize information density, following the JetBrains Mono vertical rhythm.

## Elevation & Depth

Depth is achieved through **Stacking and Blurs** rather than traditional drop shadows.

- **Level 0 (Base):** The #FAF9F6 paper texture. It is static and flat.
- **Level 1 (Glass Panels):** Frosted surfaces (#FFFFFF80) with a `backdrop-filter: blur(20px)`. They cast a very soft, large-radius shadow (10% opacity black) to suggest they are hovering 1-2 inches above the paper.
- **Level 2 (Active Overlays):** Modals or pop-overs. These use a slightly more opaque white (#FFFFF2) and a more pronounced shadow to indicate higher elevation.
- **Etching:** Doodle motifs are "etched" into the glass panels using a `multiply` blend mode at 80% opacity, making them look like part of the physical material.

## Shapes

The design system employs **Softly Rounded** corners for all structural containers.

- **Panels:** Use a 1rem (16px) radius to create a "liquid" feel that matches the glass metaphor.
- **Buttons/Inputs:** Use 0.5rem (8px) for a slightly more defined, clickable appearance.
- **Doodles:** Should never be perfectly circular or straight. Use SVG paths with slight "wobble" transforms or hand-drawn path data. Lines should have varying stroke weights (1.5px to 2.5px) to mimic physical ink pressure.

## Components

### Buttons
Primary buttons are solid Charcoal (#1A1A1A) with JetBrains Mono text in white. Secondary buttons are transparent glass frames with a 1px black ink border. All buttons should have a slight "wiggle" animation on hover.

### Input Fields
Inputs are semi-opaque glass with a 1px bottom-border only (ink style). The cursor should be a solid black block, staying true to the terminal aesthetic.

### Cards & Panels
Every card is a glass panel. They must include a "Doodle Motif" in the top right or bottom left corner (e.g., a small gear or coffee cup icon) that appears hand-drawn.

### Tooltips
Tooltips look like paper scraps. They use the `Caveat` font and have a slight 2-degree rotation to look like they were taped onto the screen.

### Terminal & Code Editor
These are the only "opaque" areas. They use a slightly darker version of the paper or a very thick frosted glass to ensure no background "clutter" interferes with code legibility.