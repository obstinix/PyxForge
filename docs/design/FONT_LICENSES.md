# Bundled Fonts and Icons

Everything here ships inside the PyxForge binary through `go:embed`; nothing is downloaded at
runtime (Section 11.3, Section 12). Each license text sits next to the files it covers.
Downloaded 2026-10-08; SHA-256 of each bundled file below.

## Fonts (`internal/ui/theme/fonts/`)

| Family | Files | Role | Source | License |
|---|---|---|---|---|
| Geist 1.7.2 | `Geist-Regular.ttf`, `Geist-Medium.ttf`, `Geist-SemiBold.ttf` | UI text; Fyne regular and bold | `github.com/vercel/geist-font` release v1.7.2, archive SHA-256 `7fc800d2…b04e2` matched GitHub's published digest | SIL OFL 1.1, `Geist-OFL.txt` |
| JetBrains Mono 2.304 | `JetBrainsMono-Regular.ttf`, `-Bold.ttf`, `-Italic.ttf` | code, data, terminal; Fyne monospace | `github.com/JetBrains/JetBrainsMono` release v2.304 (the release predates GitHub digests; archive SHA-256 `6f6376c6…7bbf` recorded) | SIL OFL 1.1, `JetBrainsMono-OFL.txt` |
| Syne | `Syne-SemiBold.ttf` (static 600 instance) | display: panel and dialog titles | Google Fonts static instance (`fonts.gstatic.com/s/syne/v24/…`); license from `github.com/google/fonts` `ofl/syne/OFL.txt` | SIL OFL 1.1, `Syne-OFL.txt` |

| File | SHA-256 |
|---|---|
| Geist-Regular.ttf | `5c8968eafb98a4c4f47033daf29e38e284a6f2a82eb017d171ab040fe7c4b615` |
| Geist-Medium.ttf | `0090e004725f6f64b841715b4167920580f883fcf9b67fc6d744089103fec101` |
| Geist-SemiBold.ttf | `612ec98df33935354f39e81e54101656961ab6e5549f64b63eb57868ba7bab8d` |
| JetBrainsMono-Regular.ttf | `a0bf60ef0f83c5ed4d7a75d45838548b1f6873372dfac88f71804491898d138f` |
| JetBrainsMono-Bold.ttf | `5590990c82e097397517f275f430af4546e1c45cff408bde4255dad142479dcb` |
| JetBrainsMono-Italic.ttf | `9d0a1f7a708e6af183f1193b7e81d40da294f5c67682c085d8401c60aac8ded4` |
| Syne-SemiBold.ttf | `97b5b68a4ef5bd771287825879378e1fba6c08e690a35fb4845935396f5663dd` |

The OFL allows bundling with software of any license, including Apache-2.0, provided the fonts
are not sold on their own and keep their license and reserved names. Apple's SF fonts are never
bundled (Section 3.5).

## Icons (`internal/ui/icons/svg/`)

Lucide 1.53.0 (`github.com/lucide-icons/lucide`, `lucide-icons-1.53.0.zip`, SHA-256
`9b493937…53d3f` matched GitHub's published digest). ISC License, `internal/ui/icons/LICENSE`.
Only the 46 icons the app uses are copied: binary, bot, bug, check, chevron-down, chevron-right,
circle, circle-check, circle-x, command, cpu, ellipsis, file, file-code, file-diff, file-text, flag,
folder, folder-open, folder-tree, git-branch, git-commit-horizontal, hammer, hash, info, list,
list-tree, memory-stick, minus, palette, panel-bottom, panel-left, panel-right, pause, play, plus,
refresh-cw, search, server, settings, square, step-forward, sun-moon, terminal, triangle-alert, x.

All share one drawing style: 24 px grid, 2 px round strokes, `stroke="currentColor"`, no fills.
PyxForge recolours them itself (`internal/ui/icons`), because Fyne's themed-resource recolouring
replaces fills and would fill these outlines solid. It also rewrites Lucide's repeated arc
parameters into explicit arc commands at load time: Fyne's SVG renderer draws implicit arc
repetition wrongly (17 of the original 41 icons use it; the settings gear rendered as an "8").
