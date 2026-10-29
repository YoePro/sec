package parser

import (
	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
)

// parsePostfixSpread retains the single postfix expansion or an explicitly
// invalid repeated expansion, including its real source operand. Repeated
// markers are consumed locally without swallowing the enclosing list closer.
// Rules: rules/declarations/spread.md — §§1–2;
// rules/compiler/parser_recovery.md — §§7.3,14.4,63.5.
func (p *Parser) parsePostfixSpread(left ast.Expression) ast.Expression {
	p.nextToken()
	spread := &ast.SpreadExpression{Token: p.curToken, Value: left}
	_, repeated := left.(*ast.SpreadExpression)
	if !repeated && p.peekToken.Type != lexer.SPREAD {
		return spread
	}
	if !repeated {
		left = spread
		p.nextToken()
	}
	token := p.curToken
	message := "spread cannot be repeated; expand the source with one postfix `...`"
	p.addDiagnostic(diagnostics.ParserInvalidExpression, token, nil, nil, "%s", message)
	invalid := p.invalidExpression(token, message, diagnostics.ParserInvalidExpression)
	invalid.Left, invalid.Operator = left, token
	start := p.peekToken
	skipped := 0
	for p.peekToken.Type == lexer.SPREAD {
		p.nextToken()
		invalid.Recovery.End = p.curToken
		skipped++
	}
	if skipped > 0 {
		p.recordSkippedRecovery(start, p.curToken, skipped, RecoveryExact)
		invalid.Recovery.Skipped = skipped
	}
	return invalid
}
