# PyxForge Multi-Theme System (owner requirement)

Source: owner's answer to Decision D2, given 2026-10-08. Recorded verbatim below the line.
It supersedes the D2 recommended default in the master prompt and Section 7's "at most 3
themes in 3.0". Open questions it raises are tracked in `docs/architecture/DECISIONS.md`.

---

Do NOT select only one of the proposed visual directions.

All proposed visual systems are valid and must become first-class user-selectable PyxForge themes.

The user should be able to choose their preferred visual identity from the application's Settings / Appearance system.

## Required themes

Implement at minimum:

### 1. Smoked Kraft

A warm, dark, dense desktop surface system.

Characteristics:

* smoked kraft surfaces
* layered dark panels
* high information density
* warm neutral undertones
* restrained accent
* excellent for long development sessions

This should be one of the primary dark themes.

---

### 2. Ink & Paper

A light editorial surface system.

Characteristics:

* ink-like text
* paper-like background
* subtle surface elevation
* strong readability
* restrained borders
* excellent long-session readability

This should be the primary light theme.

---

### 3. Ink & Glass

A premium translucent visual system.

Characteristics:

* dark ink foundation
* translucent surfaces
* subtle blur where the native platform supports it
* elevated overlays
* refined borders
* restrained translucency

IMPORTANT:

Do not make the entire application excessively translucent.

Ink & Glass should use glass selectively for:

* command palette
* dialogs
* workspace switcher
* floating inspector
* notifications
* onboarding
* agent overlays

Core editor and terminal surfaces must remain highly readable.

---

### 4. Verdigris Forge

A tinted industrial/technical dark theme.

Characteristics:

* verdigris/forge-inspired dark surfaces
* muted green/teal undertones
* technical atmosphere
* high contrast
* restrained saturation
* systems-programming character

Do not turn this into a neon cyberpunk theme.

---

### 5. Monochrome + Crimson

A minimal PyxForge identity.

Characteristics:

* near-black / white / gray foundation
* restrained crimson accent
* minimal decoration
* high contrast
* extremely clean information hierarchy

Do not introduce gradients merely to make the monochrome theme more interesting.

---

# ACCENT SYSTEM

Accents must be independent semantic tokens.

Do NOT hard-code crimson directly into individual components.

Create tokens such as:

```text
accent.primary
accent.hover
accent.active
accent.muted
accent.focus
accent.selection
accent.border
```

At minimum provide:

### Crimson

PyxForge's primary signature accent.

### Amber

A warm alternative accent.

The user should be able to change the accent without changing the underlying theme.

Therefore combinations such as:

```text
Smoked Kraft + Crimson
Smoked Kraft + Amber

Ink & Paper + Crimson
Ink & Paper + Amber

Ink & Glass + Crimson
Ink & Glass + Amber

Verdigris Forge + Crimson
Verdigris Forge + Amber

Monochrome + Crimson
Monochrome + Amber
```

should be possible.

---

# THEME ARCHITECTURE

Themes must be data-driven/token-driven.

Do NOT implement themes by duplicating entire UI implementations.

Use a shared design-token model.

Conceptually:

```text
Theme
 ├── surfaces
 ├── text
 ├── borders
 ├── shadows
 ├── glass
 ├── accent
 ├── selection
 ├── diagnostics
 ├── syntax
 ├── typography
 ├── spacing
 └── density
```

The UI components consume tokens.

For example:

```text
Theme
   ↓
Theme Tokens
   ↓
Fyne Components
   ↓
Application
```

NOT:

```text
SmokedKraftUI/
InkPaperUI/
VerdigrisUI/
MonochromeUI/
```

There must be one component system with multiple visual token sets.

---

# THEME SWITCHING

Users must be able to change themes from:

```text
Settings
→ Appearance
→ Theme
```

Also provide a fast command-palette action:

```text
Change Theme
```

Theme changes should update the application without requiring a restart wherever technically practical.

The selected theme should persist across launches.

---

# THEME PREVIEW

The appearance settings should show visual previews.

Each theme preview should communicate:

* background
* panel
* elevated surface
* text
* accent
* selection
* terminal/editor appearance

Avoid giant decorative previews.

The preview itself should demonstrate the actual design tokens.

---

# SYSTEM THEME

Also support:

```text
System
```

where practical.

System mode should select an appropriate light/dark theme based on the operating system preference.

Do not remove manual selection.

Users must always be able to explicitly choose their theme.

---

# EDITOR THEME

The application theme and Neovim/editor theme must remain synchronized.

Changing:

```text
PyxForge Theme
```

should update the Neovim/editor appearance where technically possible.

However, do not tightly couple the Fyne UI implementation to Neovim internals.

Create a clear theme synchronization boundary.

Conceptually:

```text
PyxForge Theme
      │
      ├── Fyne Theme Tokens
      │
      ├── Editor Theme
      │
      ├── Terminal Theme
      │
      └── Diagnostic Colors
```

---

# SYNTAX COLORS

Syntax highlighting must remain readable across every theme.

Do not simply reuse one syntax palette across all themes.

Create semantic syntax tokens such as:

```text
syntax.comment
syntax.keyword
syntax.type
syntax.function
syntax.variable
syntax.constant
syntax.string
syntax.number
syntax.operator
syntax.attribute
syntax.error
```

Each theme should define appropriate values.

Syntax colors must remain restrained.

Avoid rainbow syntax highlighting.

---

# ACCESSIBILITY

Every theme must maintain:

* readable contrast
* visible focus states
* distinguishable active states
* distinguishable disabled states
* readable diagnostics
* readable selections

Do not sacrifice usability for aesthetics.

---

# THEME FILE ORGANIZATION

Use a structure similar to:

```text
internal/ui/theme/
├── theme.go
├── tokens.go
├── registry.go
├── system.go
├── smoked_kraft.go
├── ink_paper.go
├── ink_glass.go
├── verdigris_forge.go
├── monochrome.go
├── crimson.go
├── amber.go
└── preview.go
```

Adapt the structure if a better implementation emerges.

The important requirement is:

**One component architecture + multiple tokenized themes.**

---

# DESIGN REVIEW REQUIREMENT

During Phase 1, render representative PyxForge screens using every theme.

At minimum compare:

* main workspace
* file explorer
* editor
* terminal
* command palette
* debugger
* agent panel
* settings

Do not declare the theme system complete until all themes remain visually coherent.

The goal is not for every theme to look identical.

The goal is for every theme to feel like:

> A legitimate PyxForge theme built from the same design system.

---

# IMPORTANT

Do not remove any of the five visual directions because one is considered the "best".

The purpose of the theme system is to let the user choose.

The visual identity of PyxForge is therefore:

**the design system + component language + typography + interaction model**, not one fixed color palette.
