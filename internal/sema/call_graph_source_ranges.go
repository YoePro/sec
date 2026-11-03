package sema

import (
	"sec/internal/ast"
	"sec/internal/lexer"
)

// functionDeclarationEnd preserves source endpoints for both ordinary source
// declarations and the compiler's init/free/accessor wrappers around parsed
// bodies. Synthetic code without source ranges stays unknown rather than
// absorbing subsequent declarations during focused analysis.
// Rules: rules/analysis/call_graph.md — "Callable node";
// rules/tooling/cli.md — §5.3.
func functionDeclarationEnd(fn *ast.FunctionDeclaration) lexer.Token {
	if fn.EndToken.Line > 0 {
		return fn.EndToken
	}
	if fn.Body != nil {
		return fn.Body.EndToken
	}
	return lexer.Token{}
}
