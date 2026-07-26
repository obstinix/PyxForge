---
name: Blueprint Doodle
colors:
  surface: '#07151f'
  surface-dim: '#07151f'
  surface-bright: '#2d3b47'
  surface-container-lowest: '#030f1a'
  surface-container-low: '#0f1d28'
  surface-container: '#13212c'
  surface-container-high: '#1e2b37'
  surface-container-highest: '#293642'
  on-surface: '#d6e4f4'
  on-surface-variant: '#bbc9cf'
  inverse-surface: '#d6e4f4'
  inverse-on-surface: '#24323e'
  outline: '#859398'
  outline-variant: '#3c494e'
  surface-tint: '#3cd7ff'
  primary: '#a8e8ff'
  on-primary: '#003642'
  primary-container: '#00d4ff'
  on-primary-container: '#00586b'
  inverse-primary: '#00677e'
  secondary: '#a7c8ff'
  on-secondary: '#003061'
  secondary-container: '#1f477b'
  on-secondary-container: '#93b6f1'
  tertiary: '#cce3e6'
  on-tertiary: '#1f3436'
  tertiary-container: '#b0c7ca'
  on-tertiary-container: '#3f5456'
  error: '#ffb4ab'
  on-error: '#690005'
  error-container: '#93000a'
  on-error-container: '#ffdad6'
  primary-fixed: '#b4ebff'
  primary-fixed-dim: '#3cd7ff'
  on-primary-fixed: '#001f27'
  on-primary-fixed-variant: '#004e5f'
  secondary-fixed: '#d5e3ff'
  secondary-fixed-dim: '#a7c8ff'
  on-secondary-fixed: '#001b3c'
  on-secondary-fixed-variant: '#1f477b'
  tertiary-fixed: '#d0e7ea'
  tertiary-fixed-dim: '#b4cbce'
  on-tertiary-fixed: '#091f21'
  on-tertiary-fixed-variant: '#364a4d'
  background: '#07151f'
  on-background: '#d6e4f4'
  surface-variant: '#293642'
typography:
  display-lg:
    fontFamily: Bricolage Grotesque
    fontSize: 48px
    fontWeight: '800'
    lineHeight: '1.1'
  headline-md:
    fontFamily: Bricolage Grotesque
    fontSize: 24px
    fontWeight: '600'
    lineHeight: '1.2'
  title-sm:
    fontFamily: Space Grotesk
    fontSize: 16px
    fontWeight: '700'
    lineHeight: '1.4'
  code-md:
    fontFamily: JetBrains Mono
    fontSize: 14px
    fontWeight: '400'
    lineHeight: '1.5'
  code-sm:
    fontFamily: JetBrains Mono
    fontSize: 12px
    fontWeight: '400'
    lineHeight: '1.4'
  label-caps:
    fontFamily: Space Grotesk
    fontSize: 11px
    fontWeight: '600'
    lineHeight: '1.2'
    letterSpacing: 0.05em
rounded:
  sm: 0.125rem
  DEFAULT: 0.25rem
  md: 0.375rem
  lg: 0.5rem
  xl: 0.75rem
  full: 9999px
spacing:
  grid-unit: 8px
  panel-gap: 16px
  sidebar-width: 280px
  safe-area: 24px
---

## Brand & Style
The design system for this desktop application balances the rigorous precision of systems engineering with the whimsical, tactile nature of a physical notebook sketch. It is designed for high-density technical workflows—reverse engineering, binary analysis, and low-level debugging—while mitigating "technical fatigue" through a playful, hand-drawn aesthetic.

The visual style is a hybrid of **Brutalism** (raw, honest line work and high-contrast grids) and **Tactile Sketching**. Every UI element should feel like it was drawn with a technical pencil or highlighted with a felt-tip marker on a drafting board. The goal is to evoke the feeling of a "mad scientist's" workspace—highly organized but full of character.

## Colors
The palette is rooted in the "Blueprint" aesthetic, utilizing high-contrast tones to ensure technical legibility.

- **Primary (Highlighter Cyan):** Used exclusively for active states, selected code blocks, and primary action "ink." It mimics a bright felt-tip highlighter.
- **Secondary (Blueprint Deep Blue):** The foundational canvas. All windows and panels are variations of this deep, matte blue.
- **Tertiary (Pencil White/Cyan):** A pale, slightly desaturated cyan used for "pencil" line work, borders, and secondary text.
- **Background Grid:** A subtle, repeating pattern of `#003D7A` (slightly lighter than the background) creating a 20px graph-paper grid across the entire workspace.
- **Syntax Highlighting:** Should use "Neon-Pastel" versions of standard colors (soft reds for errors, lime greens for successes) to pop against the deep blue.

## Typography
The typographic hierarchy distinguishes between "Interface Guidance" and "Technical Content."

- **Interface/Decorative:** Use **Bricolage Grotesque** for all headers and panel titles. Its eccentric, variable widths mimic the inconsistency of human handwriting while remaining professional and legible.
- **Technical/Data:** Use **JetBrains Mono** for all byte-level data, registers, disassembly, and terminal output. This is the "truth" layer of the UI and must remain perfectly aligned and monospaced.
- **UI Navigation:** Use **Space Grotesk** for menu items, buttons, and utility labels. Its geometric nature bridges the gap between the quirky headers and the rigid code font.

## Layout & Spacing
This design system utilizes a **Fixed Grid** desktop model. The main application window is divided into "Drafting Zones."

- **Panels:** Each functional area (e.g., Hex View, Register View) is a panel with a "hand-drawn" border. 
- **Dividers:** Horizontal and vertical dividers should not be straight lines; use a `Rough.js` style wobbly path with a `0.5px` stroke variation. 
- **Torn Edges:** The bottom edge of the "Terminal" or "Console" area should feature a "torn perforation" vector path, suggesting a page ripped from a technical manual.
- **Density:** Information density should be high. Use the 8px grid-unit for tight padding within technical tables, but maintain 24px of "sketch margins" around the edges of the main application window to prevent visual claustrophobia.

## Elevation & Depth
Depth is not communicated via traditional drop shadows, which would clash with the flat "paper" metaphor. Instead, elevation is achieved through:

1.  **Layer Stacking:** Active windows or pop-overs feature a "Shadow Stroke"—a second, thicker, darker blue line drawn 4px offset from the main border, mimicking a heavy pencil outline.
2.  **Opacity Blurs:** Background panels remain opaque, but tooltips and modals use a subtle "frosted blueprint" effect (backdrop-blur) to suggest they are floating above the drafting board.
3.  **The "Highlighter" Fill:** When a component is "raised" or focused, it receives a faint, semi-transparent Cyan wash (`#00D4FF` at 10% opacity) as if it were colored in with a broad marker.

## Shapes
The shape language is "Intentionally Imperfect." While the underlying hit-boxes are standard rectangles for usability, the visual rendering should avoid perfect 90-degree corners.

- **Borders:** Use a `Soft` roundedness (0.25rem) but apply a "pencil jitter" effect. The strokes should overlap slightly at the corners, resembling how a person draws a box by hand.
- **Buttons:** Primary buttons are outlined boxes with a single-color fill. The fill should not perfectly hit the edges of the border, leaving tiny "white gaps" typical of hand-coloring.
- **Icons:** Icons must be "single-line" style, appearing to be drawn with a 0.5pt technical pen. Avoid solid fills unless highlighting a state.

## Components
Consistent implementation of the "Blueprint Doodle" aesthetic requires specific component behaviors:

- **Buttons:** Rectangular with a "wobbly" stroke. On hover, the button's background fills with the Cyan Highlighter color. On click, the stroke weight increases from 1px to 2px.
- **Code Editor:** The background is the standard deep blue, but "Selected Text" uses a high-opacity Cyan background with ragged edges on the selection box, as if manually highlighted.
- **Input Fields:** A simple horizontal "hand-drawn" line instead of a full box, with a blinking "pencil tip" cursor.
- **Chips/Tabs:** Active tabs look like "Post-it" notes or taped-on labels. The "tape" effect is a semi-transparent rectangle at the top of the tab.
- **Schematic Symbols:** Small, non-functional doodles (resistors, capacitors, logic gates) should be placed in the corners of empty states or footer bars to reinforce the engineering notebook theme.
- **Scrollbars:** Ultra-thin "pencil lines" that thicken when hovered, avoiding the standard OS-native appearance to maintain immersion.