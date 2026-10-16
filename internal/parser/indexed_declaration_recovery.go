package parser

import (
	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
)

// rejectIndexedDeclarationMutation rejects declaration operators on an indexed
// expression without recovering them into executable assignments. The RHS and
// any try handler are consumed as one invalid statement so later code survives.
// This syntax check needs no parallel map type model in the parser.
// Rules: rules/collections/collections.md — §14.5;
// rules/foundations/grammar.md — "Assignment statements", "Try assignment";
// rules/compiler/parser_recovery.md — "Assignment recovery", "Diagnostic deduplication".
func (p *Parser) rejectIndexedDeclarationMutation(target ast.Expression, start lexer.Token, fallible bool) ast.Statement {
	if _, indexed := target.(*ast.IndexExpression); !indexed {
		return nil
	}
	if p.peekToken.Type != lexer.DECLARE && p.peekToken.Type != lexer.MOVE_DECLARE {
		return nil
	}
	p.nextToken()
	operator := p.curToken
	message := "declaration operator " + operator.Lexeme + " cannot mutate an indexed place; use = or <- for assignment"
	before := len(p.diagnostics)
	p.addDiagnostic(diagnostics.ParserInvalidAssignmentExpr, operator, []lexer.TokenType{lexer.ASSIGN, lexer.MOVE_ASSIGN}, &operator, "%s at %d:%d", message, operator.Line, operator.Column)
	if len(p.diagnostics) > before {
		p.diagnostics[len(p.diagnostics)-1].Help = "Use let name := value to declare a binding. Map upsert uses try mapping[key] = value; an explicit ownership move uses <-."
	}
	p.parseAssignmentValue(operator, fallible)
	if fallible && p.peekToken.Type == lexer.LBRACE {
		p.nextToken()
		p.parseTryHandlerBlock()
	}
	return &ast.InvalidStatement{Token: start, Message: message, Recovery: &ast.RecoveryInfo{DiagnosticID: diagnostics.ParserInvalidAssignmentExpr, Message: message, Start: start, End: p.curToken, Reported: true}}
}
