package cst

import "sec/internal/lexer"

// DelimiterGroup is a source-order structural view over one real opening
// delimiter. Open and Close are indexes in Document.Elements; Close is -1
// when no matching real closer was found. Parent is the containing group index
// or -1 for a top-level group. No synthetic token is implied by an open group.
type DelimiterGroup struct {
	Open   int
	Close  int
	Parent int
	Span   Span
}

// buildDelimiterGroups records nesting of real (), [], and {} tokens without
// interpreting their grammatical roles or inventing parser recovery. A closer
// that does not match the innermost opener remains an unmatched real token.
// Literal and comment contents are opaque because the lexer emits each as one
// token. Incomplete groups extend to EOF while retaining their real opener.
//
// Rules:
//   - rules/tooling/formatter.md — "Source model", "Same-line delimiter trivia"
//   - rules/compiler/parser_recovery.md — "Token and trivia retention", "Virtual missing tokens"
func (d *Document) buildDelimiterGroups(sourceEnd int) {
	stack := []int{}
	for elementIndex, element := range d.Elements {
		if element.Kind != Token {
			continue
		}
		typ := element.Token.Type
		if isOpeningDelimiter(typ) {
			parent := -1
			if len(stack) > 0 {
				parent = stack[len(stack)-1]
			}
			d.Groups = append(d.Groups, DelimiterGroup{
				Open: elementIndex, Close: -1, Parent: parent,
				Span: Span{Start: element.Span.Start, End: sourceEnd},
			})
			stack = append(stack, len(d.Groups)-1)
			continue
		}
		if !isClosingDelimiter(typ) {
			continue
		}
		if len(stack) == 0 || matchingDelimiter(d.Elements[d.Groups[stack[len(stack)-1]].Open].Token.Type) != typ {
			d.UnmatchedClosers = append(d.UnmatchedClosers, elementIndex)
			continue
		}
		group := stack[len(stack)-1]
		d.Groups[group].Close = elementIndex
		d.Groups[group].Span.End = element.Span.End
		stack = stack[:len(stack)-1]
	}
}

// isOpeningDelimiter identifies only real lexical group starters; a grammar
// parser decides later whether one begins a block, call, type, or literal.
// Rules: rules/tooling/formatter.md — "Source model", "Same-line delimiter trivia".
func isOpeningDelimiter(typ lexer.TokenType) bool {
	switch typ {
	case lexer.LPAREN, lexer.LBRACKET, lexer.LBRACE:
		return true
	default:
		return false
	}
}

// isClosingDelimiter identifies real lexical group endings without assigning
// grammar meaning or inserting a token on malformed input.
// Rules: rules/compiler/parser_recovery.md — "Virtual missing tokens".
func isClosingDelimiter(typ lexer.TokenType) bool {
	switch typ {
	case lexer.RPAREN, lexer.RBRACKET, lexer.RBRACE:
		return true
	default:
		return false
	}
}

// matchingDelimiter maps each opening token to its one real closing token.
// Rules: rules/tooling/formatter.md — "Same-line delimiter trivia".
func matchingDelimiter(open lexer.TokenType) lexer.TokenType {
	switch open {
	case lexer.LPAREN:
		return lexer.RPAREN
	case lexer.LBRACKET:
		return lexer.RBRACKET
	case lexer.LBRACE:
		return lexer.RBRACE
	default:
		return ""
	}
}
