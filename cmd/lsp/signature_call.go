package main

import (
	"reflect"

	"sec/internal/ast"
	"sec/internal/lexer"
)

type signatureCall struct {
	expression ast.Expression
	open       int
}

// signatureCallAtOffset selects the innermost parser-owned call argument list.
// Real delimiters and retained statement starts bound incomplete calls; names
// and types are never inferred from source spelling. Nested ordinary calls also
// participate so their argument lists cannot display an outer raw-pointer API.
// Rules: rules/tooling/lsp.md — "Signature help", "Recovery nodes";
// rules/compiler/parser_recovery.md — "Enclosing-boundary safety", "Interaction with LSP".
func signatureCallAtOffset(program *ast.Program, path, text string, offset int) (signatureCall, bool) {
	tokens := sourceTokens("", text)
	statements := map[int]bool{}
	walkLSPNodes(program, func(node any) {
		if _, statement := node.(ast.Statement); !statement {
			return
		}
		value := reflect.ValueOf(node).Elem().FieldByName("Token")
		if !value.IsValid() || !value.CanInterface() {
			return
		}
		token, ok := value.Interface().(lexer.Token)
		if ok && statementSourceMatches(token, path) {
			statements[textPositionOffset(text, token.Line, token.Column)] = true
		}
	})
	selected := signatureCall{open: -1}
	for _, expression := range astExpressionsInProgram(program) {
		var open lexer.Token
		switch node := expression.(type) {
		case *ast.CallExpression:
			open = node.ArgumentsOpen
		case *ast.NewExpression:
			open = node.ArgumentsOpen
		case *ast.OkExpression:
			open = node.ArgumentsOpen
		case *ast.ErrExpression:
			open = node.ArgumentsOpen
		case *ast.RuntimeCallExpression:
			open = node.ArgumentsOpen
		case *ast.ConversionExpression:
			open = node.ArgumentsOpen
		case *ast.BitExtractionExpression:
			open = node.ArgumentsOpen
		default:
			continue
		}
		if open.Type != lexer.LPAREN || !statementSourceMatches(open, path) {
			continue
		}
		start := textPositionOffset(text, open.Line, open.Column)
		if start >= offset || start <= selected.open {
			continue
		}
		end, inclusive := signatureArgumentEnd(tokens, statements, start, len(text))
		if offset > end || offset == end && !inclusive {
			continue
		}
		selected = signatureCall{expression: expression, open: start}
	}
	return selected, selected.open >= 0
}

// signatureArgumentEnd respects delimiter ownership and AST statement
// boundaries without duplicating the parser's statement-start grammar. A real
// ')' is inclusive; foreign closers and following statements are excluded.
// Rules: rules/compiler/parser_recovery.md — "Enclosing-boundary safety";
// rules/tooling/lsp.md — "Signature help", "Invalid regions".
func signatureArgumentEnd(tokens []lexer.Token, statements map[int]bool, open, eof int) (int, bool) {
	closers := []lexer.TokenType{lexer.RPAREN}
	for _, token := range tokens {
		if token.ByteStart <= open {
			continue
		}
		if len(closers) == 1 && statements[token.ByteStart] {
			return token.ByteStart, false
		}
		switch token.Type {
		case lexer.LPAREN:
			closers = append(closers, lexer.RPAREN)
		case lexer.LBRACKET:
			closers = append(closers, lexer.RBRACKET)
		case lexer.LBRACE:
			closers = append(closers, lexer.RBRACE)
		case lexer.RPAREN, lexer.RBRACKET, lexer.RBRACE:
			if closers[len(closers)-1] != token.Type {
				return token.ByteStart, false
			}
			closers = closers[:len(closers)-1]
			if len(closers) == 0 {
				return token.ByteStart, true
			}
		}
	}
	return eof, true
}
