package parser

import (
	compilerdiagnostics "sec/internal/diagnostics"
	"sec/internal/lexer"
)

// foreignQualifiedName is one parsed c:: qualified name. The lexer produces
// `::` as one DOUBLE_COLON token; the parser accepts it as a qualification
// separator only when it touches the names on both sides, so `c :: int` is
// rejected rather than silently joined.
type foreignQualifiedName struct {
	Name       string
	Token      lexer.Token
	Separators []lexer.Token
	Valid      bool
}

// atForeignQualifier reports whether the current token starts a c::
// foreign qualification, or the removed C:: spelling that is diagnosed and
// recovered as c::.
//
// Rules:
//   - rules/foundations/grammar.md — ForeignTypeReference
//   - rules/platform/ffi.md — §5 "Fundamental C ABI scalar family"; §6 "C library and platform binding namespaces"
//   - rules/corrections/applied/md020-md021-md023-md024-ffi-c-bindings-null-correction-20261008.md — §2
func (p *Parser) atForeignQualifier() bool {
	if p.curToken.Type != lexer.IDENT || (p.curToken.Lexeme != "C" && p.curToken.Lexeme != "c") {
		return false
	}
	return p.peekToken.Type == lexer.DOUBLE_COLON
}

// parseForeignQualifiedName consumes `c::segment{::segment}`. One segment
// names a compiler-known fundamental C type, more segments a binding of the
// selected platform C environment; Sema distinguishes them. c::fn, c::flex,
// and c::callback are distinct foreign forms that are reported as not yet
// supported rather than misparsed as names. The removed C:: prefix and
// whitespace around `::` are diagnosed (MD-020 §2.1, §2.4).
//
// Rules:
//   - rules/foundations/grammar.md — ForeignTypeReference
//   - rules/platform/ffi.md — §5, §6, §24, §22, §25, §51 "Parser requirements"
//   - rules/corrections/applied/md020-md021-md023-md024-ffi-c-bindings-null-correction-20261008.md — §2.1–2.4, §2.8
func (p *Parser) parseForeignQualifiedName() foreignQualifiedName {
	family := p.curToken
	result := foreignQualifiedName{Name: "c", Token: family}
	valid := true
	if family.Lexeme == "C" {
		p.addDiagnostic(compilerdiagnostics.ParserRemovedUppercaseForeignQualifier, family, nil, &family,
			"the C:: qualification was removed; write c:: instead")
		valid = false
	}
	segments := 0
	last := family
	for p.peekToken.Type == lexer.DOUBLE_COLON {
		p.nextToken()
		separator := p.curToken
		result.Separators = append(result.Separators, separator)
		if separator.ByteStart != last.ByteEnd || p.peekToken.ByteStart != separator.ByteEnd {
			p.addDiagnostic(compilerdiagnostics.ParserMalformedForeignQualification, separator, nil, &separator,
				"a c:: qualification contains no whitespace inside or around ::")
			valid = false
		}
		if segments == 0 && (p.peekToken.Type == lexer.FN || p.peekToken.Lexeme == contextualFlex || p.peekToken.Lexeme == contextualCallback) {
			p.nextToken()
			p.addDiagnostic(compilerdiagnostics.ParserUnsupportedForeignForm, p.curToken, nil, nil,
				"c::%s is a distinct foreign form that is not supported yet", p.curToken.Lexeme)
			result.Name += "::" + p.curToken.Lexeme
			p.extendForeignToken(&result)
			return result
		}
		if p.peekToken.Type != lexer.IDENT {
			p.addDiagnostic(compilerdiagnostics.ParserMalformedForeignQualification, separator, []lexer.TokenType{lexer.IDENT}, &p.peekToken,
				"expected a name immediately after %s::", result.Name)
			p.extendForeignToken(&result)
			return result
		}
		p.nextToken()
		last = p.curToken
		result.Name += "::" + p.curToken.Lexeme
		segments++
	}
	p.extendForeignToken(&result)
	result.Valid = valid && segments > 0
	return result
}

// extendForeignToken makes the name token span the complete qualification so
// diagnostics, hover, and definitions cover the whole foreign name.
func (p *Parser) extendForeignToken(name *foreignQualifiedName) {
	name.Token.Lexeme = name.Name
	name.Token.EndLine = p.curToken.EndLine
	name.Token.EndColumn = p.curToken.EndColumn
	name.Token.ByteEnd = p.curToken.ByteEnd
}
