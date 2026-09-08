package lexer

import (
	"fmt"
	"strconv"

	"sec/internal/diagnostics"
)

// readEscape validates one Sec escape without decoding or rewriting its source.
// It preserves quote/newline boundaries when recovering malformed escapes.
// Rules: rules/foundations/lexical_structure.md — "15. Escapes".
func (l *Lexer) readEscape(quote rune) {
	start, line, column := l.pos, l.line, l.column
	id, message := "", ""
	l.advance()
	kind := l.peek()
	if kind == 0 || isPhysicalLineEnding(kind) {
		id, message = diagnostics.LexerMalformedEscape, "incomplete escape; a backslash must be followed by a valid escape code on the same line"
	} else {
		l.advance()
		switch kind {
		case '\\', '"', '\'', 'n', 'r', 't', '0':
		case 'x':
			digits := 0
			for digits < 2 && isEscapeHex(l.peek()) {
				l.advance()
				digits++
			}
			if digits != 2 {
				id, message = diagnostics.LexerMalformedEscape, "hex escape requires exactly two hexadecimal digits after \\x"
			}
		case 'u':
			if l.peek() != '{' {
				id, message = diagnostics.LexerMalformedEscape, "Unicode escape requires \\u{H...} with one to six hexadecimal digits"
				break
			}
			l.advance()
			digitStart := l.pos
			valid := true
			for l.peek() != '}' && l.peek() != quote && l.peek() != '\\' && l.peek() != 0 && !isPhysicalLineEnding(l.peek()) {
				valid = valid && isEscapeHex(l.peek())
				l.advance()
			}
			digits := string(l.input[digitStart:l.pos])
			closed := l.peek() == '}'
			if closed {
				l.advance()
			}
			if !closed || !valid || len(digits) < 1 || len(digits) > 6 {
				id, message = diagnostics.LexerMalformedEscape, "Unicode escape requires \\u{H...} with one to six hexadecimal digits and a closing brace"
			} else if value, err := strconv.ParseUint(digits, 16, 32); err != nil || value > 0x10FFFF || (value >= 0xD800 && value <= 0xDFFF) {
				id, message = diagnostics.LexerInvalidUnicodeEscape, "Unicode escape must name a Unicode scalar value; surrogates and values above U+10FFFF are invalid"
			}
		default:
			id, message = diagnostics.LexerUnknownEscape, fmt.Sprintf("unknown escape \\%c; use a supported Sec escape or \\\\ for a literal backslash", kind)
		}
	}
	if id != "" {
		l.diagnostics = append(l.diagnostics, Diagnostic{ID: id, Message: message, Primary: l.token(ILLEGAL, string(l.input[start:l.pos]), line, column)})
	}
}

// isEscapeHex recognizes the ASCII hexadecimal grammar of Sec escape digits.
// Rules: rules/foundations/lexical_structure.md — "15. Escapes".
func isEscapeHex(ch rune) bool {
	return ch >= '0' && ch <= '9' || ch >= 'a' && ch <= 'f' || ch >= 'A' && ch <= 'F'
}
