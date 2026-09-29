package lexer

import (
	"testing"

	compilerdiagnostics "sec/internal/diagnostics"
)

// Rules:
//   - rules/foundations/lexical_structure.md — §6.1 "Identifier form"
//   - rules/foundations/lexical_structure.md — §20 "Lexical errors"
func TestNonASCIIDigitsRemainOneInvalidIdentifierCandidate(t *testing.T) {
	for _, input := range []string{"value١", "_٢name", "٣start"} {
		t.Run(input, func(t *testing.T) {
			lexer := NewWithFile(input+" valid", "identifier-digit.sec")
			token := lexer.NextToken()
			if token.Type != ILLEGAL || token.Lexeme != input || token.Line != 1 || token.Column != 1 {
				t.Fatalf("invalid identifier candidate was split: %+v", token)
			}
			diagnostics := lexer.Diagnostics()
			if len(diagnostics) != 1 || diagnostics[0].ID != compilerdiagnostics.LexerIdentifierCharacter || diagnostics[0].Primary != token {
				t.Fatalf("wrong identifier diagnostic: %+v", diagnostics)
			}
			if next := lexer.NextToken(); next.Type != IDENT || next.Lexeme != "valid" || next.Column != len([]rune(input))+2 {
				t.Fatalf("lexer did not recover at the following identifier: %+v", next)
			}
		})
	}
}
