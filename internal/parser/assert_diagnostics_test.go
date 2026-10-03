package parser

import (
	"strings"
	"testing"

	compilerdiagnostics "sec/internal/diagnostics"
	"sec/internal/lexer"
)

// Assertion syntax errors carry stable identities and a mentor help naming
// the canonical `assert condition[, "message"]` form, while the established
// messages stay unchanged.
//
// Rules:
//   - rules/errors/panic.md — § 15.1 "Canonical syntax", § 28(1)–(4)
func TestAssertSyntaxDiagnosticsAreStableMentorDiagnostics(t *testing.T) {
	tests := []struct {
		name string
		body string
		id   string
		help string
	}{
		{name: "function-like", body: "assert(count > 0)", id: compilerdiagnostics.ParserFunctionLikeAssert, help: "not a function"},
		{name: "missing separator", body: `assert count > 0 "message"`, id: compilerdiagnostics.ParserAssertMessageSeparator, help: "with a comma"},
		{name: "dynamic message", body: "assert count > 0, name", id: compilerdiagnostics.ParserAssertMessageNotLiteral, help: "string literal"},
		{name: "missing condition", body: "assert", id: compilerdiagnostics.ParserAssertMissingCondition, help: "bool condition"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			source := "fn Check(count: int, name: string) void {\n    " + test.body + "\n}\n"
			result := New(lexer.New(source)).Parse()
			found := false
			for _, diagnostic := range result.Diagnostics {
				if diagnostic.ID == test.id {
					found = true
					if !strings.Contains(diagnostic.Help, test.help) {
						t.Fatalf("help = %q, want mention of %q", diagnostic.Help, test.help)
					}
				}
			}
			if !found {
				t.Fatalf("diagnostics = %+v, want %s", result.Diagnostics, test.id)
			}
		})
	}
	if result := New(lexer.New("fn Check(count: int) void {\n    assert count > 0, \"positive\"\n}\n")).Parse(); len(result.Diagnostics) != 0 {
		t.Fatalf("canonical assertion produced diagnostics: %+v", result.Diagnostics)
	}
}
