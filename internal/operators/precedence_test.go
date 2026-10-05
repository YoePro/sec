package operators

import (
	"reflect"
	"testing"

	"sec/internal/lexer"
)

// Rules:
//   - rules/foundations/operators.md — "Canonical precedence"
//   - rules/foundations/operators.md — Appendix A.2 "Create one precedence definition"
func TestCanonicalPrecedenceDefinitions(t *testing.T) {
	want := []struct {
		category      string
		precedence    Precedence
		associativity Associativity
		spellings     []string
	}{
		{"logical-or", LogicalOr, LeftShortCircuit, []string{"||"}},
		{"logical-and", LogicalAnd, LeftShortCircuit, []string{"&&"}},
		{"bitwise-or", BitwiseOr, Left, []string{"|"}},
		{"bitwise-xor", BitwiseXor, Left, []string{"^"}},
		{"bitwise-and", BitwiseAnd, Left, []string{"&"}},
		{"equality-and-state-test", Equality, NonChainable, []string{"==", "!=", "is", "is not"}},
		{"ordered-comparison-and-membership", Comparison, NonChainable, []string{"<", "<=", ">", ">=", "in", "not in"}},
		{"shift", Shift, Left, []string{"<<", ">>"}},
		{"additive", Additive, Left, []string{"+", "-"}},
		{"multiplicative", Multiplicative, Left, []string{"*", "/", "%", "x"}},
		{"prefix", Prefix, Right, []string{"unary +", "unary -", "!", "~"}},
		{"postfix", Postfix, LeftChaining, []string{"call ()", "index or slice []", "struct literal {}", "spread ..."}},
		{"member", Member, LeftChaining, []string{"."}},
	}
	got := Definitions()
	if len(got) != len(want) {
		t.Fatalf("definition count = %d, want %d", len(got), len(want))
	}
	for index, expected := range want {
		actual := got[index]
		if actual.Category != expected.category || actual.Precedence != expected.precedence ||
			actual.Associativity != expected.associativity || !reflect.DeepEqual(actual.Spellings, expected.spellings) {
			t.Fatalf("definition %d = %+v, want %+v", index, actual, expected)
		}
		if index > 0 && got[index-1].Precedence >= actual.Precedence {
			t.Fatalf("definitions are not weak-to-strong at %q", actual.Category)
		}
	}
}

func TestTokenAndContextualSpellingsShareCanonicalPrecedence(t *testing.T) {
	checks := []struct {
		token lexer.TokenType
		text  string
		want  Precedence
	}{
		{lexer.OR, "||", LogicalOr}, {lexer.AND, "&&", LogicalAnd},
		{lexer.BIT_OR, "|", BitwiseOr}, {lexer.BIT_XOR, "^", BitwiseXor}, {lexer.BIT_AND, "&", BitwiseAnd},
		{lexer.EQ, "==", Equality}, {lexer.NEQ, "!=", Equality},
		{lexer.LT, "<", Comparison}, {lexer.IN, "in", Comparison},
		{lexer.SHIFT_LEFT, "<<", Shift}, {lexer.PLUS, "+", Additive}, {lexer.ASTERISK, "*", Multiplicative},
	}
	for _, check := range checks {
		fromToken, tokenOK := TokenPrecedence(check.token)
		fromText, textOK := BinaryPrecedence(check.text)
		if !tokenOK || !textOK || fromToken != check.want || fromText != check.want {
			t.Fatalf("%q precedence: token=%v/%v text=%v/%v want=%v", check.text, fromToken, tokenOK, fromText, textOK, check.want)
		}
	}
	for _, check := range []struct {
		text string
		want Precedence
	}{{"x", Multiplicative}, {"not in", Comparison}, {"is", Equality}, {"is not", Equality}} {
		if got, ok := BinaryPrecedence(check.text); !ok || got != check.want {
			t.Fatalf("contextual %q precedence = %v, %v; want %v", check.text, got, ok, check.want)
		}
	}
}
