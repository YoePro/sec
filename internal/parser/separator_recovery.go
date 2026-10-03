package parser

import (
	compilerdiagnostics "sec/internal/diagnostics"
	"sec/internal/lexer"
)

// recoverMissingSeparator records one proven missing-comma repair between two
// list items written on separate lines: a P2002 diagnostic at the next item
// and an insert-missing-token recovery event whose After token locates the
// virtual comma directly after the previous item. The caller has already
// proved that the next token begins the next item of the same list; the
// parser then continues with that item as if the comma were present.
//
// Rules:
//   - rules/compiler/parser_recovery.md — "Missing comma", "Prefer fewer assumptions"
//   - rules/tooling/formatter.md — § 27(37) missing list separators
func (p *Parser) recoverMissingSeparator(after lexer.Token, next lexer.Token, closer string, item string) {
	before := len(p.recovery)
	p.addDiagnostic(
		compilerdiagnostics.ParserMissingToken,
		next,
		[]lexer.TokenType{lexer.COMMA},
		&next,
		"expected ',' or '%s' after %s at %d:%d",
		closer,
		item,
		next.Line,
		next.Column,
	)
	if len(p.recovery) > before {
		p.recovery[len(p.recovery)-1].After = after
		if index := p.recovery[len(p.recovery)-1].diagnosticIndex; index >= 0 && index < len(p.diagnostics) {
			p.diagnostics[index].Help = "insert ',' after the " + item + "; " + item + "s on separate lines are still separated by commas"
		}
	}
}

// nextItemOnLaterLine reports whether next begins on a later source line than
// the previous item's last token, the layout that proves a forgotten comma
// rather than a same-line token sequence.
func nextItemOnLaterLine(after lexer.Token, next lexer.Token) bool {
	end := after.EndLine
	if end < after.Line {
		end = after.Line
	}
	return next.Line > end
}

// peekPastComments returns the first non-comment token after the peek token
// and the token following it, without consuming input.
func (p *Parser) peekPastComments() (lexer.Token, lexer.Token) {
	state := p.l.Snapshot()
	defer p.l.Restore(state)
	first := p.peekToken
	for first.Type == lexer.COMMENT {
		first = p.l.NextToken()
	}
	second := p.l.NextToken()
	for second.Type == lexer.COMMENT {
		second = p.l.NextToken()
	}
	return first, second
}

// looksLikeNextParameter proves that the token on a later line begins the
// next parameter: a consuming arrow, a ref modifier, or `name:`.
//
// Rules:
//   - rules/compiler/parser_recovery.md — "Missing comma"
func (p *Parser) looksLikeNextParameter(after lexer.Token) bool {
	if !nextItemOnLaterLine(after, p.peekToken) {
		return false
	}
	switch p.peekToken.Type {
	case lexer.CONSUME_ARROW, lexer.REF:
		return true
	case lexer.IDENT:
		state := p.l.Snapshot()
		defer p.l.Restore(state)
		return p.l.NextToken().Type == lexer.COLON
	}
	return false
}
