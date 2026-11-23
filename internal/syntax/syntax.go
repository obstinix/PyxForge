// Package syntax colours source text for PyxForge's native editor. It runs Chroma's lexers
// (pure Go, MIT; github.com/alecthomas/chroma) over a document and reduces their token types
// to the eleven roles of theme.Syntax, so a theme colours every language the same way.
package syntax

import (
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
)

// Kind is a token's role; Plain is drawn in the theme's primary text colour.
type Kind uint8

const (
	Plain Kind = iota
	Comment
	Keyword
	Type
	Function
	Variable
	Constant
	String
	Number
	Operator
	Attribute
	Error
)

// Span is a run of one kind on one line, in rune columns [Start, End).
type Span struct {
	Start, End int
	Kind       Kind
}

// Highlighter colours one language.
type Highlighter struct {
	lexer chroma.Lexer
	Name  string // the language, as Chroma names it
}

// ForFile picks a highlighter by file name, then by content; nil when no language matches.
func ForFile(path string, head string) *Highlighter {
	l := lexers.Match(filepath.Base(path))
	if l == nil && head != "" {
		l = lexers.Analyse(head)
	}
	if l == nil {
		switch strings.ToLower(filepath.Ext(path)) {
		case ".asm", ".s", ".inc":
			l = lexers.Get("nasm")
		case ".ld", ".lds":
			l = lexers.Get("c")
		}
	}
	if l == nil {
		return nil
	}
	return &Highlighter{lexer: chroma.Coalesce(l), Name: l.Config().Name}
}

// Lines colours text and returns one list of spans per line ("\n"-separated). Plain runs are
// left out: whatever no span covers is plain.
func (h *Highlighter) Lines(text string) [][]Span {
	out := make([][]Span, strings.Count(text, "\n")+1)
	it, err := h.lexer.Tokenise(nil, text)
	if err != nil {
		return out
	}
	line, col := 0, 0
	for tok := it(); tok != chroma.EOF; tok = it() {
		k := kindOf(tok.Type)
		v := tok.Value
		for v != "" {
			i := strings.IndexByte(v, '\n')
			part := v
			if i >= 0 {
				part = v[:i]
			}
			n := utf8.RuneCountInString(part)
			if k != Plain && n > 0 && strings.TrimSpace(part) != "" && line < len(out) {
				out[line] = append(out[line], Span{Start: col, End: col + n, Kind: k})
			}
			col += n
			if i < 0 {
				break
			}
			line++
			col = 0
			v = v[i+1:]
		}
	}
	return out
}

func kindOf(t chroma.TokenType) Kind {
	switch {
	case t == chroma.Error:
		return Error
	case t == chroma.CommentPreproc, t == chroma.CommentPreprocFile:
		return Attribute // #include, #define: directives, not prose
	case t.InCategory(chroma.Comment):
		return Comment
	case t == chroma.KeywordType:
		return Type
	case t == chroma.KeywordConstant:
		return Constant
	case t.InCategory(chroma.Keyword):
		return Keyword
	case t == chroma.NameFunction, t == chroma.NameFunctionMagic:
		return Function
	case t == chroma.NameClass, t == chroma.NameNamespace:
		return Type
	case t == chroma.NameBuiltin:
		return Function
	case t == chroma.NameConstant, t == chroma.NameEntity:
		return Constant
	case t == chroma.NameVariable, t == chroma.NameVariableInstance, t == chroma.NameVariableClass, t == chroma.NameVariableGlobal:
		return Variable
	case t == chroma.NameAttribute, t == chroma.NameDecorator, t == chroma.NameTag, t == chroma.NameLabel:
		return Attribute
	case t.InSubCategory(chroma.LiteralString):
		return String
	case t.InSubCategory(chroma.LiteralNumber):
		return Number
	case t.InCategory(chroma.Literal):
		return Constant
	case t.InCategory(chroma.Operator):
		return Operator
	}
	return Plain
}
