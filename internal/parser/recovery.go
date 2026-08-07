package parser

import "sec/internal/lexer"

// delimiterStack tracks the closers belonging to nested delimiters consumed by
// one recovery helper. A mismatched or unowned closer is never consumed: it is
// left for the caller or an outer recovery context.
type delimiterStack struct {
	closers []lexer.TokenType
}

func newDelimiterStack(expected ...lexer.TokenType) delimiterStack {
	return delimiterStack{closers: append([]lexer.TokenType(nil), expected...)}
}

func (s *delimiterStack) empty() bool {
	return len(s.closers) == 0
}

func (s *delimiterStack) depth() int {
	return len(s.closers)
}

func (s *delimiterStack) canConsume(t lexer.TokenType) bool {
	if !isDelimiterCloser(t) {
		return true
	}
	return len(s.closers) > 0 && s.closers[len(s.closers)-1] == t
}

func (s *delimiterStack) consume(t lexer.TokenType) bool {
	if closer, ok := delimiterCloser(t); ok {
		s.closers = append(s.closers, closer)
		return true
	}
	if !isDelimiterCloser(t) {
		return true
	}
	if !s.canConsume(t) {
		return false
	}
	s.closers = s.closers[:len(s.closers)-1]
	return true
}

func delimiterCloser(t lexer.TokenType) (lexer.TokenType, bool) {
	switch t {
	case lexer.LPAREN:
		return lexer.RPAREN, true
	case lexer.LBRACKET:
		return lexer.RBRACKET, true
	case lexer.LBRACE:
		return lexer.RBRACE, true
	default:
		return "", false
	}
}

func isDelimiterCloser(t lexer.TokenType) bool {
	return t == lexer.RPAREN || t == lexer.RBRACKET || t == lexer.RBRACE
}
