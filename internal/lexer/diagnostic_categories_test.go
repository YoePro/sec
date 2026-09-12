package lexer

import (
	"testing"

	compilerdiagnostics "sec/internal/diagnostics"
)

// TestRequiredLexicalErrorCategoriesHaveStableDiagnostics is the exhaustive
// executable inventory for every lexical error category required by the
// language rulebook. Each representative must reach one registered, mandatory
// lexer diagnostic without a generic or overlapping fallback.
//
// Rules:
//   - rules/foundations/lexical_structure.md — §20 "Lexical errors"
//   - rules/foundations/lexical_structure.md — §25 "Required tests"
func TestRequiredLexicalErrorCategoriesHaveStableDiagnostics(t *testing.T) {
	tests := []struct {
		category string
		input    string
		id       string
	}{
		{category: "invalid UTF-8", input: string([]byte{0xff}), id: compilerdiagnostics.LexerInvalidUTF8},
		{category: "unexpected byte-order mark", input: "module\uFEFFmain", id: compilerdiagnostics.LexerUnexpectedByteOrderMark},
		{category: "unsupported Unicode whitespace", input: "\u00A0", id: compilerdiagnostics.LexerUnsupportedWhitespace},
		{category: "non-NFC identifier", input: "cafe\u0301", id: compilerdiagnostics.LexerNonNFCIdentifier},
		{category: "invalid identifier character", input: "q\u0301", id: compilerdiagnostics.LexerIdentifierCharacter},
		{category: "unknown escape sequence", input: `"\q"`, id: compilerdiagnostics.LexerUnknownEscape},
		{category: "malformed escape sequence", input: `"\x1"`, id: compilerdiagnostics.LexerMalformedEscape},
		{category: "invalid Unicode escape", input: `"\u{D800}"`, id: compilerdiagnostics.LexerInvalidUnicodeEscape},
		{category: "invalid character literal length", input: `''`, id: compilerdiagnostics.LexerCharacterLiteralLength},
		{category: "malformed numeric literal", input: "0xGG", id: compilerdiagnostics.LexerMalformedBaseLiteral},
		{category: "invalid base digit", input: "0b2", id: compilerdiagnostics.LexerInvalidBaseDigit},
		{category: "invalid digit separator", input: "1__0", id: compilerdiagnostics.LexerInvalidDigitSeparator},
		{category: "invalid numeric suffix", input: "10z", id: compilerdiagnostics.LexerInvalidNumericSuffix},
		{category: "missing exponent digits", input: "1e+", id: compilerdiagnostics.LexerMissingExponentDigits},
		{category: "unterminated block comment", input: "/*", id: compilerdiagnostics.LexerUnterminatedBlockComment},
		{category: "unterminated ordinary string", input: `"open`, id: compilerdiagnostics.LexerUnterminatedOrdinaryString},
		{category: "unterminated raw string", input: "`open", id: compilerdiagnostics.LexerUnterminatedRawString},
		{category: "unterminated character literal", input: `'x`, id: compilerdiagnostics.LexerUnterminatedCharacterLiteral},
		{category: "unterminated interpolated string", input: `$"open`, id: compilerdiagnostics.LexerUnterminatedInterpolatedString},
		{category: "invalid source character", input: "$", id: compilerdiagnostics.LexerInvalidSourceCharacter},
	}

	seen := make(map[string]string, len(tests))
	for _, test := range tests {
		t.Run(test.category, func(t *testing.T) {
			if previous, duplicate := seen[test.id]; duplicate {
				t.Fatalf("diagnostic %s represents both %q and %q", test.id, previous, test.category)
			}
			seen[test.id] = test.category

			definition, ok := compilerdiagnostics.Lookup(test.id)
			if !ok || definition.Family != "lexer" || definition.DefaultSeverity != compilerdiagnostics.SeverityError || !definition.Mandatory || definition.Retired {
				t.Fatalf("diagnostic %s is not an active mandatory lexer error: %+v", test.id, definition)
			}

			lexer := NewWithFile(test.input, "category.sec")
			for token := lexer.NextToken(); token.Type != EOF; token = lexer.NextToken() {
			}
			diagnostics := lexer.Diagnostics()
			if len(diagnostics) != 1 || diagnostics[0].ID != test.id {
				t.Fatalf("%s diagnostics = %+v, want only %s", test.category, diagnostics, test.id)
			}
		})
	}
}
