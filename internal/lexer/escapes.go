package lexer

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"sec/internal/diagnostics"
)

// DecodeStringLiteral decodes a lexer-validated ordinary Sec string literal.
// It intentionally does not accept raw or interpolated string delimiters.
//
// Rules:
//   - rules/foundations/lexical_structure.md — §14.1 "Ordinary strings"
//   - rules/foundations/lexical_structure.md — §15 "Escapes"
func DecodeStringLiteral(lexeme string) (string, bool) {
	return decodeQuotedLiteral(lexeme, '"', false)
}

// DecodeCharacterLiteral decodes one lexer-validated Sec character literal
// and confirms that its decoded value is exactly one Unicode scalar.
//
// Rules:
//   - rules/foundations/lexical_structure.md — §13 "Character literals"
//   - rules/foundations/lexical_structure.md — §15 "Escapes"
func DecodeCharacterLiteral(lexeme string) (string, bool) {
	decoded, ok := decodeQuotedLiteral(lexeme, '\'', true)
	return decoded, ok && utf8.RuneCountInString(decoded) == 1
}

// DecodeStringText materializes lexer-validated Sec string content without
// requiring surrounding delimiters. Interpolated-string text parts use this
// after the parser has separated expressions and collapsed doubled braces.
//
// Rules:
//   - rules/foundations/lexical_structure.md — §14.3 "Interpolated strings"
//   - rules/foundations/lexical_structure.md — §15 "Escapes"
func DecodeStringText(text string) (string, bool) {
	return decodeEscapedText([]rune(text))
}

// decodeQuotedLiteral materializes Sec's escape inventory independently of
// Go's quoted-literal syntax, notably supporting Sec's \u{H...} spelling.
//
// Rules:
//   - rules/foundations/lexical_structure.md — §15 "Escapes"
func decodeQuotedLiteral(lexeme string, quote rune, character bool) (string, bool) {
	source := []rune(lexeme)
	if len(source) < 2 || source[0] != quote || source[len(source)-1] != quote {
		return "", false
	}
	result, ok := decodeEscapedText(source[1 : len(source)-1])
	if !ok || character && utf8.RuneCountInString(result) != 1 {
		return "", false
	}
	return result, true
}

// decodeEscapedText is the single materialization path for validated ordinary
// string, character, and interpolated-text contents.
//
// Rules: rules/foundations/lexical_structure.md — §15 "Escapes".
func decodeEscapedText(source []rune) (string, bool) {
	var decoded strings.Builder
	for index := 0; index < len(source); index++ {
		current := source[index]
		if current == '\n' || current == '\r' {
			return "", false
		}
		if current != '\\' {
			decoded.WriteRune(current)
			continue
		}
		index++
		if index >= len(source) {
			return "", false
		}
		switch source[index] {
		case '\\':
			decoded.WriteRune('\\')
		case '"':
			decoded.WriteRune('"')
		case '\'':
			decoded.WriteRune('\'')
		case 'n':
			decoded.WriteRune('\n')
		case 'r':
			decoded.WriteRune('\r')
		case 't':
			decoded.WriteRune('\t')
		case '0':
			decoded.WriteRune(0)
		case 'x':
			if index+2 >= len(source) {
				return "", false
			}
			value, err := strconv.ParseUint(string(source[index+1:index+3]), 16, 8)
			if err != nil {
				return "", false
			}
			decoded.WriteRune(rune(value))
			index += 2
		case 'u':
			if index+1 >= len(source) || source[index+1] != '{' {
				return "", false
			}
			digitStart := index + 2
			end := digitStart
			for end < len(source) && source[end] != '}' {
				end++
			}
			if end >= len(source) || end == digitStart || end-digitStart > 6 {
				return "", false
			}
			value, err := strconv.ParseUint(string(source[digitStart:end]), 16, 32)
			if err != nil || value > utf8.MaxRune || value >= 0xD800 && value <= 0xDFFF {
				return "", false
			}
			decoded.WriteRune(rune(value))
			index = end
		default:
			return "", false
		}
	}
	return decoded.String(), true
}

// readEscape validates one Sec escape without decoding or rewriting its source.
// It preserves quote/newline boundaries when recovering malformed escapes.
// Rules: rules/foundations/lexical_structure.md — "15. Escapes".
func (l *Lexer) readEscape(quote rune) {
	start, line, column := l.pos, l.line, l.column
	id, message := "", ""
	l.advance()
	kind := l.peek()
	if l.atEnd() || isPhysicalLineEnding(kind) {
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
			for l.peek() != '}' && l.peek() != quote && l.peek() != '\\' && !l.atEnd() && !isPhysicalLineEnding(l.peek()) {
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
