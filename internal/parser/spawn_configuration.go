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
	p.parseThreadConfigurationCallable(expr)
}

// parseThreadConfigurationCallable preserves enclosing delimiters after either
// configuration form lacks its child callable.
// Rules: rules/concurrency/spawn.md — §4(1),(10);
// rules/compiler/parser_recovery.md — expression recovery.
func (p *Parser) parseThreadConfigurationCallable(expr *ast.SpawnExpression) {
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

// inlineThreadConfigurationAhead recognizes a named-field initializer without
// consuming a spawned statement block. An empty brace pair is configuration
// only when a callable follows on the same line; otherwise it stays a body.
// Rules: rules/concurrency/threads.md — §11(1–2);
// rules/concurrency/spawn.md — §4(1–3).
func (p *Parser) inlineThreadConfigurationAhead() bool {
	if p.peekToken.Type != lexer.LBRACE {
		return false
	}
	state := p.l.Snapshot()
	defer p.l.Restore(state)
	next := func() lexer.Token {
		token := p.l.NextToken()
		for token.Type == lexer.COMMENT {
			token = p.l.NextToken()
		}
		return token
	}
	first := next()
	switch first.Type {
	case lexer.IDENT:
		return next().Type == lexer.COLON
	case lexer.SPREAD:
		return true
	case lexer.RBRACE:
		following := next()
		return following.Line == first.Line && (following.Type == lexer.IDENT || following.Type == lexer.FN)
	}
	return false
}

// parseInlineThreadConfiguration reuses ordinary struct literal fields and
// supplies their contextual core type, then parses the separate child operand.
// No ownership marker is synthesized for this fresh value.
// Rules: rules/concurrency/threads.md — §§10(4),11(1–5).
func (p *Parser) parseInlineThreadConfiguration(expr *ast.SpawnExpression) {
	p.nextToken()
	expr.InlineConfiguration = true
	expr.Configuration = p.parseStructLiteralWithType(&ast.TypeReference{Token: p.curToken, Name: "ThreadConfig"})
	if expr.Configuration == nil {
		return
	}
	p.parseThreadConfigurationCallable(expr)
}
