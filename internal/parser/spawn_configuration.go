package parser

import (
	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
)

// parseNamedThreadConfiguration separates the owning configuration from the
// child callable in spawn thread <-config Work(). A missing marker on a named
// configuration is retained for ordinary Sema consuming-argument diagnostics.
// No type lookup occurs here; task/process operands retain their own grammar.
// Rules: rules/concurrency/spawn.md — §4(1),(10);
// rules/concurrency/threads.md — §10(3).
func (p *Parser) parseNamedThreadConfiguration(expr *ast.SpawnExpression) {
	if expr.Kind != contextualThread {
		return
	}
	explicit := false
	switch value := expr.Value.(type) {
	case *ast.PrefixExpression:
		explicit = value.Operator == "<-"
		if !explicit {
			return
		}
	case *ast.Identifier, *ast.MemberExpression, *ast.IndexExpression:
		if p.peekToken.Line != p.curToken.Line || (p.peekToken.Type != lexer.IDENT && p.peekToken.Type != lexer.FN) {
			return
		}
	default:
		return
	}
	expr.Configuration = expr.Value
	switch p.peekToken.Type {
	case lexer.EOF, lexer.SEMICOLON, lexer.RBRACE, lexer.COMMA, lexer.RPAREN, lexer.RBRACKET, lexer.LET, lexer.RETURN:
		message := "spawn thread configuration must be followed by a callable expression"
		p.addDiagnostic(diagnostics.ParserInvalidExpression, expr.Token, nil, nil, "%s", message)
		expr.Value = p.invalidExpression(expr.Token, message, diagnostics.ParserInvalidExpression)
		return
	}
	p.nextToken()
	expr.Value = p.parseExpression(PREFIX)
}
