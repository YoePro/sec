package parser

import (
	"sec/internal/ast"
	compilerdiagnostics "sec/internal/diagnostics"
	"sec/internal/lexer"
)

// isHardwareCategory reports the contextual category word after `hardware`.
func isHardwareCategory(token lexer.Token) bool {
	if token.Type != lexer.IDENT {
		return false
	}
	switch token.Lexeme {
	case "region", "resource", "endpoint":
		return true
	}
	return false
}

// parseHardwareDeclaration parses a module-scope hardware fact declaration
// with its restricted metadata members. `hardware` stays an identifier
// outside this shape; member names, duplicates and value types are checked by
// Sema against the category schema.
//
// Rules:
//   - rules/foundations/grammar.md — HardwareDeclaration (Amendment MD-111)
//   - rules/foundations/lexical_structure.md — contextual `hardware` (Amendment MD-111)
//   - rules/corrections/MD-111-hardware-facts-approved-decision-20261030.md — §§ 3(5), 8(1)
func (p *Parser) parseHardwareDeclaration() *ast.HardwareDeclaration {
	declaration := &ast.HardwareDeclaration{Token: p.curToken}
	p.nextToken()
	declaration.KindToken = p.curToken
	declaration.Category = p.curToken.Lexeme
	if p.peekToken.Type != lexer.IDENT {
		p.hardwareSyntaxError(declaration, p.peekToken, []lexer.TokenType{lexer.IDENT}, "hardware %s declaration requires a name", declaration.Category)
		p.skipHardwareBody()
		return declaration
	}
	p.nextToken()
	declaration.Name = &ast.Identifier{Token: p.curToken, Value: p.curToken.Lexeme}
	if p.peekToken.Type != lexer.LBRACE {
		p.hardwareSyntaxError(declaration, p.peekToken, []lexer.TokenType{lexer.LBRACE}, "hardware %s %s requires a member block", declaration.Category, declaration.Name.Value)
		p.skipHardwareBody()
		return declaration
	}
	p.nextToken()
	for p.peekToken.Type != lexer.RBRACE {
		if p.peekToken.Type == lexer.EOF {
			p.hardwareSyntaxError(declaration, p.peekToken, []lexer.TokenType{lexer.RBRACE}, "hardware %s %s is missing its closing }", declaration.Category, declaration.Name.Value)
			return declaration
		}
		member, ok := p.parseHardwareMember(declaration)
		if ok {
			declaration.Members = append(declaration.Members, member)
		} else {
			p.skipHardwareMember()
		}
		switch p.peekToken.Type {
		case lexer.COMMA:
			p.nextToken()
		case lexer.RBRACE:
		default:
			if ok {
				p.hardwareSyntaxError(declaration, p.peekToken, []lexer.TokenType{lexer.COMMA, lexer.RBRACE}, "hardware members are separated by commas")
				p.skipHardwareMember()
				if p.peekToken.Type == lexer.COMMA {
					p.nextToken()
				}
			}
		}
	}
	p.nextToken()
	return declaration
}

// parseHardwareMember parses `name: value`, leaving the value's last token
// current.
func (p *Parser) parseHardwareMember(declaration *ast.HardwareDeclaration) (*ast.HardwareMember, bool) {
	if p.peekToken.Type != lexer.IDENT {
		p.hardwareSyntaxError(declaration, p.peekToken, []lexer.TokenType{lexer.IDENT}, "hardware member requires a name")
		return nil, false
	}
	p.nextToken()
	member := &ast.HardwareMember{Name: &ast.Identifier{Token: p.curToken, Value: p.curToken.Lexeme}}
	if p.peekToken.Type != lexer.COLON {
		p.hardwareSyntaxError(declaration, p.peekToken, []lexer.TokenType{lexer.COLON}, "hardware member %s requires `: value`", member.Name.Value)
		return nil, false
	}
	p.nextToken()
	value, ok := p.parseHardwareValue(declaration, true)
	if !ok {
		return nil, false
	}
	member.Value = value
	return member, true
}

// parseHardwareValue parses one restricted metadata value. Lists hold only
// integer literals.
func (p *Parser) parseHardwareValue(declaration *ast.HardwareDeclaration, allowList bool) (*ast.HardwareValue, bool) {
	start := p.peekToken
	switch start.Type {
	case lexer.INT:
		p.nextToken()
		return &ast.HardwareValue{Kind: ast.HardwareValueInteger, Token: p.curToken, Text: p.curToken.Lexeme}, true
	case lexer.STRING:
		p.nextToken()
		return &ast.HardwareValue{Kind: ast.HardwareValueString, Token: p.curToken, Text: trimStringQuotes(p.curToken.Lexeme)}, true
	case lexer.TRUE, lexer.FALSE:
		p.nextToken()
		return &ast.HardwareValue{Kind: ast.HardwareValueBool, Token: p.curToken, Bool: start.Type == lexer.TRUE}, true
	case lexer.IDENT:
		p.nextToken()
		value := &ast.HardwareValue{Kind: ast.HardwareValuePath, Token: p.curToken, Path: []*ast.Identifier{{Token: p.curToken, Value: p.curToken.Lexeme}}}
		for p.peekToken.Type == lexer.DOT {
			p.nextToken()
			if p.peekToken.Type != lexer.IDENT {
				p.hardwareSyntaxError(declaration, p.peekToken, []lexer.TokenType{lexer.IDENT}, "hardware metadata path requires a name after `.`")
				return nil, false
			}
			p.nextToken()
			value.Path = append(value.Path, &ast.Identifier{Token: p.curToken, Value: p.curToken.Lexeme})
		}
		return value, true
	case lexer.LBRACKET:
		if !allowList {
			break
		}
		p.nextToken()
		value := &ast.HardwareValue{Kind: ast.HardwareValueList, Token: p.curToken}
		for p.peekToken.Type != lexer.RBRACKET {
			if p.peekToken.Type != lexer.INT {
				p.hardwareSyntaxError(declaration, p.peekToken, []lexer.TokenType{lexer.INT, lexer.RBRACKET}, "hardware metadata lists contain only integer literals")
				return nil, false
			}
			p.nextToken()
			value.Elements = append(value.Elements, &ast.HardwareValue{Kind: ast.HardwareValueInteger, Token: p.curToken, Text: p.curToken.Lexeme})
			if p.peekToken.Type == lexer.COMMA {
				p.nextToken()
			} else if p.peekToken.Type != lexer.RBRACKET {
				p.hardwareSyntaxError(declaration, p.peekToken, []lexer.TokenType{lexer.COMMA, lexer.RBRACKET}, "hardware metadata list elements are separated by commas")
				return nil, false
			}
		}
		p.nextToken()
		return value, true
	}
	p.hardwareSyntaxError(declaration, start, nil, "hardware metadata value must be an integer, string or boolean literal, a qualified enum case, a hardware symbol or an integer list, not %s", describeHardwareToken(start))
	return nil, false
}

func describeHardwareToken(token lexer.Token) string {
	if token.Lexeme != "" {
		return "`" + token.Lexeme + "`"
	}
	return string(token.Type)
}

func (p *Parser) hardwareSyntaxError(declaration *ast.HardwareDeclaration, token lexer.Token, expected []lexer.TokenType, format string, args ...any) {
	declaration.Invalid = true
	unexpected := token
	p.addDiagnostic(compilerdiagnostics.ParserInvalidHardwareDeclaration, token, expected, &unexpected, format+" at %d:%d", append(args, token.Line, token.Column)...)
}

// skipHardwareMember advances to the next top-level comma or the closing
// brace of the member block without consuming it.
func (p *Parser) skipHardwareMember() {
	depth := 0
	for p.peekToken.Type != lexer.EOF {
		switch p.peekToken.Type {
		case lexer.LBRACKET, lexer.LBRACE, lexer.LPAREN:
			depth++
		case lexer.RBRACKET, lexer.RPAREN:
			if depth > 0 {
				depth--
			}
		case lexer.RBRACE:
			if depth == 0 {
				return
			}
			depth--
		case lexer.COMMA:
			if depth == 0 {
				return
			}
		}
		p.nextToken()
	}
}

// skipHardwareBody consumes a following member block after a malformed header.
func (p *Parser) skipHardwareBody() {
	if p.peekToken.Type != lexer.LBRACE {
		return
	}
	p.nextToken()
	depth := 1
	for depth > 0 && p.peekToken.Type != lexer.EOF {
		p.nextToken()
		switch p.curToken.Type {
		case lexer.LBRACE:
			depth++
		case lexer.RBRACE:
			depth--
		}
	}
}
