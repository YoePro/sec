package parser

import "sec/internal/lexer"

// skipTestParameterList consumes the balanced source-visible parameter list
// forbidden on a test declaration and leaves the following header/body token
// available to the owning parser.
//
// Rules:
//   - rules/tooling/testing.md — §5.5 "No parameters"
//   - rules/compiler/parser_recovery.md — "Recovery goals"
func (p *Parser) skipTestParameterList() {
	scan := p.beginRecoveryScan(true)
	defer scan.finish()
	delimiters := newDelimiterStack(lexer.RPAREN)
	for !delimiters.empty() && p.peekToken.Type != lexer.EOF {
		if !delimiters.canConsume(p.peekToken.Type) {
			return
		}
		scan.advance()
		delimiters.consume(p.curToken.Type)
	}
}

// recoverTestBodyStart skips an invalid test header up to its owned body while
// refusing to consume a later top-level declaration on a following line.
//
// Rules:
//   - rules/tooling/testing.md — §§5.4–5.6 test declaration shape
//   - rules/compiler/parser_recovery.md — "Declaration-header recovery"
func (p *Parser) recoverTestBodyStart() bool {
	scan := p.beginRecoveryScan(false)
	defer scan.finish()
	startLine := p.curToken.Line
	delimiters := newDelimiterStack()
	for p.peekToken.Type != lexer.EOF {
		if delimiters.empty() {
			if p.peekToken.Type == lexer.LBRACE {
				return true
			}
			if p.peekToken.Line > startLine && (isDeclarationStart(p.peekToken.Type) ||
				p.peekToken.Type == lexer.IDENT && p.peekToken.Lexeme == contextualTest) {
				return false
			}
		}
		if !delimiters.canConsume(p.peekToken.Type) {
			return false
		}
		scan.advance()
		delimiters.consume(p.curToken.Type)
	}
	return false
}

// skipConditionPatternPayload consumes the balanced payload list that starts at
// the current token so the condition can resynchronize at the body brace.
// Rules: rules/compiler/parser_recovery.md — "Recovery event", "Enclosing-boundary safety".
func (p *Parser) skipConditionPatternPayload() {
	scan := p.beginRecoveryScan(true)
	defer scan.finish()
	delimiters := newDelimiterStack()
	delimiters.consume(p.curToken.Type)
	for !delimiters.empty() && p.peekToken.Type != lexer.EOF {
		if !delimiters.canConsume(p.peekToken.Type) {
			return
		}
		scan.advance()
		delimiters.consume(p.curToken.Type)
	}
}

// skipRestOfStatementLine consumes the remaining tokens of the current line,
// keeping delimiters balanced and leaving a closing brace that ends the
// enclosing block for its owner.
// Rules: rules/compiler/parser_recovery.md — "Recovery event", "Enclosing-boundary safety".
func (p *Parser) skipRestOfStatementLine() {
	scan := p.beginRecoveryScan(false)
	defer p.endRecoveryEpisode()
	defer scan.finish()
	line := p.curToken.Line
	delimiters := newDelimiterStack()
	for p.peekToken.Type != lexer.EOF {
		if delimiters.empty() && p.peekToken.Line != line {
			return
		}
		if !delimiters.canConsume(p.peekToken.Type) {
			return
		}
		scan.advance()
		delimiters.consume(p.curToken.Type)
	}
}

// skipUntilBlockStart preserves foreign closers and later declarations while
// locating the owned body opener.
// Rules: rules/compiler/parser_recovery.md — "Declaration-header recovery", "Enclosing-boundary safety".
func (p *Parser) skipUntilBlockStart() {
	scan := p.beginRecoveryScan(false)
	defer scan.finish()
	delimiters := newDelimiterStack()
	if p.curToken.Type == lexer.LBRACE {
		return
	}
	for p.curToken.Type != lexer.EOF {
		if delimiters.empty() && (p.peekToken.Type == lexer.LBRACE) {
			p.nextToken() // the body opener belongs to the caller, not the skipped range
			return
		}
		if delimiters.empty() && p.peekToken.Line > p.curToken.Line && isDeclarationStart(p.peekToken.Type) {
			return
		}
		if !delimiters.canConsume(p.peekToken.Type) {
			return
		}
		scan.advance()
		delimiters.consume(p.curToken.Type)
	}
}

// skipRejectedMatchPayload keeps the invalid arm as one recovery episode. The
// outer payload '(' was already consumed, so the first unmatched ')' is its
// boundary; nested delimiters are balanced while scanning toward it, and
// unowned closers or the arm arrow remain available to the caller.
//
// Rule: rules/compiler/parser_recovery.md, match-arm synchronization.
func (p *Parser) skipRejectedMatchPayload() {
	scan := p.beginRecoveryScan(true)
	defer scan.finish()
	delimiters := newDelimiterStack(lexer.RPAREN)
	if _, opens := delimiterCloser(p.curToken.Type); opens {
		delimiters.consume(p.curToken.Type)
	}
	for !delimiters.empty() && p.peekToken.Type != lexer.EOF {
		if delimiters.depth() == 1 && p.peekToken.Type == lexer.ARROW {
			return
		}
		if !delimiters.canConsume(p.peekToken.Type) {
			return
		}
		scan.advance()
		delimiters.consume(p.curToken.Type)
	}
}

// recoverAttributeArguments skips to the `)` that closes the current argument
// list so the following declaration still parses. Foreign closing delimiters
// remain unconsumed.
// Rules: rules/compiler/parser_recovery.md — "Attribute recovery", "Record skipped ranges", "Enclosing-boundary safety".
func (p *Parser) recoverAttributeArguments() {
	scan := p.beginRecoveryScan(false)
	defer scan.finish()
	delimiters := newDelimiterStack(lexer.RPAREN)
	for !delimiters.empty() && p.peekToken.Type != lexer.EOF {
		if !delimiters.canConsume(p.peekToken.Type) {
			return
		}
		// A later declaration proves the attribute list cannot own these tokens.
		if delimiters.depth() == 1 && p.peekToken.Line > p.curToken.Line && isDeclarationStart(p.peekToken.Type) {
			return
		}
		scan.advance()
		delimiters.consume(p.curToken.Type)
	}
}
