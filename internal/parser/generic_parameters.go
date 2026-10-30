package parser

import (
	"sec/internal/ast"
	"sec/internal/lexer"
)

// parseGenericParameters parses declaration-owned generic parameters and
// preserves completed and later comma-separated parameters around a malformed
// entry instead of discarding the committed declaration.
//
// Rules:
//   - rules/declarations/generics.md — "Generic parameter syntax", §16A
//   - rules/corrections/md117-heterogeneous-variadic-type-packs-correction.md — §2(1–6)
//   - rules/compiler/parser_recovery.md — "Generic parameter recovery"
func (p *Parser) parseGenericParameters(allowPack bool) []*ast.GenericParameter {
	var packSeen bool
	params := []*ast.GenericParameter{}

	if p.peekToken.Type == lexer.RBRACKET {
		p.addError("expected generic parameter name at %d:%d", p.peekToken.Line, p.peekToken.Column)
		p.nextToken()
		return nil
	}

	for {
		packToken := lexer.Token{}
		if p.peekToken.Type == lexer.SPREAD {
			p.nextToken()
			packToken = p.curToken
			if !allowPack {
				p.addError("generic type packs are permitted only on functions and methods at %d:%d", packToken.Line, packToken.Column)
			}
			if packSeen {
				p.addError("a generic declaration may have only one type pack at %d:%d", packToken.Line, packToken.Column)
			}
		} else if packSeen {
			p.addError("generic type pack must be last at %d:%d", p.peekToken.Line, p.peekToken.Column)
		}
		if p.peekToken.Type != lexer.IDENT {
			p.addError("expected generic parameter name at %d:%d", p.peekToken.Line, p.peekToken.Column)
			before := p.curToken
			p.skipGenericParameterList()
			// A separator followed by EOF or a foreign closer leaves nothing to
			// skip; retrying from the same comma would make no progress.
			// rules/compiler/parser_recovery.md — "Recovery invariants": "Progress".
			if p.curToken.Type == lexer.COMMA && p.curToken != before {
				if p.peekToken.Type == lexer.RBRACKET {
					p.nextToken()
					return params
				}
				continue
			}
			return params
		}
		p.nextToken()
		param := &ast.GenericParameter{
			TypePack:  packToken.Type == lexer.SPREAD,
			PackToken: packToken,
			Token:     p.curToken,
			Name:      &ast.Identifier{Token: p.curToken, Value: p.curToken.Lexeme},
		}

		packSeen = packSeen || param.TypePack
		if p.peekToken.Type == lexer.COLON {
			p.nextToken()
			param.Constraints = p.parseGenericConstraintList(param)
			if len(param.Constraints) == 0 || param.Constraints[len(param.Constraints)-1].Invalid {
				params = append(params, param)
				p.skipGenericParameterList()
				if p.curToken.Type == lexer.COMMA {
					if p.peekToken.Type == lexer.RBRACKET {
						p.nextToken()
						return params
					}
					continue
				}
				return params
			}
		}

		params = append(params, param)

		switch p.peekToken.Type {
		case lexer.COMMA:
			p.nextToken()
			if p.peekToken.Type == lexer.RBRACKET {
				p.nextToken()
				return params
			}
		case lexer.RBRACKET:
			p.nextToken()
			return params
		default:
			p.addError("expected ',' or ']' after generic parameter %s at %d:%d", param.Name.Value, p.peekToken.Line, p.peekToken.Column)
			p.skipGenericParameterList()
			if p.curToken.Type == lexer.COMMA {
				if p.peekToken.Type == lexer.RBRACKET {
					p.nextToken()
					return params
				}
				continue
			}
			return params
		}
	}
}
