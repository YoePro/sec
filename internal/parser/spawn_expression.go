package parser

import (
	"sec/internal/ast"
	compilerdiagnostics "sec/internal/diagnostics"
	"sec/internal/lexer"
)

// parseSpawnExpression preserves an explicit task/thread/process token so
// parser consumers can distinguish the contextual modifier from the implicit
// task default.
//
// Rules:
//   - rules/concurrency/spawn.md — §§2(1–9),3A(3),25(5)
//   - rules/foundations/grammar.md — §89
//   - rules/foundations/lexical_structure.md — §8 contextual reserved words
func (p *Parser) parseSpawnExpression() ast.Expression {
	defer p.withRecoveryContext(RecoveryContextSpawn)()
	expr := &ast.SpawnExpression{Token: p.curToken, Kind: contextualTask}
	defer func() { expr.EndToken = p.curToken }()
	if p.peekToken.Type == lexer.IDENT && isSpawnKindModifier(p.peekToken.Lexeme) {
		p.nextToken()
		expr.KindToken = p.curToken
		expr.Kind = p.curToken.Lexeme
	}
	if expr.Kind == contextualThread && p.inlineThreadConfigurationAhead() {
		p.parseInlineThreadConfiguration(expr)
		return expr
	}
	if p.peekToken.Type == lexer.LBRACE {
		p.nextToken()
		expr.Body = p.parseStatementBlock("spawn body")
		return expr
	}
	// Preserve the enclosing delimiter so malformed spawn operands cannot
	// consume a call argument, array boundary or surrounding block.
	switch p.peekToken.Type {
	case lexer.EOF, lexer.SEMICOLON, lexer.RBRACE, lexer.COMMA, lexer.RPAREN, lexer.RBRACKET, lexer.LET, lexer.RETURN:
		message := "spawn requires an expression or block after its execution kind"
		p.addDiagnostic(compilerdiagnostics.ParserInvalidExpression, expr.Token, nil, nil, "%s", message)
		expr.Value = p.invalidExpression(expr.Token, message, compilerdiagnostics.ParserInvalidExpression)
		return expr
	}
	p.nextToken()
	operandStart := p.curToken
	expr.Value = p.parseExpression(PREFIX)
	if expr.Value == nil {
		invalid := p.invalidExpression(operandStart, "invalid spawn operand", compilerdiagnostics.ParserInvalidExpression)
		invalid.Recovery.End = p.curToken
		expr.Value = invalid
		return expr
	}
	p.parseNamedThreadConfiguration(expr)
	return expr
}

// isSpawnKindModifier recognizes modifiers only at the spawn grammar boundary.
// Rules: rules/concurrency/spawn.md — §2(4–9); rules/foundations/grammar.md — §89.
func isSpawnKindModifier(value string) bool {
	switch value {
	case contextualTask, contextualThread, contextualProcess:
		return true
	default:
		return false
	}
}
