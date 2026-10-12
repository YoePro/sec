package parser

import (
	"sec/internal/ast"
	"sec/internal/lexer"
)

// parseRegisterType parses a nominal register representation and retains a
// missing width as an invalid expression when the owned closing bracket makes
// the body boundary reliable. Accepted modifier token ranges remain available
// to the lossless formatter instead of being rediscovered from word spelling.
//
// Rules:
//   - rules/declarations/registers.md — "Width"
//   - rules/foundations/grammar.md — "Register declaration"
//   - rules/compiler/parser_recovery.md — "Register recovery"
//   - rules/tooling/formatter.md — §5(2,4–5),§22(1)
func (p *Parser) parseRegisterType() *ast.RegisterType {
	registerType := &ast.RegisterType{
		Token:           p.curToken,
		AllocationOrder: "lsb-first",
	}
	allocationOrderSet := false

	if !p.expectPeek(lexer.LBRACKET) {
		return registerType
	}
	widthExpression, width, ok := p.parseRegisterWidthExpression("register width")
	registerType.WidthExpression = widthExpression
	if !ok {
		if p.peekToken.Type != lexer.RBRACKET {
			return registerType
		}
		p.nextToken()
	} else {
		registerType.Width = width
		if !p.expectPeek(lexer.RBRACKET) {
			return registerType
		}
	}
	for p.peekToken.Type == lexer.IDENT {
		modifierToken := p.peekToken
		p.nextToken()
		modifier, ok := p.parseRegisterTypeModifier()
		if !ok {
			p.addError("unknown register modifier %q at %d:%d", modifierToken.Lexeme, modifierToken.Line, modifierToken.Column)
			return registerType
		}
		switch modifier {
		case "lsb-first", "msb-first":
			if allocationOrderSet {
				if registerType.AllocationOrder == modifier {
					p.addError("duplicate register allocation modifier %q at %d:%d", modifier, modifierToken.Line, modifierToken.Column)
				} else {
					p.addError("conflicting register allocation modifiers %q and %q at %d:%d", registerType.AllocationOrder, modifier, modifierToken.Line, modifierToken.Column)
				}
				return registerType
			}
			registerType.AllocationOrder = modifier
			allocationOrderSet = true
		case "little-endian", "big-endian":
			if registerType.ByteOrder != "" {
				if registerType.ByteOrder == modifier {
					p.addError("duplicate register byte-order modifier %q at %d:%d", modifier, modifierToken.Line, modifierToken.Column)
				} else {
					p.addError("conflicting register byte-order modifiers %q and %q at %d:%d", registerType.ByteOrder, modifier, modifierToken.Line, modifierToken.Column)
				}
				return registerType
			}
			registerType.ByteOrder = modifier
		}
		registerType.ModifierTokens = append(registerType.ModifierTokens, ast.RegisterModifierTokens{Start: modifierToken, End: p.curToken})
	}
	if !p.expectPeek(lexer.LBRACE) {
		return registerType
	}

	registerType.Fields = p.parseRegisterFields()
	if !p.expectPeek(lexer.RBRACE) {
		return registerType
	}

	return registerType
}

// parseRegisterTypeModifier recognizes the closed register allocation and
// byte-order modifier spellings, leaving semantic values on RegisterType.
// Rules: rules/declarations/registers.md — §7 "Field allocation order",§8 "Byte order".
func (p *Parser) parseRegisterTypeModifier() (string, bool) {
	prefix := p.curToken.Lexeme
	if prefix != "lsb" && prefix != "msb" && prefix != "little" && prefix != "big" {
		return "", false
	}
	if !p.expectPeek(lexer.MINUS) || !p.expectPeek(lexer.IDENT) {
		return "", false
	}
	suffix := p.curToken.Lexeme
	if (prefix == "lsb" || prefix == "msb") && suffix == "first" {
		return prefix + "-first", true
	}
	if (prefix == "little" || prefix == "big") && suffix == "endian" {
		return prefix + "-endian", true
	}
	return "", false
}
