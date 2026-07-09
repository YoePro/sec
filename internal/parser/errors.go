package parser

import (
	"sec/internal/ast"
	"sec/internal/lexer"
)

// Intended for error helper functions

func (p *Parser) expectPeekRangeOperator() bool {
	switch p.peekToken.Type {
	case lexer.RANGE, lexer.RANGE_EXCLUSIVE:
		p.nextToken()
		return true

	default:
		p.addError(
			"expected range operator ('..' or '..<'), got %q at %d:%d",
			p.peekToken.Lexeme,
			p.peekToken.Line,
			p.peekToken.Column,
		)
		return false
	}
}

func (p *Parser) requireRangeBound(contract *ast.RangeContract) bool {
	if contract.Min != nil || contract.Max != nil {
		return true
	}

	p.addError(
		"range contract must have at least one bound at %d:%d",
		contract.Token.Line,
		contract.Token.Column,
	)

	return false
}
