package parser

import "sec/internal/lexer"

// recoveryScan accounts only for source tokens actually discarded by a
// specialized recovery helper. Lookahead boundaries remain outside the range.
// Rules: rules/compiler/parser_recovery.md — "Recovery event", "Record skipped ranges",
// "Enclosing-boundary safety", Appendix A.10.
type recoveryScan struct {
	parser     *Parser
	start, end lexer.Token
	skipped    int
}

// beginRecoveryScan includes the current token only when the caller has
// already committed that token to the malformed region.
// Rules: rules/compiler/parser_recovery.md — "Skipped tokens", "Recovery event".
func (p *Parser) beginRecoveryScan(includeCurrent bool) *recoveryScan {
	scan := &recoveryScan{parser: p, start: p.peekToken}
	if includeCurrent && p.curToken.Type != lexer.EOF {
		scan.start, scan.end, scan.skipped = p.curToken, p.curToken, 1
	}
	return scan
}

// advance records a real consumed token; callers establish delimiter ownership
// with delimiterStack before advancing.
// Rules: rules/compiler/parser_recovery.md — "Progress", "Delimiter depth".
func (scan *recoveryScan) advance() {
	scan.parser.nextToken()
	if scan.parser.curToken.Type != lexer.EOF {
		scan.end = scan.parser.curToken
		scan.skipped++
	}
}

// finish emits one skip event without fabricating an empty or EOF range.
// Rules: rules/compiler/parser_recovery.md — "Recovery event", "Parser error episodes".
func (scan *recoveryScan) finish() {
	if scan.skipped > 0 {
		scan.parser.recordSkippedRecovery(scan.start, scan.end, scan.skipped, RecoveryProbable)
	}
}
