package parser

import (
	"sec/internal/ast"
	"sec/internal/diagnostics"
)

const reservedQuestionMarkMessage = "`?` is reserved and has no meaning in Sec 0.1"

// diagnoseReservedQuestionMark emits the canonical focused diagnostic for the
// reserved question-mark token without assigning it ternary, optional-access,
// or error-propagation semantics.
//
// Rules:
//   - rules/foundations/operators.md — "Question mark `?`"
//   - rules/foundations/grammar.md — "General conditional expression"
//   - rules/compiler/parser_recovery.md — "Reserved syntax"
func (p *Parser) diagnoseReservedQuestionMark() {
	token := p.curToken
	p.addDiagnostic(
		diagnostics.ParserReservedSyntax,
		token,
		nil,
		&token,
		"%s",
		reservedQuestionMarkMessage,
	)
}

// parseReservedQuestionMarkExpression retains an invalid expression node for
// tooling after emitting the focused reserved-syntax diagnostic.
//
// Rules:
//   - rules/foundations/operators.md — "Question mark `?`"
//   - rules/compiler/parser_recovery.md — "Reserved syntax"
func (p *Parser) parseReservedQuestionMarkExpression() ast.Expression {
	p.diagnoseReservedQuestionMark()
	return p.invalidExpression(p.curToken, reservedQuestionMarkMessage, diagnostics.ParserReservedSyntax)
}
