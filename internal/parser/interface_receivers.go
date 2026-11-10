package parser

import (
	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
)

// parseConsumingInterfaceReceiver retains an invalid whole-self requirement
// without adding it to the valid interface method set. Parsing its signature
// preserves following members and permits consuming ordinary parameters/types.
// Rules: rules/declarations/interfaces.md — §3.3;
// rules/memory/ownership.md — Whole-self consumption;
// rules/compiler/parser_recovery.md — Invalid AST nodes.
func (p *Parser) parseConsumingInterfaceReceiver() *ast.InvalidMember {
	start := p.curToken
	message := "interface methods cannot consume the complete receiver; use fn or mut fn, with -> on ordinary parameters when needed"
	p.addDiagnostic(diagnostics.ParserConsumingInterfaceReceiver, start, nil, nil, "%s", message)
	if !p.expectPeek(lexer.FN) {
		return nil
	}
	p.parseFunctionSignature(true)
	return &ast.InvalidMember{Token: start, Message: message, Recovery: &ast.RecoveryInfo{
		DiagnosticID: diagnostics.ParserConsumingInterfaceReceiver, Message: message,
		Start: start, End: p.curToken, Reported: true,
	}}
}
