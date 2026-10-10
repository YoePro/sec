package parser

import (
	"sec/internal/ast"
	compilerdiagnostics "sec/internal/diagnostics"
	"sec/internal/lexer"
)

// parseSwitchCaseItem parses one comma-separated switch alternative. A bare
// `_` is retained as an identifier item so Sema can reject the wildcard
// pattern with the switch-specific diagnostic.
//
// Rules:
//   - rules/control-flow/flowcontrol_switch.md — §7 "Case expressions", §8 "Range cases", §10 "Relational cases"
//   - rules/control-flow/flowcontrol_switch.md — §28 "No pattern matching", §35 "Parser requirements"
func (p *Parser) parseSwitchCaseItem(subjectless bool) ast.SwitchCaseItem {
	previousStart := p.switchCaseStart
	p.switchCaseStart = p.curToken
	defer func() { p.switchCaseStart = previousStart }()
	if p.curToken.Type == lexer.UNDERSCORE && (p.peekToken.Type == lexer.COLON || p.peekToken.Type == lexer.COMMA) {
		wildcard := &ast.Identifier{Token: p.curToken, Value: "_"}
		return &ast.SwitchValueCase{Token: p.curToken, Value: wildcard}
	}
	if subjectless {
		value := p.parseExpression(LOWEST)
		if value == nil {
			return nil
		}
		return &ast.SwitchValueCase{Token: expressionToken(value), Value: value}
	}

	switch p.curToken.Type {
	case lexer.LT, lexer.LTE, lexer.GT, lexer.GTE:
		token := p.curToken
		operator := p.curToken.Lexeme
		p.nextToken()
		value := p.parseExpression(COMPARE)
		if value == nil {
			return nil
		}
		return &ast.SwitchRelationalCase{Token: token, Operator: operator, Value: value}
	case lexer.RANGE, lexer.RANGE_EXCLUSIVE:
		rangeExpr := &ast.RangeExpression{Token: p.curToken, Exclusive: p.curToken.Type == lexer.RANGE_EXCLUSIVE}
		if p.isExpressionStart(p.peekToken.Type) {
			p.nextToken()
			rangeExpr.End = p.parseExpression(COMPARE)
		}
		return &ast.SwitchRangeCase{Token: rangeExpr.Token, Range: rangeExpr}
	default:
		value := p.parseRangeOrExpression()
		if value == nil {
			return nil
		}
		if rangeExpr, ok := value.(*ast.RangeExpression); ok {
			return &ast.SwitchRangeCase{Token: rangeExpr.Token, Range: rangeExpr}
		}
		return &ast.SwitchValueCase{Token: expressionToken(value), Value: value}
	}
}

// recoverSwitchStructPattern reports a shorthand/destructuring field in the
// outer switch case expression and consumes the rest of its balanced braces.
// It preserves the invalid item, clause body, subsequent alternatives and
// declarations without inventing bindings or treating the syntax as a value.
//
// Rules:
//   - rules/control-flow/flowcontrol_switch.md — §28 "No pattern matching", §40 "Required diagnostics"
//   - rules/foundations/grammar.md — "StructLiteralItem": fields require Identifier ":" Expression
//   - rules/compiler/parser_recovery.md — "Bounded damage", "Recovery invariants", "Parser diagnostic IDs": P2009
func (p *Parser) recoverSwitchStructPattern(ref *ast.TypeReference) ast.Expression {
	message := "switch does not support pattern binding; use match"
	p.addDiagnostic(compilerdiagnostics.ParserInvalidPattern, ref.Token, nil, nil, "%s", message)
	if len(p.diagnostics) > 0 {
		p.diagnostics[len(p.diagnostics)-1].Help = "Use match to bind or destructure a variant payload; switch cases require values."
	}
	invalid := p.invalidExpression(ref.Token, message, compilerdiagnostics.ParserInvalidPattern)
	start, end := p.curToken, p.curToken
	delimiters := newDelimiterStack(lexer.RBRACE)
	skipped := 0
	for p.curToken.Type != lexer.EOF {
		if delimiters.depth() == 1 && (p.curToken.Type == lexer.CASE || p.curToken.Type == lexer.DEFAULT) {
			break
		}
		if !delimiters.consume(p.curToken.Type) {
			break
		}
		end = p.curToken
		skipped++
		if delimiters.empty() {
			break
		}
		p.nextToken()
	}
	if skipped > 0 {
		p.recordSkippedRecovery(start, end, skipped, RecoveryUnambiguous)
	}
	invalid.Recovery.End = end
	invalid.Recovery.Skipped = skipped
	return invalid
}
