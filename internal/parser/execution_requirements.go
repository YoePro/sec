package parser

import (
	"sec/internal/ast"
	"sec/internal/lexer"
)

// parseExecutionRequirements consumes a callable's optional suffix, retaining
// spelling and all repeated clauses rather than attaching a duplicate outward.
// Rules: rules/foundations/grammar.md — §§27(6),32(5),73(3),101(4),101A.
func (p *Parser) parseExecutionRequirements() *ast.ExecutionRequirementClause {
	if !p.hasExecutionRequirementSuffix() {
		return nil
	}
	p.skipPeekComments()
	p.nextToken()
	clause := p.parseExecutionRequirementClause()
	for p.hasExecutionRequirementSuffix() {
		p.skipPeekComments()
		p.nextToken()
		p.addError("repeated require clause on one callable at %d:%d", p.curToken.Line, p.curToken.Column)
		clause.Invalid = true
		clause.Repeated = append(clause.Repeated, p.parseExecutionRequirementClause())
	}
	return clause
}

// hasExecutionRequirementSuffix looks past trivia without stealing documentation
// from the following interface member or declaration when no clause is present.
// Rules: rules/foundations/grammar.md — §101A(2)–(3);
// rules/compiler/parser_recovery.md — "Token and trivia retention".
func (p *Parser) hasExecutionRequirementSuffix() bool {
	if p.peekToken.Type == lexer.COMMENT {
		first, _ := p.peekPastComments()
		return first.Type == lexer.REQUIRE
	}
	return p.peekToken.Type == lexer.REQUIRE
}

// parseExecutionRequirementClause parses contextual entries, recovering at the
// clause's own comma/close and preserving the following body or declaration.
// Rules: rules/foundations/grammar.md — §101A(1)–(5);
// rules/compiler/parser_recovery.md — "Delimiter recovery".
func (p *Parser) parseExecutionRequirementClause() *ast.ExecutionRequirementClause {
	clause := &ast.ExecutionRequirementClause{Token: p.curToken}
	if !p.expectPeek(lexer.LPAREN) {
		clause.Invalid = true
		return clause
	}
	clause.Open = p.curToken
	seen := map[string]bool{}
	for {
		p.skipPeekComments()
		switch p.peekToken.Type {
		case lexer.RPAREN:
			p.nextToken()
			clause.Close = p.curToken
			if len(clause.Entries) == 0 {
				p.addError("require clause must contain task, thread or cancellable at %d:%d", clause.Token.Line, clause.Token.Column)
				clause.Invalid = true
			}
			return clause
		case lexer.EOF, lexer.LBRACE, lexer.RBRACE, lexer.FN, lexer.LET:
			p.addError("unclosed require clause before %q at %d:%d", p.peekToken.Lexeme, p.peekToken.Line, p.peekToken.Column)
			clause.Invalid = true
			return clause
		}
		p.nextToken()
		entry := p.curToken
		clause.Entries = append(clause.Entries, entry)
		valid := entry.Type == lexer.IDENT && (entry.Lexeme == "task" || entry.Lexeme == "thread" || entry.Lexeme == "cancellable")
		if !valid {
			p.addError("unsupported require entry %q in Sec 0.1.1 at %d:%d", entry.Lexeme, entry.Line, entry.Column)
			clause.Invalid = true
		}
		if seen[entry.Lexeme] {
			p.addError("repeated require entry %q at %d:%d", entry.Lexeme, entry.Line, entry.Column)
			clause.Invalid = true
		}
		seen[entry.Lexeme] = true
		p.skipPeekComments()
		if p.peekToken.Type != lexer.COMMA && p.peekToken.Type != lexer.RPAREN {
			p.addError("require entries are names separated by commas, not expressions, at %d:%d", p.peekToken.Line, p.peekToken.Column)
			clause.Invalid = true
			for p.peekToken.Type != lexer.COMMA && p.peekToken.Type != lexer.RPAREN && p.peekToken.Type != lexer.EOF && p.peekToken.Type != lexer.LBRACE && p.peekToken.Type != lexer.RBRACE && p.peekToken.Type != lexer.FN && p.peekToken.Type != lexer.LET {
				p.nextToken()
				switch p.curToken.Type {
				case lexer.LPAREN:
					p.skipBalancedGroup(lexer.LPAREN, lexer.RPAREN)
				case lexer.LBRACKET:
					p.skipBalancedGroup(lexer.LBRACKET, lexer.RBRACKET)
				}
			}
		}
		if p.peekToken.Type == lexer.COMMA {
			p.nextToken()
			clause.Commas = append(clause.Commas, p.curToken)
		}
	}
}
