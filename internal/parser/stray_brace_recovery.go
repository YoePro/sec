package parser

import (
	compilerdiagnostics "sec/internal/diagnostics"
	"sec/internal/lexer"
)

// strayClosingBrace decides whether the current `}` is an extra closing brace
// rather than the end of the construct opened on opener's line (MD-132). Sec
// is not indentation-sensitive, so layout is only evidence for choosing which
// brace is stray when the brace-only parse would be an error anyway: the `}`
// must be the first token on its line and indented deeper than the opener
// line, and the next token must be on a later line, also indented deeper than
// the opener line, and of a kind that cannot legally follow the closed
// construct. A stray brace is reported once and skipped by the caller.
// Rules: rules/compiler/parser_recovery.md — § 24 (MD-132), "Recovery invariants".
func (p *Parser) strayClosingBrace(opener lexer.Token, construct string, continues func(lexer.TokenType) bool) bool {
	brace, next := p.curToken, p.peekToken
	if brace.Type != lexer.RBRACE || brace.Line == 0 || opener.Line == 0 {
		return false
	}
	first, known := p.lineFirstColumns[brace.Line]
	openerIndent, openerKnown := p.lineFirstColumns[opener.Line]
	if !known || !openerKnown || first != brace.Column || brace.Column <= openerIndent {
		return false
	}
	if next.Line <= brace.Line || next.Column <= openerIndent || !continues(next.Type) {
		return false
	}
	p.addDiagnostic(compilerdiagnostics.ParserUnexpectedToken, brace, nil, &brace,
		"unexpected '}' at %d:%d: the %s opened at %d:%d continues with %q at %d:%d; remove the extra brace",
		brace.Line, brace.Column, construct, opener.Line, opener.Column, next.Lexeme, next.Line, next.Column)
	p.recordSkippedRecovery(brace, brace, 1, RecoveryProbable)
	return true
}

// switchContinuesWith: only another case clause can follow inside a switch,
// and no statement can start with `case` or `default`.
func switchContinuesWith(next lexer.TokenType) bool {
	return next == lexer.CASE || next == lexer.DEFAULT
}

// functionBodyContinuesWith lists tokens that can only start a statement,
// never a module or impl member after a closed function.
func functionBodyContinuesWith(next lexer.TokenType) bool {
	switch next {
	case lexer.RETURN, lexer.IF, lexer.WHILE, lexer.FOR, lexer.SWITCH, lexer.MATCH,
		lexer.BREAK, lexer.CONTINUE, lexer.DEFER, lexer.DISCARD, lexer.CASE, lexer.DEFAULT:
		return true
	}
	return false
}
