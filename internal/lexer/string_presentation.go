package lexer

import (
	"fmt"
	"strings"
	"unicode"
)

// QuoteDecodedStringLiteral renders an already decoded source string using
// Sec's escape grammar. Control scalars use \u{...}, never Go-only escapes.
// Rules: rules/foundations/lexical_structure.md — §§14.1,15.
func QuoteDecodedStringLiteral(value string) string {
	var out strings.Builder
	out.WriteByte('"')
	for _, scalar := range value {
		switch scalar {
		case '\\':
			out.WriteString(`\\`)
		case '"':
			out.WriteString(`\"`)
		case '\n':
			out.WriteString(`\n`)
		case '\r':
			out.WriteString(`\r`)
		case '\t':
			out.WriteString(`\t`)
		case 0:
			out.WriteString(`\0`)
		default:
			if unicode.IsPrint(scalar) {
				out.WriteRune(scalar)
			} else {
				fmt.Fprintf(&out, `\u{%x}`, scalar)
			}
		}
	}
	out.WriteByte('"')
	return out.String()
}
