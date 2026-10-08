// Package defaults materializes opt-in source edits from canonical Sema default
// decisions. It does not derive language defaults or change ordinary formatting.
package defaults

import (
	"sec/internal/ast"
	astwalk "sec/internal/ast/walk"
	"sec/internal/lexer"
)

// Site retains source anchors before Sema inserts synthesized initializers.
type Site struct {
	Node          any
	Start, End    int
	SourceEntries int
}

// Capture records only existing source default sites; imported and synthesized
// nodes cannot become edit destinations in another document.
// Rules: rules/types/default_values.md — "LSP"; rules/tooling/lsp.md — "Snapshots".
func Capture(source string, program *ast.Program) []Site {
	var sites []Site
	_ = astwalk.Inspect(program, func(node any) error {
		candidate := false
		switch node := node.(type) {
		case *ast.LetStatement:
			candidate = node.Value == nil && node.Type != nil
		case *ast.TypeDeclStatement:
			candidate = node.Default == nil && node.BaseType != nil && len(node.GenericParameters) == 0
		case *ast.StructLiteral:
			candidate = true
		}
		if candidate {
			if start, end, ok := completeSpan(source, node); ok {
				site := Site{Node: node, Start: start, End: end}
				if literal, ok := node.(*ast.StructLiteral); ok {
					site.SourceEntries = len(literal.Fields)
				}
				sites = append(sites, site)
			}
		}
		return nil
	})
	return sites
}

// completeSpan closes only delimiters opened inside the parser-token envelope,
// preserving trailing comments and enclosing function/aggregate delimiters.
// Rules: rules/tooling/lsp.md — "Safe fixes"; rules/types/default_values.md — "LSP".
func completeSpan(source string, node any) (int, int, bool) {
	start, end, ok := astwalk.Span(node)
	if !ok || end > len(source) {
		return 0, 0, false
	}
	var stack []lexer.TokenType
	lex := lexer.New(source)
	for token := lex.NextToken(); token.Type != lexer.EOF; token = lex.NextToken() {
		if token.ByteStart < start || token.Type == lexer.COMMENT {
			continue
		}
		if token.ByteStart >= end && len(stack) == 0 {
			break
		}
		switch token.Type {
		case lexer.LPAREN:
			stack = append(stack, lexer.RPAREN)
		case lexer.LBRACKET:
			stack = append(stack, lexer.RBRACKET)
		case lexer.LBRACE:
			stack = append(stack, lexer.RBRACE)
		case lexer.RPAREN, lexer.RBRACKET, lexer.RBRACE:
			if len(stack) == 0 || stack[len(stack)-1] != token.Type {
				return 0, 0, false
			}
			stack = stack[:len(stack)-1]
		}
		if token.ByteEnd > end {
			end = token.ByteEnd
		}
	}
	return start, end, len(stack) == 0
}

// lastSourceToken ignores comments when deciding whether an aggregate needs a
// separator before appended default fields.
// Rules: rules/types/default_values.md — "LSP", "Struct spread and defaults";
// rules/tooling/formatter.md — comment preservation.
func lastSourceToken(source string, start, end int) lexer.TokenType {
	last := lexer.EOF
	lex := lexer.New(source[start:end])
	for token := lex.NextToken(); token.Type != lexer.EOF; token = lex.NextToken() {
		if token.Type != lexer.COMMENT {
			last = token.Type
		}
	}
	return last
}
