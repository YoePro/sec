package lexer

import "testing"

// TestQuotedDecodedStringsUseSecGrammar preserves decoded constant values,
// including controls for which Go's quoted spelling is not valid Sec syntax.
// Rules: rules/foundations/lexical_structure.md — §§14.1,15.
func TestQuotedDecodedStringsUseSecGrammar(t *testing.T) {
	for _, value := range []string{"", "Alpha Beta", "line\n\"quoted\"", "\a\b\f\v\x00", "å雪\u2028", `\u{41}`} {
		quoted := QuoteDecodedStringLiteral(value)
		l := New(quoted)
		token := l.NextToken()
		if token.Type != STRING || l.NextToken().Type != EOF {
			t.Fatalf("not one Sec string literal: %q", quoted)
		}
		decoded, ok := DecodeStringLiteral(quoted)
		if !ok || decoded != value {
			t.Fatalf("invalid/lossy Sec spelling %q: %q -> %q", quoted, value, decoded)
		}
	}
}
