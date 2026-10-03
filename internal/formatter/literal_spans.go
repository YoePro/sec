package formatter

import (
	"strings"

	"sec/internal/lexer"
)

// literalEnds maps the byte offset of every string, raw-string, character,
// and interpolated-string literal in one source fragment to its exclusive
// end offset. The spans come from the real lexer, so braces, quotes, and
// comment-like text inside nested interpolation expressions such as
// $"{Format("text }")}" never reach the line-oriented delimiter scanners. A
// literal left unterminated on the fragment extends to its end, matching the
// lexer's malformed-candidate boundary.
//
// Rules:
//   - rules/foundations/lexical_structure.md — §14 "String literals", §14.3 "Interpolated strings"
//   - rules/tooling/formatter.md — § 29 "Formatter invariants" (literal contents are never rewritten)
func literalEnds(text string) map[int]int {
	if !strings.ContainsAny(text, "\"'`") {
		return nil
	}
	ends := map[int]int{}
	source := lexer.New(text)
	for {
		token := source.NextToken()
		if token.Type == lexer.EOF {
			return ends
		}
		switch token.Type {
		case lexer.STRING, lexer.CHAR, lexer.RAW_STRING, lexer.INTERPSTRING:
		case lexer.ILLEGAL:
			if !strings.HasPrefix(token.Lexeme, "\"") && !strings.HasPrefix(token.Lexeme, "'") &&
				!strings.HasPrefix(token.Lexeme, "`") && !strings.HasPrefix(token.Lexeme, "$\"") {
				continue
			}
		default:
			continue
		}
		if token.ByteEnd > token.ByteStart && token.ByteStart >= 0 && token.ByteEnd <= len(text) {
			ends[token.ByteStart] = token.ByteEnd
		}
	}
}

// literalClosed reports whether a lexer literal span ends with its closing
// delimiter rather than at an unterminated fragment boundary.
func literalClosed(literal string) bool {
	if len(literal) < 2 {
		return false
	}
	last := literal[len(literal)-1]
	switch literal[0] {
	case '`':
		return last == '`'
	case '\'':
		return last == '\''
	default:
		return last == '"'
	}
}
