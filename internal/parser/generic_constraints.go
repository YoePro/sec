package parser

import (
	"sec/internal/ast"
	compilerdiagnostics "sec/internal/diagnostics"
	"sec/internal/lexer"
)

// parseGenericConstraintList parses the ordered conjunction after a generic
// parameter colon. The caller owns recovery at the parameter-list boundary;
// this helper retains an invalid final constraint so tooling can preserve the
// committed parameter and its source location.
//
// Rules:
//   - rules/declarations/generics.md — §11 "Constraints"
//   - rules/declarations/generics.md — §12 "Multiple constraints"
//   - rules/declarations/generics.md — §32 "Parser requirements"
func (p *Parser) parseGenericConstraintList(parameter *ast.GenericParameter) []*ast.TypeReference {
	constraints := []*ast.TypeReference{}
	after := ":"
	for {
		if !isTypeStart(p.peekToken.Type) {
			unexpected := p.peekToken
			p.addDiagnostic(
				compilerdiagnostics.ParserInvalidTypeReference,
				unexpected,
				nil,
				&unexpected,
				"expected constraint type after '%s' for generic parameter %s at %d:%d",
				after,
				parameter.Name.Value,
				unexpected.Line,
				unexpected.Column,
			)
			constraints = append(constraints, p.invalidTypeReference(unexpected, ""))
			return constraints
		}

		p.nextToken()
		constraints = append(constraints, p.parseTypeReference())
		if p.peekToken.Type != lexer.BIT_AND {
			return constraints
		}
		p.nextToken()
		parameter.ConstraintOperators = append(parameter.ConstraintOperators, p.curToken)
		after = "&"
	}
}
