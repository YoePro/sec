package parser

import (
	"sec/internal/diagnostics"
	"sec/internal/lexer"
)

// addInterfacePlacementDiagnostic keeps the original recovery/message while
// identifying the invalid conformance site and the canonical repair.
// Rules: rules/declarations/interfaces.md — §§4,11,12.
func (p *Parser) addInterfacePlacementDiagnostic(token lexer.Token, format string, args ...any) {
	before := len(p.diagnostics)
	p.addDiagnostic(diagnostics.ParserInterfaceConformancePlacement, token, nil, &token, format, args...)
	if len(p.diagnostics) > before {
		p.diagnostics[len(p.diagnostics)-1].Help = "Declare conformance on the type's primary impl: impl Type implements Interface. Keep extension members in impl extends Type without an implements list."
	}
}
