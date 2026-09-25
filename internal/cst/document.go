// Package cst provides a lossless lexical syntax layer for source-to-source
// tooling. Structural grammar nodes and parser recovery nodes can be added on
// top of this source-ordered foundation.
package cst

import (
	"strings"
	"unicode/utf8"

	"sec/internal/lexer"
)

// Kind classifies a real source element. In particular, malformed lexer
// tokens remain error elements rather than disappearing from the syntax model.
type Kind string

const (
	Token      Kind = "token"
	Whitespace Kind = "whitespace"
	Comment    Kind = "comment"
	Error      Kind = "error"
	BOM        Kind = "bom"
)

// Span is a half-open byte range in the original UTF-8 source. The lexer token
// separately retains its one-based scalar line and column.
type Span struct {
	Start int
	End   int
}

// Element retains exact source bytes, even when the lexer diagnoses invalid
// UTF-8 and represents an offending byte as a replacement scalar internally.
type Element struct {
	Kind  Kind
	Span  Span
	Text  string
	Token lexer.Token
	Roles []Role
}

// Document is the lexical layer of a concrete syntax tree. Elements are
// source-ordered, non-overlapping, and cover every source byte exactly once.
// Groups add delimiter nesting without assigning grammatical roles; unmatched
// closers remain source element indexes. EOF is kept separately because it is
// a zero-width lexical anchor.
type Document struct {
	Elements         []Element
	Groups           []DelimiterGroup
	UnmatchedClosers []int
	EOF              lexer.Token
	Diagnostics      []lexer.Diagnostic
}

// Text reconstitutes the exact source, including comments, invalid bytes,
// whitespace, physical line endings, and an optional initial BOM.
func (d Document) Text() string {
	var result strings.Builder
	for _, element := range d.Elements {
		result.WriteString(element.Text)
	}
	return result.String()
}

// Build consumes the same lexer used by the AST parser, fills the gaps between
// its real tokens with source-preserving whitespace trivia, and derives a
// non-synthetic delimiter hierarchy from the resulting token tape.
//
// Rules:
//   - rules/foundations/lexical_structure.md — §§1.1, 3, 5, 18, 20
//   - rules/tooling/formatter.md — "Source model", Appendix A.5
//   - rules/compiler/parser_recovery.md — "Token and trivia retention"
func Build(source, file string) Document {
	lex := lexer.NewWithFile(source, file)
	offsets, hasBOM := runeByteOffsets(source)
	document := Document{}
	previousEnd := 0
	if hasBOM {
		document.Elements = append(document.Elements, Element{Kind: BOM, Span: Span{0, 3}, Text: source[:3]})
		previousEnd = 3
	}
	for {
		token := lex.NextToken()
		endRune := lex.Snapshot().Pos
		if token.Type == lexer.EOF {
			document.EOF = token
			if previousEnd < len(source) {
				document.appendTrivia(source, previousEnd, len(source))
			}
			break
		}
		startRune := endRune - utf8.RuneCountInString(token.Lexeme)
		start := offsets[startRune]
		end := offsets[endRune]
		if previousEnd < start {
			document.appendTrivia(source, previousEnd, start)
		}
		kind := Token
		switch token.Type {
		case lexer.COMMENT:
			kind = Comment
		case lexer.ILLEGAL:
			kind = Error
		}
		document.Elements = append(document.Elements, Element{
			Kind: kind, Span: Span{start, end}, Text: source[start:end], Token: token,
		})
		previousEnd = end
	}
	document.Diagnostics = lex.Diagnostics()
	document.buildDelimiterGroups(len(source))
	return document
}

// appendTrivia keeps lexer-skipped whitespace while retaining a later BOM as
// an error element: the lexer decodes that invalid scalar to a space for
// recovery, but its original bytes must not be mislabeled as whitespace.
//
// Rules:
//   - rules/foundations/lexical_structure.md — §1.1 "Encoding"
//   - rules/tooling/formatter.md — "Source model"
func (d *Document) appendTrivia(source string, start, end int) {
	const bom = "\xef\xbb\xbf"
	for start < end {
		index := strings.Index(source[start:end], bom)
		if index < 0 {
			d.Elements = append(d.Elements, Element{Kind: Whitespace, Span: Span{start, end}, Text: source[start:end]})
			return
		}
		at := start + index
		if at > start {
			d.Elements = append(d.Elements, Element{Kind: Whitespace, Span: Span{start, at}, Text: source[start:at]})
		}
		d.Elements = append(d.Elements, Element{Kind: Error, Span: Span{at, at + len(bom)}, Text: source[at : at+len(bom)]})
		start = at + len(bom)
	}
}

// runeByteOffsets maps the lexer's decoded-rune cursor to original byte
// offsets. An initial BOM is omitted from the cursor but retained as source.
// Invalid UTF-8 consumes one raw byte per replacement scalar, like the lexer.
//
// Rules:
//   - rules/foundations/lexical_structure.md — §§1.1, 1.3
func runeByteOffsets(source string) ([]int, bool) {
	begin := 0
	hasBOM := strings.HasPrefix(source, "\xef\xbb\xbf")
	if hasBOM {
		begin = 3
	}
	offsets := []int{begin}
	for at := begin; at < len(source); {
		_, size := utf8.DecodeRuneInString(source[at:])
		at += size
		offsets = append(offsets, at)
	}
	return offsets, hasBOM
}
