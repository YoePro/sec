package parser

import (
	"strings"

	"sec/internal/ast"
	compilerdiagnostics "sec/internal/diagnostics"
	"sec/internal/lexer"
)

// Sec 0.1 accepts only canonical syntax. The parser still recognizes the
// legacy, future, and never-normative forms below so it can retain the AST
// for recovery, but each one is an error with a focused migration message.
//
// Rules:
//   - rules/foundations/grammar.md — "Grammar implementation status", "Legacy, future, and recovery syntax"
//   - rules/corrections/applied/missing-decisions-md001-md009-correction-20261003.md — §§ 3–4

// reportLegacyAssignedType rejects `type Name = T` and the compact variant
// `type Name = A B ...`.
func (p *Parser) reportLegacyAssignedType(assign lexer.Token, name string, variants []string) {
	if len(variants) > 0 {
		p.addDiagnostic(compilerdiagnostics.ParserLegacyAssignedType, assign, nil, nil,
			"legacy compact variant declaration `type %s = %s` is not Sec 0.1 syntax; declare the alternatives with enum or union", name, strings.Join(variants, " "))
		return
	}
	p.addDiagnostic(compilerdiagnostics.ParserLegacyAssignedType, assign, nil, nil,
		"legacy declaration `type %s = ...` is not Sec 0.1 syntax; write `type %s <Type>` without `=`", name, name)
}

// rejectAdditionalUnderlyingTypes reports and skips further type references
// on the declaration line after the single underlying type.
func (p *Parser) rejectAdditionalUnderlyingTypes(name string, declarationLine int) {
	if p.peekToken.Line != declarationLine || p.peekToken.Type == lexer.EOF || !isTypeStart(p.peekToken.Type) || p.isStatementStart(p.peekToken.Type) {
		return
	}
	p.nextToken()
	p.addDiagnostic(compilerdiagnostics.ParserMultipleUnderlyingTypes, p.curToken, nil, nil,
		"named type %s declares more than one underlying type; a named type has exactly one underlying type", name)
	p.parseTypeReference()
	for p.peekToken.Line == declarationLine && p.peekToken.Type != lexer.EOF && isTypeStart(p.peekToken.Type) && !p.isStatementStart(p.peekToken.Type) {
		p.nextToken()
		p.parseTypeReference()
	}
}

// rejectAdditionalTypeDeclarationNames reports `type A, B int`: a type
// declaration declares exactly one name. The extra names are consumed so the
// first declaration is retained for recovery.
//
// Rules:
//   - rules/foundations/grammar.md — "Named type declaration"
//   - rules/corrections/applied/missing-decisions-md001-md009-correction-20261003.md — § 3.1
func (p *Parser) rejectAdditionalTypeDeclarationNames(first string) {
	if p.peekToken.Type != lexer.COMMA {
		return
	}
	p.nextToken()
	comma := p.curToken
	names := []string{first}
	for p.curToken.Type == lexer.COMMA && p.peekToken.Type == lexer.IDENT {
		p.nextToken()
		names = append(names, p.curToken.Lexeme)
		if p.peekToken.Type != lexer.COMMA {
			break
		}
		p.nextToken()
	}
	separate := make([]string, 0, len(names))
	for _, name := range names {
		separate = append(separate, "`type "+name+" <Type>`")
	}
	p.addDiagnostic(compilerdiagnostics.ParserMultipleTypeDeclarationNames, comma, nil, nil,
		"a type declaration declares exactly one name; declare %s separately", strings.Join(separate, " and "))
}

// reportLegacyInlineContract rejects a contract written on a field or
// variable type instead of on a named type.
func (p *Parser) reportLegacyInlineContract(contractStart lexer.Token, owner string) {
	p.addDiagnostic(compilerdiagnostics.ParserLegacyInlineContract, contractStart, nil, nil,
		"inline contract on %s is not Sec 0.1 syntax; declare a named constrained type such as `type Name int range 0..100` and use that type", owner)
}

// reportPrefixSequenceType rejects `[]T` and `[N]T`, which were never Sec
// syntax; the canonical spellings are `T[]` and `T[N]`.
func (p *Parser) reportPrefixSequenceType(open lexer.Token, length string, element string) {
	if length == "" {
		p.addDiagnostic(compilerdiagnostics.ParserPrefixSequenceType, open, nil, nil,
			"prefix sequence type `[]%s` is not Sec syntax; write `%s[]`", element, element)
		return
	}
	p.addDiagnostic(compilerdiagnostics.ParserPrefixSequenceType, open, nil, nil,
		"prefix array type `[%s]%s` is not Sec syntax; write `%s[%s]`", length, element, element, length)
}

// reportLegacyEnumColonInitializer rejects `Member: value` inside an enum
// body; explicit enum initializers use `=` (rules/declarations/enums.md).
func (p *Parser) reportLegacyEnumColonInitializer(colon lexer.Token, member string) {
	p.addDiagnostic(compilerdiagnostics.ParserLegacyEnumColonInitializer, colon, nil, nil,
		"legacy enum initializer `%s: ...` is not Sec 0.1 syntax; write `%s = ...`", member, member)
}

// reportFutureStructDeclaration rejects the planned Sec 0.2 standalone struct
// declaration in Sec 0.1 source.
func (p *Parser) reportFutureStructDeclaration(structToken lexer.Token, name string) {
	p.addDiagnostic(compilerdiagnostics.ParserFutureStructDeclaration, structToken, nil, nil,
		"`struct %s { ... }` is planned Sec 0.2 syntax; Sec 0.1 declares structs as `type %s struct { ... }`", name, name)
}

// typeReferenceSpelling renders a parsed type reference for a migration
// message using canonical postfix sequence syntax.
func typeReferenceSpelling(ref *ast.TypeReference) string {
	if ref == nil {
		return "T"
	}
	if ref.ElementType != nil {
		element := typeReferenceSpelling(ref.ElementType)
		if ref.ArrayLengthExpression != nil {
			return element + "[" + ref.ArrayLengthExpression.String() + "]"
		}
		return element + "[]"
	}
	name := ref.Name
	if ref.Ref {
		name = "ref " + name
		if ref.MutableRef {
			name = "ref mut " + ref.Name
		}
	}
	if len(ref.TypeArgs) == 0 {
		return name
	}
	arguments := make([]string, 0, len(ref.TypeArgs))
	for _, argument := range ref.TypeArgs {
		arguments = append(arguments, typeReferenceSpelling(argument))
	}
	return name + "[" + strings.Join(arguments, ", ") + "]"
}
