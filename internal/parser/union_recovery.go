package parser

import (
	"sec/internal/ast"
	compilerdiagnostics "sec/internal/diagnostics"
	"sec/internal/lexer"
)

// parseUnionVariantPayload retains the committed single-payload variant when
// its type position is empty or malformed. Recovery consumes an owned closing
// parenthesis when one exists, but stops before the union-body close, a comma,
// or a probable variant beginning on a later line.
//
// Rules:
//   - rules/compiler/parser_recovery.md — "Union recovery"
//   - rules/compiler/parser_recovery.md — "Initial invalid expression and type retention"
//   - rules/declarations/unions.md — §3.2 "Single unnamed payload"
func (p *Parser) parseUnionVariantPayload(variantToken lexer.Token) *ast.TypeReference {
	if !isTypeStart(p.peekToken.Type) {
		unexpected := p.peekToken
		p.addDiagnostic(
			compilerdiagnostics.ParserInvalidTypeReference,
			unexpected,
			nil,
			&unexpected,
			"expected union variant payload type, got %q at %d:%d",
			unexpected.Type,
			unexpected.Line,
			unexpected.Column,
		)
		payload := p.invalidTypeReference(unexpected, "")
		p.recoverUnionVariantPayload(variantToken, payload)
		return payload
	}

	p.nextToken()
	payload := p.parseTypeReference()
	if p.peekToken.Type == lexer.RPAREN {
		p.nextToken()
		return payload
	}

	unexpected := p.peekToken
	p.addDiagnostic(
		compilerdiagnostics.ParserMissingToken,
		unexpected,
		[]lexer.TokenType{lexer.RPAREN},
		&unexpected,
		"expected ')' after union variant payload, got %q at %d:%d",
		unexpected.Type,
		unexpected.Line,
		unexpected.Column,
	)
	payload = p.markInvalidTypeReference(payload)
	p.recoverUnionVariantPayload(variantToken, payload)
	return payload
}

func (p *Parser) recoverUnionVariantPayload(variantToken lexer.Token, payload *ast.TypeReference) {
	start, end, skipped := p.peekToken, p.peekToken, 0
	for p.peekToken.Type != lexer.EOF &&
		p.peekToken.Type != lexer.RBRACE &&
		p.peekToken.Type != lexer.COMMA &&
		!(p.peekToken.Type == lexer.IDENT && p.peekToken.Line > variantToken.Line) {
		p.nextToken()
		end = p.curToken
		skipped++
		if p.curToken.Type == lexer.RPAREN {
			break
		}
	}
	if skipped == 0 {
		return
	}
	recovery := p.recordSkippedRecovery(start, end, skipped, RecoveryProbable)
	if payload != nil && payload.Recovery != nil {
		payload.Recovery.Start = recovery.Start
		payload.Recovery.End = recovery.End
		payload.Recovery.Skipped = recovery.Skipped
	}
}
