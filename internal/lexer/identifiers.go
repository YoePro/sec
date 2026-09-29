package lexer

import (
	"fmt"
	"unicode"

	"golang.org/x/text/unicode/norm"

	compilerdiagnostics "sec/internal/diagnostics"
)

// readIdentifier retains a maximal identifier-like candidate, including
// combining marks and non-ASCII decimal digits, so identifierToken can issue
// one focused diagnostic without splitting the source into misleading tokens.
//
// Rules:
//   - rules/foundations/lexical_structure.md — §6.1 "Identifier form"
//   - rules/foundations/lexical_structure.md — §6.2 "Unicode normalization"
//   - rules/foundations/lexical_structure.md — §20 "Lexical errors"
func (l *Lexer) readIdentifier() string {
	start := l.pos
	for isIdentifierCandidateContinue(l.peek()) {
		l.advance()
	}
	return string(l.input[start:l.pos])
}

// isIdentifierCandidateContinue is deliberately broader than the canonical
// identifier grammar. Non-ASCII decimal digits and combining marks are retained
// only so the lexer can reject the complete candidate with L1005.
//
// Rules:
//   - rules/foundations/lexical_structure.md — §6.1 "Identifier form"
//   - rules/foundations/lexical_structure.md — §18 "Token boundaries"
func isIdentifierCandidateContinue(ch rune) bool {
	return isLetter(ch) || isDigit(ch) || unicode.IsDigit(ch) || unicode.IsMark(ch)
}

// identifierToken validates one retained identifier candidate without
// rewriting its original spelling. NFC failures take precedence; otherwise a
// combining mark or non-ASCII decimal digit produces L1005.
//
// Rules:
//   - rules/foundations/lexical_structure.md — §6.1 "Identifier form"
//   - rules/foundations/lexical_structure.md — §6.2 "Unicode normalization"
//   - rules/foundations/lexical_structure.md — §20 "Lexical errors"
func (l *Lexer) identifierToken(literal string, line, column int) Token {
	token := l.token(lookupIdent(literal), literal, line, column)
	if !norm.NFC.IsNormalString(literal) {
		token.Type = ILLEGAL
		l.diagnostics = append(l.diagnostics, Diagnostic{
			ID:      compilerdiagnostics.LexerNonNFCIdentifier,
			Message: fmt.Sprintf("identifier %q is not in Unicode NFC; use the NFC spelling %q at %d:%d", literal, norm.NFC.String(literal), line, column),
			Primary: token,
		})
		return token
	}
	for _, ch := range literal {
		if unicode.IsMark(ch) || unicode.IsDigit(ch) && !isDigit(ch) {
			token.Type = ILLEGAL
			kind := "combining mark"
			if unicode.IsDigit(ch) {
				kind = "non-ASCII digit"
			}
			l.diagnostics = append(l.diagnostics, Diagnostic{
				ID:      compilerdiagnostics.LexerIdentifierCharacter,
				Message: fmt.Sprintf("identifier %q contains %s U+%04X; identifiers permit letters, ASCII digits, and underscore at %d:%d", literal, kind, ch, line, column),
				Primary: token,
			})
			break
		}
	}
	return token
}
