package editor

import (
	"fmt"
	"image/color"

	"github.com/obstinix/PyxForge/internal/ui/theme"
)

// Colorscheme turns the current PyxForge theme into Neovim highlight groups, so the editor
// shares the shell's surfaces, text, accent and syntax colours (parity N12). Translucent
// tokens are composited over the base surface: Neovim takes opaque colours only.
func Colorscheme(t theme.Tokens) (background string, groups map[string]map[string]any) {
	base := t.Surface.Base
	over := func(c color.NRGBA) string { return hexOf(theme.Over(c, base)) }
	op := func(c color.NRGBA) string { return hexOf(c) }
	fg := func(c color.NRGBA) map[string]any { return map[string]any{"fg": over(c)} }
	bold := func(c color.NRGBA) map[string]any { return map[string]any{"fg": over(c), "bold": true} }
	link := func(name string) map[string]any { return map[string]any{"link": name} }

	background = "dark"
	if t.Polarity == theme.Light {
		background = "light"
	}
	sx, st := t.Syntax, t.Status
	groups = map[string]map[string]any{
		// Surfaces and chrome.
		"Normal":       {"fg": op(t.Text.Primary), "bg": op(base)},
		"NormalNC":     link("Normal"),
		"NormalFloat":  {"fg": op(t.Text.Primary), "bg": op(t.Surface.Overlay)},
		"FloatBorder":  {"fg": over(t.Border.Strong), "bg": op(t.Surface.Overlay)},
		"SignColumn":   {"bg": op(base)},
		"EndOfBuffer":  fg(t.Text.Disabled),
		"NonText":      fg(t.Text.Disabled),
		"Whitespace":   fg(t.Text.Disabled),
		"LineNr":       fg(t.Text.Tertiary),
		"CursorLineNr": bold(t.Text.Primary),
		"CursorLine":   {"bg": over(t.State.Selected)},
		"ColorColumn":  {"bg": over(t.State.Hover)},
		"StatusLine":   {"fg": op(t.Text.Primary), "bg": op(t.Surface.Raised)},
		"StatusLineNC": {"fg": op(t.Text.Tertiary), "bg": op(t.Surface.Sunken)},
		"WinSeparator": fg(t.Border.Strong),
		"TabLine":      {"fg": op(t.Text.Secondary), "bg": op(t.Surface.Raised)},
		"TabLineSel":   {"fg": op(t.Text.Primary), "bg": op(base), "bold": true},
		"TabLineFill":  {"bg": op(t.Surface.Raised)},
		"Visual":       {"bg": over(t.Accent.Selection)},
		"Search":       {"bg": over(t.Accent.Muted), "fg": op(t.Text.Primary)},
		"IncSearch":    {"bg": op(t.Accent.Primary), "fg": op(t.Accent.OnPrimary)},
		"CurSearch":    link("IncSearch"),
		"MatchParen":   {"bg": over(t.State.Pressed), "bold": true},
		"Pmenu":        {"fg": op(t.Text.Primary), "bg": op(t.Surface.Overlay)},
		"PmenuSel":     {"fg": op(t.Text.Primary), "bg": hexOf(theme.Over(t.Accent.Selection, t.Surface.Overlay))},
		"PmenuSbar":    {"bg": op(t.Surface.Sunken)},
		"PmenuThumb":   {"bg": over(t.Border.Strong)},
		"Folded":       {"fg": op(t.Text.Secondary), "bg": op(t.Surface.Sunken)},
		"Title":        bold(t.Text.Primary),
		"Directory":    fg(t.Text.Primary),
		"ModeMsg":      fg(t.Text.Secondary),
		"MoreMsg":      fg(t.Text.Secondary),
		"Question":     fg(t.Text.Secondary),
		"ErrorMsg":     fg(st.Error),
		"WarningMsg":   fg(st.Warning),
		"Cursor":       {"fg": op(base), "bg": op(t.Text.Primary)},

		// Syntax: keywords are bold, so their colour can stay quiet (DESIGN_SYSTEM.md §5).
		"Comment":    fg(sx.Comment),
		"Keyword":    bold(sx.Keyword),
		"Statement":  bold(sx.Keyword),
		"PreProc":    bold(sx.Keyword),
		"Type":       fg(sx.Type),
		"Function":   fg(sx.Function),
		"Identifier": fg(sx.Variable),
		"Constant":   fg(sx.Constant),
		"String":     fg(sx.String),
		"Character":  fg(sx.String),
		"Number":     fg(sx.Number),
		"Float":      fg(sx.Number),
		"Boolean":    fg(sx.Constant),
		"Operator":   fg(sx.Operator),
		"Special":    fg(sx.Attribute),
		"Delimiter":  fg(sx.Operator),
		"Error":      fg(sx.Error),
		"Todo":       bold(sx.Comment),
		"Underlined": {"underline": true},

		// Tree-sitter captures follow the classic groups.
		"@variable":         link("Identifier"),
		"@variable.builtin": link("Constant"),
		"@keyword":          link("Keyword"),
		"@function":         link("Function"),
		"@function.call":    link("Function"),
		"@type":             link("Type"),
		"@string":           link("String"),
		"@number":           link("Number"),
		"@constant":         link("Constant"),
		"@comment":          link("Comment"),
		"@operator":         link("Operator"),
		"@punctuation":      link("Delimiter"),
		"@property":         link("Identifier"),
		"@label":            link("Keyword"),

		// Diffs tint whole lines lightly with the status colours; the changed text is stronger.
		"DiffAdd":    {"bg": tint(st.Success, 0x2e, base)},
		"DiffDelete": {"bg": tint(st.Error, 0x26, base), "fg": over(t.Text.Disabled)},
		"DiffChange": {"bg": tint(st.Info, 0x1f, base)},
		"DiffText":   {"bg": tint(st.Info, 0x4d, base), "bold": true},
		"Added":      fg(st.Success),
		"Removed":    fg(st.Error),
		"Changed":    fg(st.Info),

		// Diagnostics use the status colours, with an undercurl under the text.
		"DiagnosticError":          fg(st.Error),
		"DiagnosticWarn":           fg(st.Warning),
		"DiagnosticInfo":           fg(st.Info),
		"DiagnosticHint":           fg(st.Info),
		"DiagnosticOk":             fg(st.Success),
		"DiagnosticUnderlineError": {"undercurl": true, "sp": over(st.Error)},
		"DiagnosticUnderlineWarn":  {"undercurl": true, "sp": over(st.Warning)},
		"DiagnosticUnderlineInfo":  {"undercurl": true, "sp": over(st.Info)},
		"DiagnosticUnderlineHint":  {"undercurl": true, "sp": over(st.Info)},
	}
	return background, groups
}

// TerminalColors maps the theme onto the 16 ANSI colours terminals use: status colours for
// the normal red, green, yellow and blue, syntax colours for the rest and the bright set, and
// the text hierarchy for black and white (swapped on light themes, so "black" stays dark).
func TerminalColors(t theme.Tokens) [16]string {
	over := func(c color.NRGBA) string { return hexOf(theme.Over(c, t.Surface.Base)) }
	sx, st := t.Syntax, t.Status
	black, white := over(t.Surface.Raised), over(t.Text.Primary)
	if t.Polarity == theme.Light {
		black, white = white, over(t.Surface.Raised)
	}
	return [16]string{
		black, over(st.Error), over(st.Success), over(st.Warning),
		over(st.Info), over(sx.Keyword), over(sx.Type), over(t.Text.Secondary),
		over(t.Text.Tertiary), over(sx.Error), over(sx.String), over(sx.Number),
		over(sx.Function), over(sx.Constant), over(sx.Attribute), white,
	}
}

// tint is c at alpha a over base, as an opaque colour.
func tint(c color.NRGBA, a uint8, base color.NRGBA) string {
	c.A = a
	return hexOf(theme.Over(c, base))
}

func hexOf(c color.NRGBA) string { return fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B) }
