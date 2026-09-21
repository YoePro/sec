package lexer

import "testing"

// TestAvailabilityWordsRemainContextual verifies that ownership-state syntax
// does not reserve ordinary identifier spellings in the lexer.
//
// Rules:
//   - rules/memory/ownership.md — §21 "is available and is not available"
//   - rules/foundations/lexical_structure.md — contextual spellings
func TestAvailabilityWordsRemainContextual(t *testing.T) {
	l := New("value is available\nvalue is not available")
	want := []string{"value", "is", "available", "value", "is", "not", "available"}
	for index, lexeme := range want {
		token := l.NextToken()
		if token.Type != IDENT || token.Lexeme != lexeme {
			t.Fatalf("token %d = %#v, want contextual identifier %q", index, token, lexeme)
		}
	}
}
