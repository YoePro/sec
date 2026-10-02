package parser

import (
	compilerdiagnostics "sec/internal/diagnostics"
	"sec/internal/lexer"
)

// foreignQualifiedName is one parsed C:: or c:: qualified name. The lexer
// produces `::` as two ':' tokens; the parser accepts them as the grammar's
// "::" terminal only when both are adjacent to each other and to the names
// around them, so ordinary `name: Type` annotations never change meaning.
type foreignQualifiedName struct {
	Name       string
	Token      lexer.Token
	Separators []lexer.Token
	Valid      bool
}

// atForeignQualifier reports whether the current token starts a C:: or c::
// foreign qualification.
//
// Rules:
//   - rules/foundations/grammar.md — ForeignTypeReference
//   - rules/platform/ffi.md — §5 "Fundamental C ABI scalar family"; §6 "C library and platform binding namespaces"
func (p *Parser) atForeignQualifier() bool {
	if p.curToken.Type != lexer.IDENT || (p.curToken.Lexeme != "C" && p.curToken.Lexeme != "c") {
		return false
	}
	return p.peekToken.Type == lexer.COLON && p.peekToken.ByteStart == p.curToken.ByteEnd && p.secondForeignColonFollows()
}

// secondForeignColonFollows speculatively inspects the token after the peek
// colon without consuming it.
func (p *Parser) secondForeignColonFollows() bool {
	state := p.l.Snapshot()
	defer p.l.Restore(state)
	next := p.l.NextToken()
	return next.Type == lexer.COLON && next.ByteStart == p.peekToken.ByteEnd
}

// parseForeignQualifiedName consumes `C::name` or `c::segment{::segment}`.
// C:: names exactly one compiler-known fundamental C ABI type; c:: names a
// hierarchical target/library binding path with at least two segments after
// the family. C::fn, C::flex, and C::callback are distinct foreign forms that
// are reported as not yet supported rather than misparsed as scalar names.
//
// Rules:
//   - rules/foundations/grammar.md — ForeignTypeReference
//   - rules/platform/ffi.md — §5, §6, §24, §22, §25, §51 "Parser requirements"
func (p *Parser) parseForeignQualifiedName() foreignQualifiedName {
	family := p.curToken
	result := foreignQualifiedName{Name: family.Lexeme, Token: family}
	segments := 0
	for p.peekToken.Type == lexer.COLON && p.peekToken.ByteStart == p.curToken.ByteEnd && p.secondForeignColonFollows() {
		p.nextToken()
		first := p.curToken
		p.nextToken()
		second := p.curToken
		result.Separators = append(result.Separators, first, second)
		if family.Lexeme == "C" && segments == 0 && (p.peekToken.Type == lexer.FN || p.peekToken.Lexeme == "flex" || p.peekToken.Lexeme == "callback") && p.peekToken.ByteStart == second.ByteEnd {
			p.nextToken()
			p.addDiagnostic(compilerdiagnostics.ParserUnsupportedForeignForm, p.curToken, nil, nil,
				"C::%s is a distinct foreign form that is not supported yet", p.curToken.Lexeme)
			result.Name += "::" + p.curToken.Lexeme
			p.extendForeignToken(&result)
			return result
		}
		if p.peekToken.Type != lexer.IDENT || p.peekToken.ByteStart != second.ByteEnd {
			p.addDiagnostic(compilerdiagnostics.ParserMalformedForeignQualification, second, []lexer.TokenType{lexer.IDENT}, &p.peekToken,
				"expected a name immediately after %s::", result.Name)
			p.extendForeignToken(&result)
			return result
		}
		p.nextToken()
		result.Name += "::" + p.curToken.Lexeme
		segments++
		if family.Lexeme == "C" {
			break
		}
	}
	p.extendForeignToken(&result)
	if family.Lexeme == "c" && segments < 2 {
		p.addDiagnostic(compilerdiagnostics.ParserMalformedForeignQualification, result.Token, nil, nil,
			"c:: binding names require a namespace and a name, such as c::stddef::size_t; got %s", result.Name)
		return result
	}
	result.Valid = true
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
