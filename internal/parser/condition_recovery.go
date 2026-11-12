package parser

import (
	"sec/internal/ast"
	compilerdiagnostics "sec/internal/diagnostics"
	"sec/internal/lexer"
)

// recoverConditionBodyBrace handles stray tokens between a parsed if/while
// condition and its body brace, such as `if ch == 'a'r {`. When the real
// opening brace of the body follows on the same source line at the same
// delimiter depth, the body boundary is established safely (parser_recovery.md
// § 51.2(1), § 53.3): the stray tokens are skipped as one recovery event, the
// condition becomes an invalid expression spanning the condition and the
// stray tokens, and curToken is left on the body brace so the caller parses
// the body. Without such a brace nothing is consumed and false is returned,
// keeping the existing synchronization (§ 51.2(2)).
//
// Without this, the unconsumed body was reparsed as statements of the
// enclosing block and its closing brace closed the enclosing function, moving
// every later impl member to module scope.
func (p *Parser) recoverConditionBodyBrace(construct string, condition ast.Expression) (ast.Expression, bool) {
	stray := p.peekToken
	if stray.Type == lexer.EOF || stray.Type == lexer.RBRACE || stray.Type == lexer.LBRACE || stray.Line != p.curToken.Line {
		return condition, false
	}
	curToken, peekToken := p.curToken, p.peekToken
	lexerState := p.l.Snapshot()
	errorCount := len(p.errors)
	diagnosticCount := len(p.diagnostics)
	restore := func() (ast.Expression, bool) {
		p.curToken, p.peekToken = curToken, peekToken
		p.l.Restore(lexerState)
		p.rollbackErrors(errorCount)
		if len(p.diagnostics) > diagnosticCount {
			p.diagnostics = p.diagnostics[:diagnosticCount]
		}
		return condition, false
	}
	depth := 0
	skipped := 0
	last := stray
	for {
		next := p.peekToken
		if next.Type == lexer.EOF || next.Line != stray.Line {
			return restore()
		}
		switch next.Type {
		case lexer.LPAREN, lexer.LBRACKET:
			depth++
		case lexer.RPAREN, lexer.RBRACKET:
			depth--
			if depth < 0 {
				return restore()
			}
		case lexer.RBRACE:
			return restore()
		case lexer.LBRACE:
			if depth == 0 {
				p.addDiagnostic(compilerdiagnostics.ParserUnexpectedToken, stray, []lexer.TokenType{lexer.LBRACE}, &stray,
					"expected '{' after %s condition at %d:%d", construct, stray.Line, stray.Column)
				p.recordSkippedRecovery(stray, last, skipped, RecoveryProbable)
				start := conditionStartToken(condition, curToken)
				message := "invalid " + construct + " condition"
				p.nextToken()
				return &ast.InvalidExpression{
					Token: start, Message: message,
					Recovery: &ast.RecoveryInfo{DiagnosticID: compilerdiagnostics.ParserUnexpectedToken, Message: message, Start: start, End: last, Skipped: skipped},
				}, true
			}
		}
		p.nextToken()
		last = p.curToken
		skipped++
	}
}

// conditionStartToken returns the first token of a parsed condition.
func conditionStartToken(condition ast.Expression, fallback lexer.Token) lexer.Token {
	for {
		infix, ok := condition.(*ast.InfixExpression)
		if !ok || infix.Left == nil {
			break
		}
		condition = infix.Left
	}
	if condition == nil {
		return fallback
	}
	if token := expressionToken(condition); token.Line > 0 {
		return token
	}
	return fallback
}
