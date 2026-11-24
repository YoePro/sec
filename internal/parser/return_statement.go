package parser

import (
	"sec/internal/ast"
	compilerdiagnostics "sec/internal/diagnostics"
	"sec/internal/lexer"
)

// parseReturnStatement retains the optional terminal move marker separately
// from the complete returned expression. Both spellings use the same semantic
// ownership boundary; the marker is neither a unary operation nor a requirement
// that the returned expression name a reusable binding.
// Rules:
//   - rules/foundations/grammar.md — "Return statement"
//   - rules/declarations/functions.md — §14 "Return ownership"
//   - rules/memory/copy_move.md — §9 "Return boundaries"
func (p *Parser) parseReturnStatement() ast.Statement {
	defer p.withRecoveryContext(RecoveryContextReturn)()
	stmt := &ast.ReturnStatement{Token: p.curToken}

	if p.peekToken.Type == lexer.RBRACE || p.peekToken.Type == lexer.EOF || isSwitchClauseStart(p.peekToken) {
		return stmt
	}
	if p.peekToken.Line > p.curToken.Line && p.isReturnTerminatorToken(p.peekToken) {
		return stmt
	}

	p.nextToken()
	if p.curToken.Type == lexer.MOVE_ASSIGN {
		stmt.MoveToken = p.curToken
		if isMissingOperandBoundary(p.peekToken.Type) || isSwitchClauseStart(p.peekToken) ||
			p.peekToken.Line > p.curToken.Line && p.isReturnTerminatorToken(p.peekToken) {
			stmt.Value = p.missingOperandExpression(p.curToken, p.peekToken)
			return stmt
		}
		p.nextToken()
	}
	valueStart := p.curToken
	stmt.Value = p.parseExpression(LOWEST)
	if stmt.Value == nil {
		// A malformed operand is still a value-bearing source return, never
		// a bare return or fallthrough (spawn.md §§3A(3),25(5);
		// parser_recovery.md — Invalid AST nodes).
		invalid := p.invalidExpression(valueStart, "invalid returned expression", compilerdiagnostics.ParserInvalidExpression)
		invalid.Recovery.End = p.curToken
		stmt.Value = invalid
	}
	if p.peekToken.Type == lexer.COMMA && p.peekToken.Line == p.curToken.Line {
		// rules/foundations/grammar.md — "Return statement": one optional
		// Expression. Keep the first value and the statement.
		comma := p.peekToken
		p.addDiagnostic(compilerdiagnostics.ParserInvalidStatement, comma, nil, &comma,
			"return takes one value; to return several values, return one struct or Result value that holds them at %d:%d", comma.Line, comma.Column)
		p.skipRestOfStatementLine()
	}

	return stmt
}
