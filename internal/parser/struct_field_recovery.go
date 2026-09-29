package parser

import "sec/internal/lexer"

// looksLikeNextStructField proves the narrow missing-comma repair permitted by
// the recovery grammar. It deliberately requires a later source line and an
// immediate colon; same-line adjacency and ambiguous identifier sequences are
// left invalid without reinterpretation.
//
// Rules:
//   - rules/compiler/parser_recovery.md — "Struct declaration recovery"
//   - rules/compiler/parser_recovery.md — "Missing comma"
func (p *Parser) looksLikeNextStructField() bool {
	if p.peekToken.Type != lexer.IDENT || p.peekToken.Line <= p.curToken.Line {
		return false
	}
	state := p.l.Snapshot()
	defer p.l.Restore(state)
	return p.l.NextToken().Type == lexer.COLON
}
